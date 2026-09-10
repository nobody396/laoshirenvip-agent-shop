package container

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestPaymentOrderFinancialSummaryUsesImmutableOrderAndPaymentSnapshots(t *testing.T) {
	dsn := fmt.Sprintf("file:payment_financial_summary_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&resellerdomain.OrderSnapshot{}); err != nil {
		t.Fatal(err)
	}
	resellerID := uint(8)
	order := &orderdomain.Order{
		ID:               175,
		OrderNo:          "DJ20260910151057552747",
		ResellerID:       &resellerID,
		Currency:         "CNY",
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		Items: []orderdomain.OrderItem{{
			TitleJSON:       jsonmap.JSON{"zh-CN": "ChatGPT会员充值"},
			SKUSnapshotJSON: jsonmap.JSON{"spec_values": map[string]interface{}{"name": "ChatGPT Pro 20X 菲区 1个月"}},
			CostPrice:       money.FromDecimal(decimal.NewFromInt(1050)),
			Quantity:        1,
		}},
	}
	snapshot := &resellerdomain.OrderSnapshot{
		OrderID:        order.ID,
		ResellerID:     resellerID,
		Domain:         "777.lsrai.shop",
		Currency:       "CNY",
		BaseAmount:     money.FromDecimal(decimal.NewFromInt(1065)),
		ResellerAmount: money.FromDecimal(decimal.NewFromInt(1100)),
		ProfitAmount:   money.FromDecimal(decimal.NewFromInt(35)),
		ProfitEligible: true,
	}
	if err := db.Create(snapshot).Error; err != nil {
		t.Fatal(err)
	}
	payment := &paymentdomain.Payment{
		ProviderType: constants.PaymentProviderEpay,
		Amount:       money.FromDecimal(decimal.NewFromInt(1100)),
		FeeAmount:    money.FromDecimal(decimal.NewFromInt(33)),
		FeePolicy:    constants.PaymentFeePolicyMerchantAbsorbed,
	}

	summary := newPaymentOrderOwnerSummary(db, nil).financialSummary(order, payment, constants.LocaleZhCN)
	for _, want := range []string{
		"商品：",
		"ChatGPT Pro 20X 菲区 1个月 ×1｜成本 ¥1050.00",
		"我们的成本价：¥1050.00",
		"代理供货价：¥1065.00",
		"代理卖价：¥1100.00",
		"用户实际支付：¥1100.00",
		"手续费：¥33.00（代理承担）",
		"代理利润：¥2.00",
		"我们的利润：¥15.00",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
}

func TestPaymentOrderFinancialSummaryAddsCustomerSurchargeToActualPayment(t *testing.T) {
	dsn := fmt.Sprintf("file:payment_financial_surcharge_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&resellerdomain.OrderSnapshot{}); err != nil {
		t.Fatal(err)
	}
	resellerID := uint(6)
	order := &orderdomain.Order{ID: 177, ResellerID: &resellerID, Currency: "CNY", Items: []orderdomain.OrderItem{{CostPrice: money.FromDecimal(decimal.NewFromInt(115)), Quantity: 1}}}
	if err := db.Create(&resellerdomain.OrderSnapshot{OrderID: order.ID, ResellerID: resellerID, Domain: "999.lsrai.shop", Currency: "CNY", BaseAmount: money.FromDecimal(decimal.NewFromInt(117)), ResellerAmount: money.FromDecimal(decimal.NewFromInt(128)), ProfitAmount: money.FromDecimal(decimal.NewFromInt(11)), ProfitEligible: true}).Error; err != nil {
		t.Fatal(err)
	}
	payment := &paymentdomain.Payment{ProviderType: constants.PaymentProviderEpay, Amount: money.FromDecimal(decimal.RequireFromString("133.12")), FeeAmount: money.FromDecimal(decimal.RequireFromString("5.12")), FeePolicy: constants.PaymentFeePolicyCustomerSurcharge}
	summary := newPaymentOrderOwnerSummary(db, nil).financialSummary(order, payment, constants.LocaleZhCN)
	for _, want := range []string{"用户实际支付：¥133.12", "手续费：¥5.12（用户承担）", "代理利润：¥11.00", "我们的利润：¥2.00"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
}

func TestPaymentOrderFinancialSummaryUsesWalletLedgerAndProcurementEvidence(t *testing.T) {
	dsn := fmt.Sprintf("file:payment_financial_evidence_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&resellerdomain.OrderSnapshot{}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE wallet_transactions (id INTEGER PRIMARY KEY, order_id INTEGER, type TEXT, direction TEXT, balance_after NUMERIC, currency TEXT, created_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE site_connections (id INTEGER PRIMARY KEY, protocol TEXT, deleted_at DATETIME)`,
		`CREATE TABLE procurement_orders (id INTEGER PRIMARY KEY, local_order_id INTEGER, connection_id INTEGER, deleted_at DATETIME)`,
		`CREATE TABLE card_secrets (id INTEGER PRIMARY KEY, order_id INTEGER, status TEXT, deleted_at DATETIME)`,
		`INSERT INTO wallet_transactions (id,order_id,type,direction,balance_after,currency,created_at) VALUES (1,175,'order_pay','out',777,'CNY',1)`,
		`INSERT INTO site_connections (id,protocol) VALUES (2,'gmshop-edge')`,
		`INSERT INTO procurement_orders (id,local_order_id,connection_id) VALUES (1,176,2)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
	resellerID := uint(8)
	order := &orderdomain.Order{
		ID: 175, ResellerID: &resellerID, Currency: "CNY",
		WalletPaidAmount: money.FromDecimal(decimal.NewFromInt(1100)),
		Items:            []orderdomain.OrderItem{{OrderID: 176, CostPrice: money.FromDecimal(decimal.NewFromInt(1050)), Quantity: 1, FulfillmentType: constants.FulfillmentTypeUpstream}},
	}
	if err := db.Create(&resellerdomain.OrderSnapshot{OrderID: order.ID, ResellerID: resellerID, Domain: "777.lsrai.shop", Currency: "CNY", BaseAmount: money.FromDecimal(decimal.NewFromInt(1065)), ResellerAmount: money.FromDecimal(decimal.NewFromInt(1100)), ProfitAmount: money.FromDecimal(decimal.NewFromInt(35)), ProfitEligible: true}).Error; err != nil {
		t.Fatal(err)
	}
	payment := &paymentdomain.Payment{ProviderType: constants.PaymentProviderWallet, Amount: money.FromDecimal(decimal.NewFromInt(1100)), FeePolicy: constants.PaymentFeePolicyNone}
	summary := newPaymentOrderOwnerSummary(db, nil).financialSummary(order, payment, constants.LocaleZhCN)
	for _, want := range []string{"用户钱包剩余额度：¥777.00", "CDK来源：老实人VIP中央仓", "我们的成本价：以VIP供货通知为准", "我们的利润：以VIP供货通知为准"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
}
