package container

import (
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"

	"github.com/shopspring/decimal"
)

func (r *paymentOrderOwnerSummary) financialSummary(order *orderdomain.Order, payment *paymentdomain.Payment, locale string) string {
	if r == nil || r.db == nil || order == nil || order.ID == 0 || order.ResellerID == nil || *order.ResellerID == 0 {
		return ""
	}
	var snapshot resellerdomain.OrderSnapshot
	if err := r.db.Where("order_id = ? AND deleted_at IS NULL", order.ID).First(&snapshot).Error; err != nil {
		return ""
	}

	cost := decimal.Zero
	items := make([]string, 0, len(order.Items))
	for index, item := range order.Items {
		quantity := item.Quantity
		if quantity < 0 {
			return ""
		}
		itemCost := item.CostPrice.Decimal.Mul(decimal.NewFromInt(int64(quantity))).Round(2)
		cost = cost.Add(itemCost)
		items = append(items, formatFinancialItem(locale, index+1, orderResourceLabel(item), quantity, itemCost, snapshot.Currency))
	}
	cost = cost.Round(2)
	supply := snapshot.BaseAmount.Decimal.Round(2)
	sell := snapshot.ResellerAmount.Decimal.Round(2)
	agentGross := snapshot.ProfitAmount.Decimal.Round(2)
	fee := decimal.Zero
	feePolicy := constants.PaymentFeePolicyNone
	if payment != nil {
		fee = payment.FeeAmount.Decimal.Round(2)
		feePolicy = strings.TrimSpace(payment.FeePolicy)
	}
	agentNet := agentGross
	if feePolicy == constants.PaymentFeePolicyMerchantAbsorbed {
		agentNet = agentGross.Sub(fee).Round(2)
		if agentNet.IsNegative() {
			agentNet = decimal.Zero
		}
	}
	actualPaid := order.WalletPaidAmount.Decimal.Round(2)
	if payment != nil {
		if strings.EqualFold(payment.ProviderType, constants.PaymentProviderWallet) {
			actualPaid = payment.Amount.Decimal.Round(2)
		} else {
			actualPaid = actualPaid.Add(payment.Amount.Decimal).Round(2)
		}
	} else {
		actualPaid = actualPaid.Add(order.OnlinePaidAmount.Decimal).Round(2)
	}
	ownerProfit := supply.Sub(cost).Round(2)
	currency := strings.ToUpper(strings.TrimSpace(snapshot.Currency))
	if currency == "" {
		currency = strings.ToUpper(strings.TrimSpace(order.Currency))
	}
	walletRemaining := financialNotApplicable(locale)
	if order.WalletPaidAmount.Decimal.IsPositive() {
		walletRemaining = financialNeedsReview(locale)
		if amount, walletCurrency, ok := r.walletBalanceAfter(order.ID); ok {
			walletRemaining = formatFinancialMoney(amount, walletCurrency)
		}
	}
	cdkSource, centralWarehouse := r.cdkSource(order, locale)
	costText := formatFinancialMoney(cost, currency)
	ownerProfitText := formatFinancialMoney(ownerProfit, currency)
	if centralWarehouse {
		costText = localizedFinancialText(locale, "以VIP供货通知为准", "以VIP供貨通知為準", "see VIP fulfillment alert")
		ownerProfitText = costText
	}

	labels := financialLabels(locale)
	lines := []string{labels.items + "："}
	if len(items) == 0 {
		lines = append(lines, labels.noItems)
	} else {
		lines = append(lines, items...)
	}
	lines = append(lines,
		labels.cost+"："+costText,
		labels.supply+"："+formatFinancialMoney(supply, currency),
		labels.sell+"："+formatFinancialMoney(sell, currency),
		labels.actualPaid+"："+formatFinancialMoney(actualPaid, currency),
		labels.fee+"："+formatFinancialMoney(fee, currency)+"（"+financialFeePayer(locale, feePolicy, fee)+"）",
		labels.walletRemaining+"："+walletRemaining,
		labels.cdkSource+"："+cdkSource,
		labels.agentProfit+"："+formatFinancialMoney(agentNet, currency),
		labels.ownerProfit+"："+ownerProfitText,
	)
	return strings.Join(lines, "\n")
}

type financialLabelSet struct {
	items, noItems, cost, supply, sell, actualPaid, fee, walletRemaining, cdkSource, agentProfit, ownerProfit string
}

func financialLabels(locale string) financialLabelSet {
	switch settingsmessaging.NormalizeNotificationLocale(locale) {
	case constants.LocaleEnUS:
		return financialLabelSet{"Items", "No item details", "Our cost", "Supply price", "Reseller price", "Customer actually paid", "Payment fee", "Customer wallet remaining", "CDK source", "Reseller profit", "Our profit"}
	case constants.LocaleZhTW:
		return financialLabelSet{"商品", "暫無商品明細", "我們的成本價", "代理供貨價", "代理售價", "用戶實際支付", "手續費", "用戶錢包剩餘額度", "CDK來源", "代理利潤", "我們的利潤"}
	default:
		return financialLabelSet{"商品", "暂无商品明细", "我们的成本价", "代理供货价", "代理卖价", "用户实际支付", "手续费", "用户钱包剩余额度", "CDK来源", "代理利润", "我们的利润"}
	}
}

