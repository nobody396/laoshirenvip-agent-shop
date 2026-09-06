package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/dujiao-next/internal/constants"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

type memoryReferenceRegistry struct {
	next  uint
	byKey map[string]uint
	byID  map[uint]string
}

func TestSharedStockAdapterUsesStableUniqueRequestNumberWithinUpstreamLimit(t *testing.T) {
	var mu sync.Mutex
	var requestNumbers []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/shared/commodity/trade" {
			http.NotFound(w, req)
			return
		}
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		requestNumbers = append(requestNumbers, req.PostForm.Get("request_no"))
		mu.Unlock()
		_, _ = fmt.Fprint(w, `{"code":200,"data":{"amount":"37.00","tradeNo":"ORDER-A","secret":"CARD-A"}}`)
	}))
	defer server.Close()

	references := newMemoryReferenceRegistry()
	skuID, _ := references.Resolve(1, siteconnectiondomain.ExternalReferenceKindSKU, "SKU-A")
	adapter := NewSharedStockAdapter(&siteconnectiondomain.Connection{
		ID: 1, BaseURL: server.URL, ApiKey: "42", ApiSecret: "secret",
	}, t.TempDir(), references)

	const firstOrder = "DJ20260906052047400695-01"
	for _, orderNo := range []string{firstOrder, firstOrder, "DJ20260906052047400695-02"} {
		if _, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{
			SKUID: skuID, Quantity: 1, DownstreamOrderNo: orderNo,
		}); err != nil {
			t.Fatal(err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requestNumbers) != 3 {
		t.Fatalf("request count = %d", len(requestNumbers))
	}
	for _, requestNo := range requestNumbers {
		if len(requestNo) == 0 || len(requestNo) > 19 {
			t.Fatalf("request_no must contain 1-19 characters, got %q (%d)", requestNo, len(requestNo))
		}
	}
	if requestNumbers[0] != requestNumbers[1] {
		t.Fatalf("same order produced unstable request_no values: %q != %q", requestNumbers[0], requestNumbers[1])
	}
	if requestNumbers[0] == requestNumbers[2] {
		t.Fatalf("different orders produced the same request_no: %q", requestNumbers[0])
	}
}

func TestSharedStockAdapterRejectsEmptyLocalOrderNumber(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		_, _ = fmt.Fprint(w, `{"code":200,"data":{"tradeNo":"UNEXPECTED"}}`)
	}))
	defer server.Close()

	references := newMemoryReferenceRegistry()
	skuID, _ := references.Resolve(1, siteconnectiondomain.ExternalReferenceKindSKU, "SKU-A")
	adapter := NewSharedStockAdapter(&siteconnectiondomain.Connection{
		ID: 1, BaseURL: server.URL, ApiKey: "42", ApiSecret: "secret",
	}, t.TempDir(), references)

	_, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{
		SKUID: skuID, Quantity: 1, DownstreamOrderNo: " ",
	})
	if err == nil {
		t.Fatal("expected empty local order number to be rejected")
	}
	if requests != 0 {
		t.Fatalf("empty local order number reached upstream: %d requests", requests)
	}
}

func TestSharedStockAdapterPackagesCredentialURLAndUsageMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/shared/commodity/trade" {
			http.NotFound(w, req)
			return
		}
		_, _ = fmt.Fprint(w, `{"code":200,"data":{"url":"https://redeem.example/activate","amount":"115.00","tradeNo":"AISOU-1","secret":"PLUS-CDK-123"}}`)
	}))
	defer server.Close()

	references := newMemoryReferenceRegistry()
	skuID, _ := references.Resolve(1, siteconnectiondomain.ExternalReferenceKindSKU, "PLUS-PH")
	adapter := NewSharedStockAdapter(&siteconnectiondomain.Connection{
		ID: 1, BaseURL: server.URL, ApiKey: "42", ApiSecret: "secret",
	}, t.TempDir(), references)

	created, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{
		SKUID: skuID, Quantity: 1, DownstreamOrderNo: "DJ20260906052047400695-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Fulfillment == nil {
		t.Fatal("expected immediate fulfillment")
	}
	if got := created.Fulfillment.Payload; got != "PLUS-CDK-123\nhttps://redeem.example/activate" {
		t.Fatalf("raw supplier payload changed: %q", got)
	}
	data := created.Fulfillment.DeliveryData
	if got := data["note"]; got != "使用方法：复制 CDK 卡密，打开充值网址，按页面提示提交即可。" {
		t.Fatalf("usage method = %#v", got)
	}
	entries, ok := data["entries"].([]map[string]string)
	if !ok {
		t.Fatalf("entries type = %T", data["entries"])
	}
	wantEntries := []map[string]string{
		{"key": "CDK 卡密", "value": "PLUS-CDK-123"},
		{"key": "充值网址", "value": "https://redeem.example/activate"},
	}
	if fmt.Sprint(entries) != fmt.Sprint(wantEntries) {
		t.Fatalf("entries = %#v, want %#v", entries, wantEntries)
	}
	cards, ok := data["cards"].([]string)
	if !ok || len(cards) != 1 || cards[0] != created.Fulfillment.Payload {
		t.Fatalf("single-item delivery must remain one card: %#v", data["cards"])
	}
}

