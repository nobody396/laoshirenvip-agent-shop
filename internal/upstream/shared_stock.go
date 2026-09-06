package upstream

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/upstream/sharedstock"
)

type SharedStockAdapter struct {
	connectionID uint
	client       *sharedstock.Client
	references   ExternalReferenceRegistry
	downloader   *DujiaoNextAdapter
}

func NewSharedStockAdapter(conn *siteconnectiondomain.Connection, uploadsDir string, references ExternalReferenceRegistry) *SharedStockAdapter {
	return &SharedStockAdapter{
		connectionID: conn.ID,
		client:       sharedstock.NewClient(conn.BaseURL, conn.ApiKey, conn.ApiSecret),
		references:   references,
		downloader:   NewDujiaoNextAdapter(conn, uploadsDir),
	}
}

func (a *SharedStockAdapter) Ping(ctx context.Context) (*PingResult, error) {
	result, err := a.client.Connect(ctx)
	if err != nil {
		return nil, err
	}
	userID, _ := strconv.ParseUint(strings.TrimSpace(fmt.Sprint(a.connectionID)), 10, 64)
	return &PingResult{SiteName: result.ShopName, ProtocolVersion: "acg-sharedstock-v1", UserID: uint(userID), Balance: string(result.Balance), Currency: "CNY"}, nil
}

func (a *SharedStockAdapter) ListCategories(ctx context.Context) (*CategoryListResult, error) {
	categories, err := a.client.Items(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]UpstreamCategory, 0, len(categories))
	for _, category := range categories {
		key := string(category.ID)
		if key == "" {
			key = category.Name
		}
		id, err := a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindCategory, key)
		if err != nil {
			return nil, err
		}
		result = append(result, UpstreamCategory{
			ID: id, Slug: fmt.Sprintf("shared-%d", id), Name: localized(category.Name),
		})
	}
	return &CategoryListResult{Supported: true, Categories: result}, nil
}

func (a *SharedStockAdapter) ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	categories, err := a.client.Items(ctx)
	if err != nil {
		return nil, err
	}
	all := make([]UpstreamProduct, 0)
	for _, category := range categories {
		categoryKey := string(category.ID)
		if categoryKey == "" {
			categoryKey = category.Name
		}
		categoryID, err := a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindCategory, categoryKey)
		if err != nil {
			return nil, err
		}
		for _, commodity := range category.Children {
			product, err := a.product(commodity, categoryID)
			if err != nil {
				return nil, err
			}
			all = append(all, product)
		}
	}
	start := (opts.Page - 1) * opts.PageSize
	if start < 0 {
		start = 0
	}
	end := start + opts.PageSize
	if start > len(all) {
		start = len(all)
	}
	if end > len(all) {
		end = len(all)
	}
	return &ProductListResult{Total: len(all), Items: all[start:end], IncludesInactive: false}, nil
}

func (a *SharedStockAdapter) GetProduct(ctx context.Context, productID uint) (*UpstreamProduct, error) {
	code, err := a.references.Lookup(a.connectionID, siteconnectiondomain.ExternalReferenceKindProduct, productID)
	if err != nil {
		return nil, err
	}
	if code == "" {
		return nil, ErrUpstreamProductDeleted
	}
	commodity, err := a.client.Item(ctx, code)
	if err != nil {
		return nil, err
	}
	categoryID, err := a.categoryIDForProduct(ctx, code)
	if err != nil {
		return nil, err
	}
	result, err := a.product(*commodity, categoryID)
	return &result, err
}

// SharedStock's item endpoint does not carry the parent category. Importing a
// product with category 0 breaks PostgreSQL's products -> categories foreign
// key, so resolve the category from the authoritative items tree before
// returning product details. The persistent reference registry keeps the
// resulting numeric ID stable across syncs and restarts.
func (a *SharedStockAdapter) categoryIDForProduct(ctx context.Context, code string) (uint, error) {
	categories, err := a.client.Items(ctx)
	if err != nil {
		return 0, err
	}
	for _, category := range categories {
		for _, commodity := range category.Children {
			if commodity.Code != code {
				continue
			}
			key := string(category.ID)
			if key == "" {
				key = category.Name
			}
			return a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindCategory, key)
		}
	}
	return 0, ErrUpstreamProductDeleted
}

