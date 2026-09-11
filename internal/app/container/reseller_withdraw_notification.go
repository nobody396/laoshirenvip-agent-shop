package container

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type resellerWithdrawNotifier struct {
	db            *gorm.DB
	notifications notificationcontract.NotificationEnqueuer
}

type withdrawNotificationHeader struct {
	ID          uint      `gorm:"column:id"`
	ResellerID  uint      `gorm:"column:reseller_id"`
	Amount      string    `gorm:"column:amount"`
	Currency    string    `gorm:"column:currency"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	DisplayName string    `gorm:"column:display_name"`
	Email       string    `gorm:"column:email"`
	SiteName    string    `gorm:"column:site_name"`
	Domain      string    `gorm:"column:domain"`
}

type withdrawNotificationLedger struct {
	OrderID *uint  `gorm:"column:order_id"`
	Type    string `gorm:"column:type"`
	Amount  string `gorm:"column:amount"`
}

type withdrawSKUContribution struct {
	SKUID    uint
	Label    string
	Amount   decimal.Decimal
	OrderIDs map[uint]struct{}
}

type withdrawPricingItem struct {
	OrderItemID uint
	SKUID       uint
	Profit      decimal.Decimal
}

func newResellerWithdrawNotifier(db *gorm.DB, notifications notificationcontract.NotificationEnqueuer) *resellerWithdrawNotifier {
	return &resellerWithdrawNotifier{db: db, notifications: notifications}
}

func (n *resellerWithdrawNotifier) NotifyWithdrawApplied(withdrawID uint) error {
	if n == nil || n.db == nil || n.notifications == nil || withdrawID == 0 {
		return nil
	}
	header, detail, err := n.buildMessage(withdrawID)
	if err != nil {
		return err
	}
	return n.notifications.Enqueue(notificationcontract.EnqueueInput{
		EventType: constants.NotificationEventResellerWithdrawRequested,
		BizType:   constants.NotificationBizTypeResellerWithdraw,
		BizID:     withdrawID,
		Locale:    constants.LocaleZhCN,
		Data: jsonmap.JSON{
			"message":     detail,
			"withdraw_id": header.ID,
		},
	})
}

func (n *resellerWithdrawNotifier) buildMessage(withdrawID uint) (withdrawNotificationHeader, string, error) {
	var header withdrawNotificationHeader
	err := n.db.Raw(`SELECT w.id, w.reseller_id, CAST(w.amount AS text) AS amount,
		w.currency, w.created_at, COALESCE(u.display_name, '') AS display_name,
		COALESCE(u.email, '') AS email, COALESCE(sc.site_name, '') AS site_name,
		COALESCE((SELECT d.domain FROM reseller_domains d
			WHERE d.reseller_id = w.reseller_id AND d.deleted_at IS NULL
			ORDER BY d.is_primary DESC, (d.status = 'active') DESC,
				(d.verification_status = 'verified') DESC, d.id ASC LIMIT 1), '') AS domain
		FROM reseller_withdraw_requests w
		JOIN reseller_profiles rp ON rp.id = w.reseller_id AND rp.deleted_at IS NULL
		JOIN users u ON u.id = rp.user_id AND u.deleted_at IS NULL
		LEFT JOIN reseller_site_configs sc ON sc.reseller_id = w.reseller_id AND sc.deleted_at IS NULL
		WHERE w.id = ? AND w.deleted_at IS NULL`, withdrawID).Scan(&header).Error
	if err != nil {
		return header, "", err
	}
	if header.ID == 0 {
		return header, "", gorm.ErrRecordNotFound
	}

	var ledgers []withdrawNotificationLedger
	if err := n.db.Raw(`SELECT order_id, type, CAST(amount AS text) AS amount
		FROM reseller_ledger_entries
		WHERE withdraw_request_id = ? AND status = ? AND deleted_at IS NULL
		ORDER BY id ASC`, withdrawID, resellerdomain.LedgerStatusLocked).Scan(&ledgers).Error; err != nil {
		return header, "", err
	}
	contributions, fallbackDomain, other := n.skuContributions(ledgers)
	if header.Domain == "" {
		header.Domain = fallbackDomain
	}

	applicant := strings.TrimSpace(header.DisplayName)
	if applicant == "" {
		applicant = strings.TrimSpace(header.Email)
	} else if email := strings.TrimSpace(header.Email); email != "" {
		applicant += "（" + email + "）"
	}
	if applicant == "" {
		applicant = fmt.Sprintf("用户#%d", header.ResellerID)
	}
	site := strings.TrimSpace(header.SiteName)
	if site == "" {
		site = "未命名子站"
	}
	if domain := strings.TrimSpace(header.Domain); domain != "" {
		site += "（" + domain + "）"
	}
	amount, _ := decimal.NewFromString(header.Amount)
	lines := []string{
		fmt.Sprintf("提现单：#%d", header.ID),
		"申请人：" + applicant,
		"站点：" + site,
		"提现金额：" + formatFinancialMoney(amount, header.Currency),
		"申请时间：" + header.CreatedAt.In(time.FixedZone("CST", 8*60*60)).Format("2006-01-02 15:04:05") + "（北京时间）",
		"",
		"佣金明细：",
	}
	for index, item := range contributions {
		orderCount := len(item.OrderIDs)
		lines = append(lines, fmt.Sprintf("%d. %s｜佣金 %s｜%d 笔订单", index+1, item.Label, formatFinancialMoney(item.Amount, header.Currency), orderCount))
	}
	if other.IsPositive() {
		lines = append(lines, fmt.Sprintf("%d. 其他可提现流水｜%s", len(contributions)+1, formatFinancialMoney(other, header.Currency)))
	}
	if len(contributions) == 0 && !other.IsPositive() {
		lines = append(lines, "暂无可识别的佣金来源明细")
	}
	lines = append(lines, "合计："+formatFinancialMoney(amount, header.Currency))
	return header, strings.Join(lines, "\n"), nil
}

func (n *resellerWithdrawNotifier) skuContributions(ledgers []withdrawNotificationLedger) ([]withdrawSKUContribution, string, decimal.Decimal) {
	selectedByOrder := make(map[uint]decimal.Decimal)
	other := decimal.Zero
	for _, ledger := range ledgers {
		amount, err := decimal.NewFromString(ledger.Amount)
		if err != nil || !amount.IsPositive() {
			continue
		}
		if ledger.Type != resellerdomain.LedgerTypeOrderProfit || ledger.OrderID == nil || *ledger.OrderID == 0 {
			other = other.Add(amount)
			continue
		}
		selectedByOrder[*ledger.OrderID] = selectedByOrder[*ledger.OrderID].Add(amount)
	}
	if len(selectedByOrder) == 0 {
		return nil, "", other.Round(2)
	}

	orderIDs := make([]uint, 0, len(selectedByOrder))
	for orderID := range selectedByOrder {
		orderIDs = append(orderIDs, orderID)
	}
	var snapshots []resellerdomain.OrderSnapshot
	if err := n.db.Where("order_id IN ? AND deleted_at IS NULL", orderIDs).Find(&snapshots).Error; err != nil {
		for _, amount := range selectedByOrder {
			other = other.Add(amount)
		}
		return nil, "", other.Round(2)
	}

	itemsByOrder := make(map[uint][]withdrawPricingItem, len(snapshots))
	itemIDs := make([]uint, 0)
	fallbackDomain := ""
	for _, snapshot := range snapshots {
		if fallbackDomain == "" {
			fallbackDomain = strings.TrimSpace(snapshot.Domain)
		}
		items := pricingItems(snapshot.PricingSnapshotJSON)
		itemsByOrder[snapshot.OrderID] = items
		for _, item := range items {
			if item.OrderItemID != 0 {
				itemIDs = append(itemIDs, item.OrderItemID)
			}
		}
	}
	labels := make(map[uint]string)
	if len(itemIDs) > 0 {
		var orderItems []orderdomain.OrderItem
		if err := n.db.Where("id IN ? AND deleted_at IS NULL", itemIDs).Find(&orderItems).Error; err == nil {
			for _, item := range orderItems {
				labels[item.ID] = orderResourceLabel(item)
			}
		}
	}

	grouped := make(map[string]*withdrawSKUContribution)
	for orderID, selected := range selectedByOrder {
		items := itemsByOrder[orderID]
		weights := make([]decimal.Decimal, 0, len(items))
		usable := make([]withdrawPricingItem, 0, len(items))
		for _, item := range items {
			if item.Profit.IsPositive() {
				usable = append(usable, item)
				weights = append(weights, item.Profit)
			}
		}
		if len(usable) == 0 {
			other = other.Add(selected)
			continue
		}
		allocated := allocateMoney(selected, weights)
		for index, item := range usable {
			label := strings.TrimSpace(labels[item.OrderItemID])
			if label == "" {
				label = fmt.Sprintf("SKU#%d", item.SKUID)
			}
			key := fmt.Sprintf("%d:%s", item.SKUID, label)
			row := grouped[key]
			if row == nil {
				row = &withdrawSKUContribution{SKUID: item.SKUID, Label: label, OrderIDs: map[uint]struct{}{}}
				grouped[key] = row
			}
			row.Amount = row.Amount.Add(allocated[index]).Round(2)
			row.OrderIDs[orderID] = struct{}{}
		}
	}

	result := make([]withdrawSKUContribution, 0, len(grouped))
	for _, item := range grouped {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].Amount.Equal(result[j].Amount) {
			return result[i].Amount.GreaterThan(result[j].Amount)
		}
		return result[i].Label < result[j].Label
	})
	return result, fallbackDomain, other.Round(2)
}

func pricingItems(snapshot jsonmap.JSON) []withdrawPricingItem {
	rawItems, ok := snapshot["items"].([]interface{})
	if !ok {
		return nil
	}
	items := make([]withdrawPricingItem, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]interface{})
		if !ok {
			if asJSON, ok := raw.(jsonmap.JSON); ok {
				item = map[string]interface{}(asJSON)
			} else {
				continue
			}
		}
		profit, err := decimal.NewFromString(strings.TrimSpace(fmt.Sprint(item["profit_amount"])))
		if err != nil {
			continue
		}
		items = append(items, withdrawPricingItem{
			OrderItemID: uintFromSnapshot(item["order_item_id"]),
			SKUID:       uintFromSnapshot(item["sku_id"]),
			Profit:      profit.Round(2),
		})
	}
	return items
}

func uintFromSnapshot(value interface{}) uint {
	parsed, err := decimal.NewFromString(strings.TrimSpace(fmt.Sprint(value)))
	if err != nil || parsed.IsNegative() {
		return 0
	}
	return uint(parsed.IntPart())
}

func allocateMoney(total decimal.Decimal, weights []decimal.Decimal) []decimal.Decimal {
	result := make([]decimal.Decimal, len(weights))
	remaining := total.Round(2)
	remainingWeight := decimal.Zero
	for _, weight := range weights {
		if weight.IsPositive() {
			remainingWeight = remainingWeight.Add(weight)
		}
	}
	for index, weight := range weights {
		if !weight.IsPositive() || !remaining.IsPositive() || !remainingWeight.IsPositive() {
			continue
		}
		share := remaining
		if index < len(weights)-1 {
			share = remaining.Mul(weight).Div(remainingWeight).Round(2)
			if share.GreaterThan(remaining) {
				share = remaining
			}
		}
		result[index] = share
		remaining = remaining.Sub(share).Round(2)
		remainingWeight = remainingWeight.Sub(weight)
	}
	return result
}
