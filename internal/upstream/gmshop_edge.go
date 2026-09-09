package upstream

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const gmshopResponseLimit = 10 << 20

const (
	GMShopEdgeHeaderAPIKey    = "GMShop-Edge-Api-Key"
	GMShopEdgeHeaderTimestamp = "GMShop-Edge-Timestamp"
	GMShopEdgeHeaderNonce     = "GMShop-Edge-Nonce"
	GMShopEdgeHeaderSignature = "GMShop-Edge-Signature"
)

type GMShopEdgeAdapter struct {
	connectionID uint
	baseURL      string
	apiKey       string
	apiSecret    string
	references   ExternalReferenceRegistry
	downloader   *DujiaoNextAdapter
	client       *http.Client
}

func NewGMShopEdgeAdapter(conn *siteconnectiondomain.Connection, uploadsDir string, references ExternalReferenceRegistry) *GMShopEdgeAdapter {
	return &GMShopEdgeAdapter{
		connectionID: conn.ID,
		baseURL:      strings.TrimRight(conn.BaseURL, "/"),
		apiKey:       conn.ApiKey,
		apiSecret:    conn.ApiSecret,
		references:   references,
		downloader:   NewDujiaoNextAdapter(conn, uploadsDir),
		client:       &http.Client{Timeout: 30 * time.Second},
	}
}

type gmshopProduct struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	ImageURLs     []string    `json:"image_urls"`
	CategoryNames []string    `json:"category_names"`
	Active        bool        `json:"active"`
	UpdatedAt     string      `json:"updated_at"`
	SKUs          []gmshopSKU `json:"skus"`
}

type gmshopSKU struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	CostMinor     string `json:"cost_minor"`
	StockQuantity int    `json:"stock_quantity"`
	Active        bool   `json:"active"`
}

func (a *GMShopEdgeAdapter) Ping(ctx context.Context) (*PingResult, error) {
	var result struct {
		SiteName     string `json:"site_name"`
		BalanceMinor string `json:"balance_minor"`
		Currency     string `json:"currency"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v1/supplier/ping", nil, &result); err != nil {
		return nil, err
	}
	return &PingResult{
		SiteName: result.SiteName, ProtocolVersion: "gmshop-edge-v1",
		UserID: a.connectionID, Balance: minorToMajor(result.BalanceMinor, 2), Currency: result.Currency,
	}, nil
}

func (a *GMShopEdgeAdapter) ListCategories(ctx context.Context) (*CategoryListResult, error) {
	var result struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v1/supplier/categories", nil, &result); err != nil {
		return nil, err
	}
	categories := make([]UpstreamCategory, 0, len(result.Items))
	for _, item := range result.Items {
		external := strings.TrimSpace(item.Name)
		if external == "" {
			external = item.ID
		}
		id, err := a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindCategory, external)
		if err != nil {
			return nil, err
		}
		categories = append(categories, UpstreamCategory{ID: id, Slug: fmt.Sprintf("gmshop-%d", id), Name: localized(item.Name)})
	}
	return &CategoryListResult{Supported: true, Categories: categories}, nil
}

func (a *GMShopEdgeAdapter) ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	query := url.Values{"page": {strconv.Itoa(opts.Page)}, "page_size": {strconv.Itoa(opts.PageSize)}}
	if opts.UpdatedAfter != nil {
		query.Set("updated_after", opts.UpdatedAfter.Format(time.RFC3339))
	}
	var result struct {
		Total int             `json:"total"`
		Items []gmshopProduct `json:"items"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v1/supplier/products?"+query.Encode(), nil, &result); err != nil {
		return nil, err
	}
	products := make([]UpstreamProduct, 0, len(result.Items))
	for _, item := range result.Items {
		product, err := a.product(item)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return &ProductListResult{Total: result.Total, Items: products, IncludesInactive: false}, nil
}

func (a *GMShopEdgeAdapter) GetProduct(ctx context.Context, productID uint) (*UpstreamProduct, error) {
	external, err := a.references.Lookup(a.connectionID, siteconnectiondomain.ExternalReferenceKindProduct, productID)
	if err != nil {
		return nil, err
	}
	if external == "" {
		return nil, ErrUpstreamProductDeleted
	}
	var result struct {
		Product gmshopProduct `json:"product"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v1/supplier/products/"+url.PathEscape(external), nil, &result); err != nil {
		if extractUpstreamErrorCode(err) == "supplier_product_not_found" {
			return nil, ErrUpstreamProductDeleted
		}
		return nil, err
	}
	product, err := a.product(result.Product)
	return &product, err
}

func (a *GMShopEdgeAdapter) CreateOrder(ctx context.Context, req CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error) {
	if strings.TrimSpace(req.DownstreamOrderNo) == "" {
		return nil, fmt.Errorf("gmshop-edge downstream order number is required")
	}
	externalSKU, err := a.references.Lookup(a.connectionID, siteconnectiondomain.ExternalReferenceKindSKU, req.SKUID)
	if err != nil {
		return nil, err
	}
	if externalSKU == "" {
		return nil, fmt.Errorf("gmshop-edge sku reference not found")
	}
	body := map[string]interface{}{
		"sku_id": externalSKU, "quantity": req.Quantity,
		"downstream_order_no": req.DownstreamOrderNo, "trace_id": req.TraceID,
	}
	var result struct {
		OK               bool   `json:"ok"`
		OrderID          string `json:"order_id"`
		Status           string `json:"status"`
		AmountMinor      string `json:"amount_minor"`
		Currency         string `json:"currency"`
		CurrencyDecimals int    `json:"currency_decimals"`
		ErrorCode        string `json:"error_code"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v1/supplier/orders", body, &result); err != nil {
		return nil, err
	}
	if !result.OK || result.OrderID == "" {
		return &CreateUpstreamOrderResp{OK: false, ErrorCode: result.ErrorCode}, nil
	}
	orderID, err := a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindOrder, result.OrderID)
	if err != nil {
		return nil, err
	}
	return &CreateUpstreamOrderResp{
		OK: true, OrderID: orderID, OrderNo: result.OrderID, Status: result.Status,
		Amount: minorToMajor(result.AmountMinor, result.CurrencyDecimals), Currency: result.Currency,
	}, nil
}

