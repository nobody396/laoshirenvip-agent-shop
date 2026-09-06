package application

import (
	"testing"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestNeutralProfitStatusUnavailableWhenIneligible(t *testing.T) {
	status := neutralProfitStatus(resellerdomain.OrderSnapshot{
		ProfitEligible: false,
		ProfitAmount:   money.FromDecimal(decimal.NewFromInt(10)),
	}, orderdomain.Order{Status: constants.OrderStatusPaid}, nil)
	if status != resellercontract.ProfitStatusUnavailable {
		t.Fatalf("expected unavailable, got %s", status)
	}
}

func TestMaskBuyerEmail(t *testing.T) {
	if got := maskBuyerEmail("ashang@example.com"); got != "a***@example.com" {
		t.Fatalf("unexpected mask: %s", got)
	}
}

func TestBuildOrderListItemUsesNetProfitLedgerAndFeeSnapshot(t *testing.T) {
	row := resellercontract.OrderSnapshotRow{
		Snapshot: resellerdomain.OrderSnapshot{
			Currency:     "CNY",
			BaseAmount:   money.FromDecimal(decimal.NewFromInt(100)),
			ProfitAmount: money.FromDecimal(decimal.NewFromInt(50)),
		},
		Order: orderdomain.Order{
			OrderNo:     "DJ-NET-PROFIT",
			Status:      constants.OrderStatusPaid,
			TotalAmount: money.FromDecimal(decimal.NewFromInt(150)),
		},
		LedgerEntries: []resellerdomain.LedgerEntry{{
			Type:   resellerdomain.LedgerTypeOrderProfit,
			Amount: money.FromDecimal(decimal.NewFromInt(44)),
			MetadataJSON: map[string]interface{}{
				"gross_profit_amount": "50.00",
				"payment_fee_amount":  "6.00",
				"net_profit_amount":   "44.00",
			},
		}},
	}

	got := buildOrderListItem(row)
	if got.GrossProfitAmount.StringFixed(2) != "50.00" || got.PaymentFeeAmount.StringFixed(2) != "6.00" || got.ProfitAmount.StringFixed(2) != "44.00" {
		t.Fatalf("unexpected reseller profit breakdown: gross=%s fee=%s net=%s", got.GrossProfitAmount.StringFixed(2), got.PaymentFeeAmount.StringFixed(2), got.ProfitAmount.StringFixed(2))
	}
}

func TestOrderQueryServiceRejectsInactiveProfile(t *testing.T) {
	svc := NewOrderQueryService(orderQueryStoreStub{profile: &resellerdomain.Profile{
		ID:     1,
		UserID: 9,
		Status: resellerdomain.ProfileStatusDisabled,
	}})
	_, _, err := svc.ListUserOrders(9, resellercontract.OrderListInput{Page: 1, PageSize: 10})
	if err != resellercontract.ErrProfileInactive {
		t.Fatalf("expected profile inactive, got %v", err)
	}
}

type orderQueryStoreStub struct {
	profile *resellerdomain.Profile
}

func (s orderQueryStoreStub) GetProfileByUserID(userID uint) (*resellerdomain.Profile, error) {
	return s.profile, nil
}
func (s orderQueryStoreStub) GetProfileByID(id uint) (*resellerdomain.Profile, error) {
	return s.profile, nil
}
func (s orderQueryStoreStub) ListOrderSnapshotsByReseller(filter resellercontract.OrderSnapshotListFilter) ([]resellercontract.OrderSnapshotRow, int64, error) {
	return nil, 0, nil
}
func (s orderQueryStoreStub) StatsOrderSnapshotsByReseller(filter resellercontract.OrderSnapshotListFilter) (resellercontract.OrderStatsRow, error) {
	return resellercontract.OrderStatsRow{}, nil
}
func (s orderQueryStoreStub) GetOrderSnapshotByResellerOrderNo(resellerID uint, orderNo string) (*resellercontract.OrderSnapshotRow, error) {
	return nil, nil
}