func (a *SharedStockAdapter) CreateOrder(ctx context.Context, req CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error) {
	key, err := a.references.Lookup(a.connectionID, siteconnectiondomain.ExternalReferenceKindSKU, req.SKUID)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, fmt.Errorf("shared-stock sku reference not found")
	}
	code, race := decodeSharedKey(key)
	contact, password := sharedStockOrderLookup(req.DownstreamOrderNo)
	requestNo := sharedStockRequestNo(req.DownstreamOrderNo)
	trade, err := a.client.Trade(ctx, sharedstock.TradeRequest{
		SharedCode: code, Race: race, Quantity: req.Quantity, RequestNo: requestNo,
		Contact: contact, Password: password,
	})
	if err != nil {
		return nil, err
	}
	rawOrderID := string(trade.TradeNo)
	if rawOrderID == "" {
		return nil, sharedstock.ErrRequestUncertain
	}
	orderID, err := a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindOrder, rawOrderID)
	if err != nil {
		return nil, err
	}
	result := &CreateUpstreamOrderResp{OK: true, OrderID: orderID, OrderNo: rawOrderID, Status: "accepted", Amount: string(trade.Amount), Currency: "CNY"}
	if strings.TrimSpace(trade.Secret) != "" {
		now := time.Now()
		payload := sharedStockDeliveryPayload(trade.Secret, trade.URL, sharedStockRedeemURL(code))
		result.Status = "delivered"
		result.Fulfillment = &UpstreamFulfillment{
			Type: "auto", Status: "delivered", Payload: payload,
			DeliveryData: sharedStockDeliveryData(payload, req.Quantity), DeliveredAt: &now,
		}
	}
	return result, nil
}

// sharedStockRequestNo derives the stable upstream idempotency key from the
// complete local order number. AISOU accepts at most 19 characters, so hashing
// avoids the collisions that would be introduced by simply truncating the
// shared prefix of child order numbers. Eleven digest bytes encode to 18
// unpadded base32 characters; the leading L identifies our request namespace.
func sharedStockRequestNo(orderNo string) string {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(orderNo))
	token := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:11])
	return "L" + token
}

// sharedStockOrderLookup supplies the email contact and >=6-character query
// password required by every currently enabled AISOU product. Values are
// deterministic per downstream order, so an idempotent retry uses the exact
// same lookup credentials without exposing a customer address upstream.
func sharedStockOrderLookup(orderNo string) (string, string) {
	digest := sha256.Sum256([]byte(strings.TrimSpace(orderNo)))
	token := fmt.Sprintf("%x", digest[:6])
	return "order-" + token + "@lsrai.shop", "Ls" + token
}

func (a *SharedStockAdapter) GetOrder(ctx context.Context, orderID uint) (*UpstreamOrderDetail, error) {
	raw, err := a.references.Lookup(a.connectionID, siteconnectiondomain.ExternalReferenceKindOrder, orderID)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, fmt.Errorf("shared-stock order reference not found")
	}
	result, err := a.client.Query(ctx, raw)
	if err != nil {
		return nil, err
	}
	detail := &UpstreamOrderDetail{OrderID: orderID, OrderNo: raw, Status: "accepted", Currency: "CNY"}
	if strings.TrimSpace(result.Secret) != "" {
		now := time.Now()
		detail.Status = "delivered"
		detail.Fulfillment = &UpstreamFulfillment{Type: "auto", Status: "delivered", Payload: result.Secret, DeliveryData: sharedStockDeliveryData(result.Secret, 1), DeliveredAt: &now}
	}
	return detail, nil
}