func (a *GMShopEdgeAdapter) GetOrder(ctx context.Context, orderID uint) (*UpstreamOrderDetail, error) {
	external, err := a.references.Lookup(a.connectionID, siteconnectiondomain.ExternalReferenceKindOrder, orderID)
	if err != nil {
		return nil, err
	}
	if external == "" {
		return nil, fmt.Errorf("gmshop-edge order reference not found")
	}
	var result struct {
		OrderID          string   `json:"order_id"`
		Status           string   `json:"status"`
		AmountMinor      string   `json:"amount_minor"`
		Currency         string   `json:"currency"`
		CurrencyDecimals int      `json:"currency_decimals"`
		Cards            []string `json:"cards"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v1/supplier/orders/"+url.PathEscape(external), nil, &result); err != nil {
		return nil, err
	}
	detail := &UpstreamOrderDetail{
		OrderID: orderID, OrderNo: result.OrderID, Status: result.Status,
		Amount: minorToMajor(result.AmountMinor, result.CurrencyDecimals), Currency: result.Currency,
	}
	if result.Status == "supplied" && len(result.Cards) > 0 {
		payload := strings.Join(result.Cards, "\n")
		now := time.Now()
		detail.Status = "delivered"
		detail.Fulfillment = &UpstreamFulfillment{
			Type: "auto", Status: "delivered", Payload: payload,
			DeliveryData: jsonmap.JSON{"cards": result.Cards}, DeliveredAt: &now,
		}
	}
	return detail, nil
}

func (a *GMShopEdgeAdapter) CancelOrder(ctx context.Context, orderID uint) error {
	external, err := a.references.Lookup(a.connectionID, siteconnectiondomain.ExternalReferenceKindOrder, orderID)
	if err != nil {
		return err
	}
	if external == "" {
		return fmt.Errorf("gmshop-edge order reference not found")
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v1/supplier/orders/"+url.PathEscape(external)+"/cancel", map[string]interface{}{}, &result); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("gmshop-edge cancel failed")
	}
	return nil
}

func (a *GMShopEdgeAdapter) DownloadImage(ctx context.Context, imageURL string) (string, error) {
	return a.downloader.DownloadImage(ctx, imageURL)
}

func (a *GMShopEdgeAdapter) product(value gmshopProduct) (UpstreamProduct, error) {
	productID, err := a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindProduct, value.ID)
	if err != nil {
		return UpstreamProduct{}, err
	}
	categoryName := "Central Catalog"
	if len(value.CategoryNames) > 0 && strings.TrimSpace(value.CategoryNames[0]) != "" {
		categoryName = strings.TrimSpace(value.CategoryNames[0])
	}
	categoryID, err := a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindCategory, categoryName)
	if err != nil {
		return UpstreamProduct{}, err
	}
	skus := make([]UpstreamSKU, 0, len(value.SKUs))
	for _, item := range value.SKUs {
		id, err := a.references.Resolve(a.connectionID, siteconnectiondomain.ExternalReferenceKindSKU, item.ID)
		if err != nil {
			return UpstreamProduct{}, err
		}
		status := "out_of_stock"
		if item.StockQuantity > 0 {
			status = "in_stock"
		}
		skus = append(skus, UpstreamSKU{
			ID: id, SKUCode: item.ID, SpecValues: jsonmap.JSON{"name": item.Name},
			PriceAmount: minorToMajor(item.CostMinor, 2), StockStatus: status,
			StockQuantity: item.StockQuantity, IsActive: item.Active,
		})
	}
	updatedAt, _ := time.Parse(time.RFC3339, value.UpdatedAt)
	return UpstreamProduct{
		ID: productID, Title: localized(value.Name), Description: localized(value.Description), Content: localized(value.Description),
		Images: value.ImageURLs, Tags: value.CategoryNames, PriceAmount: minimumPrice(skus), Currency: "CNY",
		FulfillmentType: "auto", ManualFormSchema: jsonmap.JSON{}, IsActive: value.Active,
		CategoryID: categoryID, SKUs: skus, UpdatedAt: updatedAt,
	}, nil
}

func (a *GMShopEdgeAdapter) request(ctx context.Context, method, pathWithQuery string, body interface{}, result interface{}) error {
	var rawBody []byte
	if body != nil {
		var err error
		rawBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal gmshop-edge request: %w", err)
		}
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := uuid.NewString()
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+pathWithQuery, bytes.NewReader(rawBody))
	if err != nil {
		return fmt.Errorf("create gmshop-edge request: %w", err)
	}
	req.Header.Set(GMShopEdgeHeaderAPIKey, a.apiKey)
	req.Header.Set(GMShopEdgeHeaderTimestamp, timestamp)
	req.Header.Set(GMShopEdgeHeaderNonce, nonce)
	req.Header.Set(GMShopEdgeHeaderSignature, signGMShopEdge(a.apiSecret, method, pathWithQuery, timestamp, nonce, rawBody))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("send gmshop-edge request: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, gmshopResponseLimit+1))
	if err != nil {
		return fmt.Errorf("read gmshop-edge response: %w", err)
	}
	if len(responseBody) > gmshopResponseLimit {
		return fmt.Errorf("gmshop-edge response too large")
	}
	if resp.StatusCode != http.StatusOK {
		var failure struct {
			ErrorCode string `json:"error_code"`
			Error     struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(responseBody, &failure)
		code := failure.ErrorCode
		if code == "" {
			code = failure.Error.Code
		}
		return &upstreamHTTPError{Status: resp.StatusCode, Code: code, Body: string(responseBody)}
	}
	if err := json.Unmarshal(responseBody, result); err != nil {
		return fmt.Errorf("decode gmshop-edge response: %w", err)
	}
	return nil
}

func signGMShopEdge(secret, method, pathWithQuery, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	payload := strings.Join([]string{strings.ToUpper(method), pathWithQuery, timestamp, nonce, hex.EncodeToString(digest[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyGMShopEdgeSignature(secret, method, pathWithQuery, timestamp, nonce, signature string, body []byte) bool {
	expected := signGMShopEdge(secret, method, pathWithQuery, timestamp, nonce, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

func minorToMajor(value string, decimals int) string {
	n, err := decimal.NewFromString(strings.TrimSpace(value))
	if err != nil || n.IsNegative() {
		return "0"
	}
	if decimals < 0 {
		return "0"
	}
	return n.Shift(-int32(decimals)).String()
}

func minimumPrice(skus []UpstreamSKU) string {
	if len(skus) == 0 {
		return "0"
	}
	minimum, err := decimal.NewFromString(skus[0].PriceAmount)
	if err != nil {
		return "0"
	}
	for _, sku := range skus[1:] {
		price, parseErr := decimal.NewFromString(sku.PriceAmount)
		if parseErr == nil && price.LessThan(minimum) {
			minimum = price
		}
	}
	return minimum.String()
}
