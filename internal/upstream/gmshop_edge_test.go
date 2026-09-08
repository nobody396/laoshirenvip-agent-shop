package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

func TestGMShopEdgeAdapterMapsCatalogAndOrderDelivery(t *testing.T) {
	const secret = "integration-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		want := signGMShopEdge(secret, req.Method, req.URL.RequestURI(), req.Header.Get("GMShop-Edge-Timestamp"), req.Header.Get("GMShop-Edge-Nonce"), raw)
		if got := req.Header.Get("GMShop-Edge-Signature"); got != want {
			t.Fatalf("signature = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/supplier/ping":
			_, _ = fmt.Fprint(w, `{"site_name":"Central","balance_minor":"12345","currency":"CNY"}`)
		case "/api/v1/supplier/categories":
			_, _ = fmt.Fprint(w, `{"items":[{"id":"1","name":"AI"}]}`)
		case "/api/v1/supplier/products":
			_, _ = fmt.Fprint(w, `{"total":1,"items":[{"id":"product-uuid","name":"ChatGPT","description":"Plans","image_urls":["https://example.com/image.png"],"category_names":["AI"],"active":true,"updated_at":"2026-09-08T00:00:00Z","skus":[{"id":"sku-uuid","name":"Go iOS","cost_minor":"4000","stock_quantity":8,"active":true}]}]}`)
		case "/api/v1/supplier/products/product-uuid":
			_, _ = fmt.Fprint(w, `{"product":{"id":"product-uuid","name":"ChatGPT","description":"Plans","image_urls":[],"category_names":["AI"],"active":true,"updated_at":"2026-09-08T00:00:00Z","skus":[{"id":"sku-uuid","name":"Go iOS","cost_minor":"4000","stock_quantity":8,"active":true}]}}`)
		case "/api/v1/supplier/orders":
			var body map[string]interface{}
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatal(err)
			}
			if body["sku_id"] != "sku-uuid" || body["downstream_order_no"] != "ORDER-1" {
				t.Fatalf("unexpected order body: %#v", body)
			}
			_, _ = fmt.Fprint(w, `{"ok":true,"order_id":"order-uuid","status":"processing","amount_minor":"4000","currency":"CNY","currency_decimals":2}`)
		case "/api/v1/supplier/orders/order-uuid":
			_, _ = fmt.Fprint(w, `{"order_id":"order-uuid","status":"supplied","amount_minor":"4000","currency":"CNY","currency_decimals":2,"cards":["gpt-go-ios-AAAA-BBBB-CCCC-DDDD\nhttps://redeem.lsrai.shop"]}`)
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	references := newMemoryReferenceRegistry()
	adapter := NewGMShopEdgeAdapter(&siteconnectiondomain.Connection{
		ID: 7, BaseURL: server.URL, ApiKey: "key-id", ApiSecret: secret,
	}, t.TempDir(), references)

	ping, err := adapter.Ping(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ping.SiteName != "Central" || ping.Balance != "123.45" || ping.ProtocolVersion != "gmshop-edge-v1" {
		t.Fatalf("unexpected ping: %+v", ping)
	}
	categories, err := adapter.ListCategories(context.Background())
	if err != nil || len(categories.Categories) != 1 || categories.Categories[0].ID == 0 {
		t.Fatalf("unexpected categories: %+v err=%v", categories, err)
	}
	products, err := adapter.ListProducts(context.Background(), ListProductsOpts{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if products.Total != 1 || len(products.Items) != 1 || len(products.Items[0].SKUs) != 1 {
		t.Fatalf("unexpected products: %+v", products)
	}
	if products.Items[0].CategoryID != categories.Categories[0].ID {
		t.Fatalf("product category must reuse the category-list identity: product=%d category=%d", products.Items[0].CategoryID, categories.Categories[0].ID)
	}
	product := products.Items[0]
	if product.Title["zh-CN"] != "ChatGPT" || product.SKUs[0].PriceAmount != "40" || product.SKUs[0].StockQuantity != 8 {
		t.Fatalf("unexpected mapped product: %+v", product)
	}

	created, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{
		SKUID: product.SKUs[0].ID, Quantity: 1, DownstreamOrderNo: "ORDER-1", TraceID: "trace-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.OK || created.OrderID == 0 || created.OrderNo != "order-uuid" || created.Amount != "40" || created.Currency != "CNY" {
		t.Fatalf("unexpected created order: %+v", created)
	}
	detail, err := adapter.GetOrder(context.Background(), created.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != "delivered" || detail.Amount != "40" || detail.Currency != "CNY" || detail.Fulfillment == nil || !strings.Contains(detail.Fulfillment.Payload, "https://redeem.lsrai.shop") {
		t.Fatalf("unexpected delivered order: %+v", detail)
	}
	if cards, ok := detail.Fulfillment.DeliveryData["cards"].([]string); !ok || len(cards) != 1 {
		t.Fatalf("delivery cards were not preserved: %#v", detail.Fulfillment.DeliveryData)
	}
}

func TestGMShopEdgeAdapterRejectsEmptyDownstreamOrderBeforeRequest(t *testing.T) {
	adapter := NewGMShopEdgeAdapter(&siteconnectiondomain.Connection{ID: 1}, t.TempDir(), newMemoryReferenceRegistry())
	_, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{DownstreamOrderNo: " "})
	if err == nil || !strings.Contains(err.Error(), "order number") {
		t.Fatalf("expected local validation error, got %v", err)
	}
}

func TestMinorToMajor(t *testing.T) {
	for input, want := range map[string]string{"0": "0", "9": "0.09", "100": "1", "12345": "123.45", "bad": "0"} {
		if got := minorToMajor(input, 2); got != want {
			t.Fatalf("minorToMajor(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMinimumPrice(t *testing.T) {
	if got := minimumPrice([]UpstreamSKU{{PriceAmount: "50"}, {PriceAmount: "40"}, {PriceAmount: "60"}}); got != "40" {
		t.Fatalf("minimum price = %q, want 40", got)
	}
}