func (r *paymentOrderOwnerSummary) walletBalanceAfter(orderID uint) (decimal.Decimal, string, bool) {
	var row struct {
		BalanceAfter decimal.Decimal `gorm:"column:balance_after"`
		Currency     string          `gorm:"column:currency"`
	}
	err := r.db.Raw(`SELECT balance_after, currency FROM wallet_transactions
		WHERE order_id = ? AND type = ? AND direction = ? AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC LIMIT 1`, orderID, constants.WalletTxnTypeOrderPay, constants.WalletTxnDirectionOut).Scan(&row).Error
	if err != nil || strings.TrimSpace(row.Currency) == "" {
		return decimal.Zero, "", false
	}
	return row.BalanceAfter.Round(2), row.Currency, true
}

func (r *paymentOrderOwnerSummary) cdkSource(order *orderdomain.Order, locale string) (string, bool) {
	orderIDs := make([]uint, 0, len(order.Items)+1)
	seen := map[uint]struct{}{}
	for _, item := range order.Items {
		id := item.OrderID
		if id == 0 {
			id = order.ID
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		orderIDs = append(orderIDs, id)
	}
	if len(orderIDs) == 0 {
		orderIDs = append(orderIDs, order.ID)
	}
	var rows []struct {
		Protocol string `gorm:"column:protocol"`
	}
	_ = r.db.Raw(`SELECT DISTINCT COALESCE(sc.protocol, '') AS protocol
		FROM procurement_orders po
		LEFT JOIN site_connections sc ON sc.id = po.connection_id AND sc.deleted_at IS NULL
		WHERE po.local_order_id IN ? AND po.deleted_at IS NULL`, orderIDs).Scan(&rows).Error
	hasCentral := false
	hasSupplierWallet := false
	for _, row := range rows {
		if strings.TrimSpace(row.Protocol) == constants.ConnectionProtocolGMShopEdge {
			hasCentral = true
		} else if strings.TrimSpace(row.Protocol) != "" {
			hasSupplierWallet = true
		}
	}
	var localCount int64
	_ = r.db.Raw(`SELECT COUNT(*) FROM card_secrets
		WHERE order_id IN ? AND status IN ('reserved', 'used') AND deleted_at IS NULL`, orderIDs).Scan(&localCount).Error
	parts := make([]string, 0, 3)
	if localCount > 0 {
		parts = append(parts, localizedFinancialText(locale, "内置库存", "內置庫存", "built-in inventory"))
	}
	if hasCentral {
		parts = append(parts, localizedFinancialText(locale, "老实人VIP中央仓", "老實人VIP中央倉", "VIP central warehouse"))
	}
	if hasSupplierWallet {
		parts = append(parts, localizedFinancialText(locale, "钱包额度下单", "錢包額度下單", "supplier wallet order"))
	}
	if len(parts) > 0 {
		return strings.Join(parts, "＋"), hasCentral
	}
	for _, item := range order.Items {
		switch strings.TrimSpace(item.FulfillmentType) {
		case constants.FulfillmentTypeAuto:
			return localizedFinancialText(locale, "内置库存", "內置庫存", "built-in inventory"), false
		case constants.FulfillmentTypeUpstream:
			return localizedFinancialText(locale, "上游采购待确认", "上游採購待確認", "upstream source pending"), false
		case constants.FulfillmentTypeManual:
			return localizedFinancialText(locale, "人工交付", "人工交付", "manual fulfillment"), false
		}
	}
	return financialNeedsReview(locale), false
}

func financialNotApplicable(locale string) string {
	return localizedFinancialText(locale, "不适用", "不適用", "not applicable")
}

func financialNeedsReview(locale string) string {
	return localizedFinancialText(locale, "待核对", "待核對", "needs review")
}

func localizedFinancialText(locale, zhCN, zhTW, enUS string) string {
	switch settingsmessaging.NormalizeNotificationLocale(locale) {
	case constants.LocaleEnUS:
		return enUS
	case constants.LocaleZhTW:
		return zhTW
	default:
		return zhCN
	}
}

func formatFinancialItem(locale string, index int, label string, quantity int, cost decimal.Decimal, currency string) string {
	if strings.TrimSpace(label) == "" {
		label = fmt.Sprintf("SKU#%d", index)
	}
	costLabel := "成本"
	if settingsmessaging.NormalizeNotificationLocale(locale) == constants.LocaleEnUS {
		costLabel = "cost"
	}
	return fmt.Sprintf("%d. %s ×%d｜%s %s", index, label, quantity, costLabel, formatFinancialMoney(cost, currency))
}

func formatFinancialMoney(value decimal.Decimal, currency string) string {
	amount := value.Round(2).StringFixed(2)
	if strings.EqualFold(strings.TrimSpace(currency), "CNY") {
		return "¥" + amount
	}
	if currency = strings.ToUpper(strings.TrimSpace(currency)); currency != "" {
		return currency + " " + amount
	}
	return amount
}

func financialFeePayer(locale, policy string, fee decimal.Decimal) string {
	if !fee.IsPositive() {
		switch settingsmessaging.NormalizeNotificationLocale(locale) {
		case constants.LocaleEnUS:
			return "no fee"
		case constants.LocaleZhTW:
			return "無手續費"
		default:
			return "无手续费"
		}
	}
	var zhCN, zhTW, enUS string
	switch policy {
	case constants.PaymentFeePolicyCustomerSurcharge:
		zhCN, zhTW, enUS = "用户承担", "用戶承擔", "paid by customer"
	case constants.PaymentFeePolicyMerchantAbsorbed:
		zhCN, zhTW, enUS = "代理承担", "代理承擔", "paid by reseller"
	default:
		zhCN, zhTW, enUS = "平台承担", "平台承擔", "paid by platform"
	}
	switch settingsmessaging.NormalizeNotificationLocale(locale) {
	case constants.LocaleEnUS:
		return enUS
	case constants.LocaleZhTW:
		return zhTW
	default:
		return zhCN
	}
}
