package container

import (
	"fmt"
	"sort"
	"strings"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"gorm.io/gorm"
)

type connectionBalanceReader interface {
	Ping(id uint) (*siteconnectionapp.PingResult, error)
}

type paymentOrderOwnerSummary struct {
	db          *gorm.DB
	connections connectionBalanceReader
}

type paymentOrderResourceRow struct {
	SKUID         uint   `gorm:"column:sku_id"`
	ConnectionID  uint   `gorm:"column:connection_id"`
	Protocol      string `gorm:"column:protocol"`
	Connection    string `gorm:"column:connection"`
	UpstreamStock int    `gorm:"column:upstream_stock"`
	LocalStock    int    `gorm:"column:local_stock"`
}

func newPaymentOrderOwnerSummary(db *gorm.DB, connections connectionBalanceReader) *paymentOrderOwnerSummary {
	return &paymentOrderOwnerSummary{db: db, connections: connections}
}

func (r *paymentOrderOwnerSummary) Summary(order *orderdomain.Order, payment *paymentdomain.Payment, locale string) (string, string) {
	return r.resourceSummary(order), r.financialSummary(order, payment, locale)
}

func (r *paymentOrderOwnerSummary) resourceSummary(order *orderdomain.Order) string {
	if r == nil || r.db == nil || order == nil || len(order.Items) == 0 {
		return ""
	}
	labels := make(map[uint]string)
	ids := make([]uint, 0, len(order.Items))
	for _, item := range order.Items {
		if item.SKUID == 0 {
			continue
		}
		if _, exists := labels[item.SKUID]; exists {
			continue
		}
		labels[item.SKUID] = orderResourceLabel(item)
		ids = append(ids, item.SKUID)
	}
	if len(ids) == 0 {
		return ""
	}
	var rows []paymentOrderResourceRow
	if err := r.db.Raw(`
		SELECT ps.id AS sku_id,
		 COALESCE(pm.connection_id, 0) AS connection_id,
		 COALESCE(sc.protocol, '') AS protocol,
		 COALESCE(sc.name, '') AS connection,
		 COALESCE(sm.upstream_stock, 0) AS upstream_stock,
		 (SELECT COUNT(*) FROM card_secrets cs
		  WHERE cs.sku_id = ps.id AND cs.status = 'available'
		   AND cs.deleted_at IS NULL) AS local_stock
		FROM product_skus ps
		LEFT JOIN sku_mappings sm ON sm.local_sku_id = ps.id
		 AND sm.deleted_at IS NULL AND sm.upstream_is_active = TRUE
		LEFT JOIN product_mappings pm ON pm.id = sm.product_mapping_id
		 AND pm.deleted_at IS NULL AND pm.is_active = TRUE
		LEFT JOIN site_connections sc ON sc.id = pm.connection_id
		 AND sc.deleted_at IS NULL AND sc.status = 'active'
		WHERE ps.id IN ? AND ps.deleted_at IS NULL`, ids).Scan(&rows).Error; err != nil {
		return ""
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].SKUID < rows[j].SKUID })
	balances := make(map[uint]string)
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		label := labels[row.SKUID]
		switch strings.TrimSpace(row.Protocol) {
		case "gmshop-edge":
			lines = append(lines, fmt.Sprintf("%s：中央库存 %d", label, row.UpstreamStock))
		case "shared-stock", "dujiao-next":
			balance, exists := balances[row.ConnectionID]
			if !exists {
				balance = "查询失败"
				if r.connections != nil {
					if result, err := r.connections.Ping(row.ConnectionID); err == nil && result != nil {
						balance = formatUpstreamBalance(result.Balance, result.Currency)
					}
				}
				balances[row.ConnectionID] = balance
			}
			name := strings.TrimSpace(row.Connection)
			if name == "" {
				name = "上游"
			}
			lines = append(lines, fmt.Sprintf("%s：%s额度 %s", label, name, balance))
		default:
			lines = append(lines, fmt.Sprintf("%s：本地库存 %d", label, row.LocalStock))
		}
	}
	return strings.Join(lines, "\n")
}

func orderResourceLabel(item orderdomain.OrderItem) string {
	var specs map[string]interface{}
	switch value := item.SKUSnapshotJSON["spec_values"].(type) {
	case map[string]interface{}:
		specs = value
	case jsonmap.JSON:
		specs = map[string]interface{}(value)
	}
	if specs != nil {
		for _, key := range []string{"name", "zh-CN", "zh-TW", "en-US"} {
			if value := strings.TrimSpace(fmt.Sprint(specs[key])); value != "" && value != "<nil>" {
				return value
			}
		}
	}
	for _, key := range []string{"zh-CN", "zh-TW", "en-US", "en"} {
		if value := strings.TrimSpace(fmt.Sprint(item.TitleJSON[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return fmt.Sprintf("SKU#%d", item.SKUID)
}

func formatUpstreamBalance(balance, currency string) string {
	balance = strings.TrimSpace(balance)
	if balance == "" {
		return "查询失败"
	}
	if strings.EqualFold(strings.TrimSpace(currency), "CNY") {
		return "¥" + balance
	}
	if currency = strings.ToUpper(strings.TrimSpace(currency)); currency != "" {
		return currency + " " + balance
	}
	return balance
}