func newMemoryReferenceRegistry() *memoryReferenceRegistry {
	return &memoryReferenceRegistry{next: 1, byKey: map[string]uint{}, byID: map[uint]string{}}
}

func (r *memoryReferenceRegistry) Resolve(_ uint, kind, externalKey string) (uint, error) {
	key := kind + ":" + externalKey
	if id := r.byKey[key]; id > 0 {
		return id, nil
	}
	id := r.next
	r.next++
	r.byKey[key] = id
	r.byID[id] = externalKey
	return id, nil
}

func (r *memoryReferenceRegistry) Lookup(_ uint, _ string, id uint) (string, error) {
	return r.byID[id], nil
}

func TestSharedVariantsPreferFactoryPricesFromINI(t *testing.T) {
	raw, err := json.Marshal("[category]\n250点=80\n500点=160\n[category_factory]\n250点=75\n500点=145\n")
	if err != nil {
		t.Fatal(err)
	}
	got := sharedVariants(raw)
	if got["250点"] != "75" || got["500点"] != "145" || len(got) != 2 {
		t.Fatalf("unexpected variants: %#v", got)
	}
}

func TestSharedStockAdapterRequiresPersistentReferenceRegistry(t *testing.T) {
	_, err := NewAdapter(&siteconnectiondomain.Connection{Protocol: constants.ConnectionProtocolSharedStock}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "reference registry") {
		t.Fatalf("expected registry requirement, got %v", err)
	}
}

func TestUnknownAdapterProtocolIsRejected(t *testing.T) {
	_, err := NewAdapter(&siteconnectiondomain.Connection{Protocol: "unknown"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "unsupported protocol") {
		t.Fatalf("expected unsupported protocol error, got %v", err)
	}
}

func TestSharedStockGetProductPreservesCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/shared/commodity/items":
			_, _ = fmt.Fprint(w, `{"code":200,"data":[{"id":7,"name":"AI","children":[{"id":23,"code":"SKU-A","name":"Plan","price":"37","stock":8,"status":1}]}]}`)
		case "/shared/commodity/item":
			_, _ = fmt.Fprint(w, `{"code":200,"data":{"id":23,"code":"SKU-A","name":"Plan","price":"37","stock":8,"status":1}}`)
		case "/shared/commodity/trade":
			if err := req.ParseForm(); err != nil {
				t.Fatal(err)
			}
			form := req.PostForm
			if got := form.Get("contact"); !strings.HasPrefix(got, "order-") || !strings.HasSuffix(got, "@lsrai.shop") {
				t.Fatalf("unexpected generated contact: %q", got)
			}
			if got := form.Get("password"); len(got) < 6 {
				t.Fatalf("generated password is too short: %q", got)
			}
			_, _ = fmt.Fprint(w, `{"code":200,"data":{"amount":"37.00","tradeNo":"ORDER-A","secret":"CARD-A"}}`)
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	references := newMemoryReferenceRegistry()
	productID, _ := references.Resolve(1, siteconnectiondomain.ExternalReferenceKindProduct, "SKU-A")
	adapter := NewSharedStockAdapter(&siteconnectiondomain.Connection{
		ID: 1, BaseURL: server.URL, ApiKey: "42", ApiSecret: "secret",
	}, t.TempDir(), references)
	categories, err := adapter.ListCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(categories.Categories) != 1 || categories.Categories[0].Slug == "" {
		t.Fatalf("expected stable non-empty category slug: %+v", categories)
	}
	product, err := adapter.GetProduct(context.Background(), productID)
	if err != nil {
		t.Fatal(err)
	}
	if product.CategoryID == 0 {
		t.Fatalf("expected stable non-zero category id: %+v", product)
	}
	skuID, _ := references.Resolve(1, siteconnectiondomain.ExternalReferenceKindSKU, "SKU-A")
	created, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{
		SKUID: skuID, Quantity: 1, DownstreamOrderNo: "LOCAL-A",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "delivered" || created.Fulfillment == nil || created.Fulfillment.Payload != "CARD-A" {
		t.Fatalf("expected immediate SharedStock delivery: %+v", created)
	}
}
