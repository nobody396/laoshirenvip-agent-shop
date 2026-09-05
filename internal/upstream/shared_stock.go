package upstream

import (
	"context"
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
		result = append(result, UpstreamCategory{ID: id, Name: localized(category.Name)})
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
	result, err := a.product(*commodity, 0)
	return &result, err
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
	trade, err := a.client.Trade(ctx, sharedstock.TradeRequest{SharedCode: code, Race: race, Quantity: req.Quantity, RequestNo: req.DownstreamOrderNo})
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
	return &CreateUpstreamOrderResp{OK: true, OrderID: orderID, OrderNo: rawOrderID, Status: "accepted", Amount: string(trade.Amount), Currency: "CNY"}, nil
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
		detail.Fulfillment = &UpstreamFulfillment{Type: "auto", Status: "delivered", Payload: result.Secret, DeliveryData: jsonmap.JSON{"cards": splitCards(result.Secret)}, DeliveredAt: &now}
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