func (a *SharedStockAdapter) CancelOrder(context.Context, uint) error {
	return sharedstock.ErrUnsupported
}
func (a *SharedStockAdapter) DownloadImage(ctx context.Context, imageURL string) (string, error) {
	return a.downloader.DownloadImage(ctx, imageURL)
}

func (a *SharedStockAdapter) product(value sharedstock.Commodity, categoryID uint) (UpstreamProduct, error) {
	productID, err := a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindProduct, value.Code)
	if err != nil {
		return UpstreamProduct{}, err
	}
	variants := sharedVariants(value.Config)
	if len(variants) == 0 {
		variants = map[string]string{"": string(value.Price)}
	}
	keys := make([]string, 0, len(variants))
	for key := range variants {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	skus := make([]UpstreamSKU, 0, len(keys))
	for _, race := range keys {
		external := encodeSharedKey(value.Code, race)
		id, err := a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindSKU, external)
		if err != nil {
			return UpstreamProduct{}, err
		}
		skus = append(skus, UpstreamSKU{ID: id, SKUCode: external, SpecValues: jsonmap.JSON{"race": race}, PriceAmount: variants[race], StockStatus: "in_stock", StockQuantity: normalizeSharedStock(value.Stock), IsActive: true})
	}
	cover := value.Cover
	if cover != "" {
		if parsed, err := url.Parse(cover); err == nil && !parsed.IsAbs() {
			cover = strings.TrimRight(a.downloader.baseURL, "/") + "/" + strings.TrimLeft(cover, "/")
		}
	}
	images := []string{}
	if cover != "" {
		images = []string{cover}
	}
	return UpstreamProduct{ID: productID, Title: localized(value.Name), Description: localized(value.Description), Content: localized(value.Description), Images: images, PriceAmount: string(value.Price), Currency: "CNY", FulfillmentType: "auto", ManualFormSchema: jsonmap.JSON{}, IsActive: true, CategoryID: categoryID, SKUs: skus}, nil
}

func localized(value string) jsonmap.JSON { return jsonmap.JSON{"zh-CN": value, "en": value} }
func encodeSharedKey(code, race string) string {
	if race == "" {
		return code
	}
	return code + "::" + url.QueryEscape(race)
}
func decodeSharedKey(value string) (string, string) {
	parts := strings.SplitN(value, "::", 2)
	if len(parts) == 1 {
		return value, ""
	}
	race, _ := url.QueryUnescape(parts[1])
	return parts[0], race
}
func normalizeSharedStock(value sharedstock.ScalarString) int {
	if value == "" || value == "-1" {
		return 2147483647
	}
	n, err := strconv.Atoi(string(value))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
func splitCards(value string) []string {
	lines := strings.FieldsFunc(value, func(r rune) bool { return r == '\n' || r == '\r' })
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if v := strings.TrimSpace(line); v != "" {
			result = append(result, v)
		}
	}
	return result
}

func sharedStockDeliveryData(value string, quantity int) jsonmap.JSON {
	payload := strings.TrimSpace(value)
	cards := splitCards(payload)
	// SharedStock returns one product's complete delivery in secret. For a
	// single-item order, a CDK and its redemption URL may occupy separate lines
	// but must remain one delivery card, matching GMShop's proven behaviour.
	if quantity <= 1 && payload != "" {
		cards = []string{payload}
	}

	data := jsonmap.JSON{"cards": cards}
	entries, hasCredential, hasURL := sharedStockDeliveryEntries(payload)
	if len(entries) > 0 {
		data["entries"] = entries
	}
	switch {
	case hasCredential && hasURL:
		data["note"] = "使用方法：复制 CDK 卡密，打开充值网址，按页面提示提交即可。"
	case hasCredential:
		data["note"] = "使用方法：复制 CDK 卡密；如订单未显示充值网址，请联系客服处理。"
	case hasURL:
		data["note"] = "使用方法：打开充值网址，按页面提示提交即可。"
	}
	return data
}

