package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

type memoryReferenceRegistry struct {
	next  uint
	byKey map[string]uint
	byID  map[uint]string
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
