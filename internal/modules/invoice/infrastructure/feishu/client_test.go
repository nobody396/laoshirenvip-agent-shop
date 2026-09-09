package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestListReadyInvoicesParsesFeishuRichTextCells(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "test-token"})
		case "/open-apis/bitable/v1/apps/base/tables/table/records/search":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"items": []any{map[string]any{
				"record_id": "rec-ready",
				"fields": map[string]any{
					"申请编号":  []any{map[string]any{"text": "INV-READY", "type": "text"}},
					"发票号码":  []any{map[string]any{"text": "FP-001", "type": "text"}},
					"发票PDF": []any{map[string]any{"file_token": "file-token", "name": "invoice.pdf"}},
					"开票日期":  time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC).UnixMilli(),
				},
			}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(Config{AppID: "app", AppSecret: "secret", BaseToken: "base", TableID: "table"})
	client.baseURL = server.URL
	items, err := client.ListReadyInvoices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RequestNo != "INV-READY" || items[0].InvoiceNumber != "FP-001" || items[0].FileToken != "file-token" {
		t.Fatalf("unexpected ready invoices: %+v", items)
	}
}
