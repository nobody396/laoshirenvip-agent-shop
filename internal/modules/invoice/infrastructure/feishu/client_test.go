package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/modules/invoice/domain"
)

func TestUpsertPaidRequestIncludesFixedInvoiceItem(t *testing.T) {
	var createdFields map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "test-token"})
		case "/open-apis/bitable/v1/apps/base/tables/table/records/search":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"items": []any{}}})
		case "/open-apis/bitable/v1/apps/base/tables/table/records":
			var payload struct {
				Fields map[string]any `json:"fields"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			createdFields = payload.Fields
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"record": map[string]any{"record_id": "rec-test"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(Config{AppID: "app", AppSecret: "secret", BaseToken: "base", TableID: "table"})
	client.baseURL = server.URL
	if _, err := client.UpsertPaidRequest(context.Background(), &domain.Request{RequestNo: "INV-TEST"}); err != nil {
		t.Fatal(err)
	}
	if got := createdFields["开票项目"]; got != domain.InvoiceItemName {
		t.Fatalf("开票项目 = %v, want %q", got, domain.InvoiceItemName)
	}
}
