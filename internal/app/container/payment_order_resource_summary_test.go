package container

import (
	"fmt"
	"strings"
	"testing"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fixedConnectionBalance struct {
	values map[uint]*siteconnectionapp.PingResult
}

func (f fixedConnectionBalance) Ping(id uint) (*siteconnectionapp.PingResult, error) {
	result := f.values[id]
	if result == nil {
		return nil, fmt.Errorf("missing connection")
	}
	return result, nil
}

func TestPaymentOrderResourceSummaryReportsCentralStockAndUpstreamBalance(t *testing.T) {
	dsn := fmt.Sprintf("file:payment_resource_summary_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE product_skus (id INTEGER PRIMARY KEY, deleted_at DATETIME)`,
		`CREATE TABLE product_mappings (id INTEGER PRIMARY KEY, connection_id INTEGER, is_active BOOLEAN, deleted_at DATETIME)`,
		`CREATE TABLE sku_mappings (id INTEGER PRIMARY KEY, local_sku_id INTEGER, product_mapping_id INTEGER, upstream_stock INTEGER, upstream_is_active BOOLEAN, deleted_at DATETIME)`,
		`CREATE TABLE site_connections (id INTEGER PRIMARY KEY, name TEXT, protocol TEXT, status TEXT, deleted_at DATETIME)`,
		`CREATE TABLE card_secrets (id INTEGER PRIMARY KEY, sku_id INTEGER, status TEXT, deleted_at DATETIME)`,
		`INSERT INTO product_skus (id) VALUES (21), (22)`,
		`INSERT INTO product_mappings (id, connection_id, is_active) VALUES (31, 41, TRUE), (32, 42, TRUE)`,
		`INSERT INTO sku_mappings (id, local_sku_id, product_mapping_id, upstream_stock, upstream_is_active) VALUES (51, 21, 31, 9, TRUE), (52, 22, 32, 27, TRUE)`,
		`INSERT INTO site_connections (id, name, protocol, status) VALUES (41, '中央仓库', 'gmshop-edge', 'active'), (42, 'Aisou', 'shared-stock', 'active')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
	reader := newPaymentOrderOwnerSummary(db, fixedConnectionBalance{values: map[uint]*siteconnectionapp.PingResult{
		42: {Balance: "1305", Currency: "CNY"},
	}})
	order := &orderdomain.Order{Items: []orderdomain.OrderItem{
		{SKUID: 21, SKUSnapshotJSON: jsonmap.JSON{"spec_values": map[string]interface{}{"name": "ChatGPT Plus 菲区"}}},
		{SKUID: 22, SKUSnapshotJSON: jsonmap.JSON{"spec_values": map[string]interface{}{"name": "ChatGPT Pro 20X 菲区"}}},
	}}
	if got := orderResourceLabel(order.Items[0]); got != "ChatGPT Plus 菲区" {
		t.Fatalf("unexpected resource label: %q (%T)", got, order.Items[0].SKUSnapshotJSON["spec_values"])
	}
	summary := reader.resourceSummary(order)
	if !strings.Contains(summary, "ChatGPT Plus 菲区：中央库存 9") {
		t.Fatalf("missing central stock: %s", summary)
	}
	if !strings.Contains(summary, "ChatGPT Pro 20X 菲区：Aisou额度 ¥1305") {
		t.Fatalf("missing upstream balance: %s", summary)
	}
}
