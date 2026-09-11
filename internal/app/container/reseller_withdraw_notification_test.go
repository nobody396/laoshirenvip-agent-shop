package container

import (
	"fmt"
	"strings"
	"testing"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type withdrawNotificationQueueStub struct {
	inputs []notificationcontract.EnqueueInput
}

func (s *withdrawNotificationQueueStub) Enqueue(input notificationcontract.EnqueueInput) error {
	s.inputs = append(s.inputs, input)
	return nil
}

func TestResellerWithdrawNotifierBuildsExactSKUCommissionBreakdown(t *testing.T) {
	dsn := fmt.Sprintf("file:withdraw_notification_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{}, &resellerdomain.Profile{}, &resellerdomain.SiteConfig{}, &resellerdomain.Domain{},
		&orderdomain.Order{}, &orderdomain.OrderItem{}, &resellerdomain.OrderSnapshot{},
		&resellerdomain.WithdrawRequest{}, &resellerdomain.LedgerEntry{},
	); err != nil {
		t.Fatal(err)
	}
	user := userdomain.User{Email: "agent@example.com", DisplayName: "代理甲", PasswordHash: "hash", Status: "active"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	profile := resellerdomain.Profile{UserID: user.ID, Status: resellerdomain.ProfileStatusActive}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&resellerdomain.SiteConfig{ResellerID: profile.ID, SiteName: "甲站"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&resellerdomain.Domain{ResellerID: profile.ID, Domain: "a.lsrai.shop", Status: "active", VerificationStatus: "verified", IsPrimary: true}).Error; err != nil {
		t.Fatal(err)
	}
	request := resellerdomain.WithdrawRequest{
		ID: 77, ResellerID: profile.ID, Amount: money.FromDecimal(decimal.NewFromInt(5)), Currency: "CNY",
		Channel: "支付宝", Account: "hidden", Status: resellerdomain.WithdrawStatusPending,
	}
	if err := db.Create(&request).Error; err != nil {
		t.Fatal(err)
	}
	order := orderdomain.Order{ID: 100, OrderNo: "DJ-100", UserID: user.ID, Status: "completed", Currency: "CNY", ResellerID: &profile.ID}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	items := []orderdomain.OrderItem{
		{ID: 201, OrderID: order.ID, ProductID: 1, SKUID: 11, TitleJSON: jsonmap.JSON{"zh-CN": "商品A"}, SKUSnapshotJSON: jsonmap.JSON{"spec_values": map[string]interface{}{"name": "SKU-A"}}, Quantity: 1, FulfillmentType: "auto"},
		{ID: 202, OrderID: order.ID, ProductID: 2, SKUID: 12, TitleJSON: jsonmap.JSON{"zh-CN": "商品B"}, SKUSnapshotJSON: jsonmap.JSON{"spec_values": map[string]interface{}{"name": "SKU-B"}}, Quantity: 1, FulfillmentType: "auto"},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	snapshot := resellerdomain.OrderSnapshot{
		OrderID: order.ID, ResellerID: profile.ID, Domain: "a.lsrai.shop", Currency: "CNY",
		BaseAmount: money.FromDecimal(decimal.NewFromInt(10)), ResellerAmount: money.FromDecimal(decimal.NewFromInt(16)),
		ProfitAmount: money.FromDecimal(decimal.NewFromInt(6)), ProfitEligible: true,
		PricingSnapshotJSON: jsonmap.JSON{"items": []interface{}{
			map[string]interface{}{"order_item_id": items[0].ID, "sku_id": items[0].SKUID, "profit_amount": "2.00"},
			map[string]interface{}{"order_item_id": items[1].ID, "sku_id": items[1].SKUID, "profit_amount": "4.00"},
		}},
	}
	if err := db.Create(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	withdrawID := request.ID
	orderID := order.ID
	ledger := resellerdomain.LedgerEntry{
		ResellerID: profile.ID, OrderID: &orderID, Type: resellerdomain.LedgerTypeOrderProfit,
		Amount: money.FromDecimal(decimal.NewFromInt(5)), Currency: "CNY", IdempotencyKey: "locked:100",
		Status: resellerdomain.LedgerStatusLocked, WithdrawRequestID: &withdrawID,
	}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatal(err)
	}

	queue := &withdrawNotificationQueueStub{}
	if err := newResellerWithdrawNotifier(db, queue).NotifyWithdrawApplied(request.ID); err != nil {
		t.Fatal(err)
	}
	if len(queue.inputs) != 1 {
		t.Fatalf("expected one notification, got %d", len(queue.inputs))
	}
	message := fmt.Sprint(queue.inputs[0].Data["message"])
	for _, want := range []string{
		"提现单：#77", "申请人：代理甲（agent@example.com）", "站点：甲站（a.lsrai.shop）",
		"提现金额：¥5.00", "SKU-A｜佣金 ¥1.67｜1 笔订单", "SKU-B｜佣金 ¥3.33｜1 笔订单", "合计：¥5.00",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q:\n%s", want, message)
		}
	}
	if strings.Contains(message, "hidden") {
		t.Fatalf("payout account leaked into notification: %s", message)
	}
}

func TestAllocateMoneyPreservesTotal(t *testing.T) {
	got := allocateMoney(decimal.RequireFromString("10.01"), []decimal.Decimal{
		decimal.NewFromInt(1), decimal.NewFromInt(1), decimal.NewFromInt(1),
	})
	total := decimal.Zero
	for _, item := range got {
		total = total.Add(item)
	}
	if !total.Equal(decimal.RequireFromString("10.01")) {
		t.Fatalf("allocation drifted: %#v total=%s", got, total)
	}
}