func sharedStockDeliveryPayload(secret string, returnedURLs ...string) string {
	parts := splitCards(secret)
	seen := make(map[string]struct{}, len(parts)+len(returnedURLs))
	for _, part := range parts {
		seen[part] = struct{}{}
	}
	for _, rawURL := range returnedURLs {
		link := standaloneHTTPURL(rawURL)
		if link == "" {
			continue
		}
		if _, exists := seen[link]; exists {
			continue
		}
		seen[link] = struct{}{}
		parts = append(parts, link)
	}
	return strings.Join(parts, "\n")
}

// sharedStockRedeemURL fills the redemption address only when AISOU's own
// product description publishes a stable URL. The trade endpoint may return
// url:null even though the CDK requires that page; never guess URLs for SKUs
// whose upstream documentation does not identify one.
func sharedStockRedeemURL(code string) string {
	return map[string]string{
		"D37C0FB7EC7A21F6": "https://aiee.fun/",                  // ChatGPT Plus 菲律宾
		"72CA8BC21CF70BBD": "https://aiee.fun/",                  // ChatGPT Pro 20X 菲律宾
		"2DF0B5724CBFF6BF": "https://aiee.fun/",                  // Codex 点数
		"D024411F1D93A771": "https://vip.sxzfd.com/",             // ChatGPT Plus iOS
		"15D9367883563482": "https://vip.sxzfd.com/claude",       // Claude Pro
		"D17577D14C0B63F9": "https://vip.sxzfd.com/claude",       // Claude Max 5X
		"3818E383895D6D12": "https://quickplus.vip/public/grok/", // SuperGrok
		"E68B2D302E19E8D4": "https://quickplus.vip/public/x_plus/",
		"024E86D71C074B4E": "https://quickplus.vip/public/x/",
	}[strings.TrimSpace(code)]
}

func sharedStockDeliveryEntries(value string) ([]map[string]string, bool, bool) {
	lines := splitCards(value)
	credentials := make([]string, 0, len(lines))
	urls := make([]string, 0, len(lines))
	for _, line := range lines {
		if link := standaloneHTTPURL(line); link != "" {
			urls = append(urls, link)
			continue
		}
		credentials = append(credentials, line)
	}

	entries := make([]map[string]string, 0, len(credentials)+len(urls))
	for index, credential := range credentials {
		key := "CDK 卡密"
		if len(credentials) > 1 {
			key = fmt.Sprintf("CDK 卡密 %d", index+1)
		}
		entries = append(entries, map[string]string{"key": key, "value": credential})
	}
	for index, link := range urls {
		key := "充值网址"
		if len(urls) > 1 {
			key = fmt.Sprintf("充值网址 %d", index+1)
		}
		entries = append(entries, map[string]string{"key": key, "value": link})
	}
	return entries, len(credentials) > 0, len(urls) > 0
}

func standaloneHTTPURL(value string) string {
	candidate := strings.TrimSpace(value)
	parsed, err := url.ParseRequestURI(candidate)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return candidate
}
func sharedVariants(raw json.RawMessage) map[string]string {
	result := map[string]string{}
	if len(raw) == 0 {
		return result
	}
	var encoded string
	if json.Unmarshal(raw, &encoded) == nil {
		return parseSharedINI(encoded)
	}
	var object map[string]any
	if json.Unmarshal(raw, &object) == nil {
		source, _ := object["category_factory"].(map[string]any)
		if len(source) == 0 {
			source, _ = object["category"].(map[string]any)
		}
		for key, value := range source {
			result[key] = fmt.Sprint(value)
		}
	}
	return result
}

func parseSharedINI(value string) map[string]string {
	categories := map[string]string{}
	factoryPrices := map[string]string{}
	section := ""
	for _, rawLine := range strings.Split(value, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, price, found := strings.Cut(line, "=")
		key, price = strings.TrimSpace(key), strings.TrimSpace(price)
		if !found || key == "" || price == "" {
			continue
		}
		switch section {
		case "category_factory":
			factoryPrices[key] = price
		case "category":
			categories[key] = price
		}
	}
	if len(factoryPrices) > 0 {
		return factoryPrices
	}
	return categories
}
