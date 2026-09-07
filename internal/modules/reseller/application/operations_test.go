package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	"github.com/dujiao-next/internal/shared/money"

	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
	"github.com/shopspring/decimal"
)

type operationsStoreStub struct {
	overview resellercontract.OperationsOverviewRow
	finance  resellercontract.OperationsFinanceRowSet
}

func (s operationsStoreStub) GetOverview(startAt, endAt time.Time) (resellercontract.OperationsOverviewRow, error) {
	return s.overview, nil
}

func (s operationsStoreStub) GetFinance(startAt, endAt time.Time) (resellercontract.OperationsFinanceRowSet, error) {
	return s.finance, nil
}

func TestResellerOperationsServiceOverviewBuildsAlertsAndFormatsAverage(t *testing.T) {
	svc := NewOperationsService(operationsStoreStub{
		overview: resellercontract.OperationsOverviewRow{
			Lifecycle: resellercontract.OperationsLifecycleRow{
				ProfilesPendingReview:           2,
				DomainsPendingReview:            1,
				ActiveProfilesWithoutSiteConfig: 3,
			},
			Orders: resellercontract.OperationsOrdersRow{
				OrdersTotal:               10,
				PaidOrders:                6,
				ActiveResellersWithOrders: 4,
				SelfDealingBlockedOrders:  1,
			},
		},
	})
	resp, err := svc.GetOverview(context.Background(), reportingdomain.Query{Range: "today", Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("GetOverview failed: %v", err)
	}
	if resp.Orders.AveragePaidOrdersPerActiveReseller != "1.50" {
		t.Fatalf("unexpected average: %s", resp.Orders.AveragePaidOrdersPerActiveReseller)
	}
	if len(resp.Alerts) != 4 {
		t.Fatalf("expected four alerts, got %+v", resp.Alerts)
	}
	if !strings.HasSuffix(resp.To, "T23:59:59+08:00") {
		t.Fatalf("expected inclusive end-of-day to timestamp, got %s", resp.To)
	}
}

func TestResellerOperationsServiceFinanceFormatsCurrencyRows(t *testing.T) {
	svc := NewOperationsService(operationsStoreStub{
		finance: resellercontract.OperationsFinanceRowSet{
			PeriodCurrencyRows: []resellercontract.OperationsPeriodCurrencyRow{{
				Currency:       "usd",
				GMVPaid:        decimal.RequireFromString("120"),
				ProfitEarned:   decimal.RequireFromString("30"),
				RefundDeducted: decimal.RequireFromString("4"),
			}},
			CurrentCurrencyRows: []resellercontract.OperationsCurrentCurrencyRow{{
				Currency:              "usd",
				AvailableBalance:      decimal.RequireFromString("26"),
				PendingWithdrawAmount: decimal.RequireFromString("8"),
				PendingWithdrawCount:  1,
			}},
		},
	})
	resp, err := svc.GetFinance(context.Background(), reportingdomain.Query{Range: "today", Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("GetFinance failed: %v", err)
	}
	if resp.PeriodCurrencyRows[0].Currency != "USD" || resp.PeriodCurrencyRows[0].GMVPaid != "120.00" {
		t.Fatalf("unexpected period row: %+v", resp.PeriodCurrencyRows[0])
	}
	if resp.CurrentCurrencyRows[0].PendingWithdrawAmount != "8.00" {
		t.Fatalf("unexpected current row: %+v", resp.CurrentCurrencyRows[0])
	}
}

var _ resellercontract.OperationsStore = operationsStoreStub{}

type customerPriceStoreStub struct {
	profile  *resellerdomain.Profile
	product  *resellercontract.ProductSettingProductRow
	settings []resellerdomain.CustomerPriceSetting
}

func (s *customerPriceStoreStub) WithinCustomerPriceSettingTransaction(run func(resellercontract.CustomerPriceSettingStore) error) error {
	return run(s)
}
func (s *customerPriceStoreStub) GetProfileByUserID(userID uint) (*resellerdomain.Profile, error) {
	if s.profile == nil || s.profile.UserID != userID {
		return nil, nil
	}
	return s.profile, nil
}
func (s *customerPriceStoreStub) GetProductWithSettings(resellerID, productID uint) (*resellercontract.ProductSettingProductRow, error) {
	if s.product == nil || s.product.Product.ID != productID {
		return nil, nil
	}
	return s.product, nil
}
func (s *customerPriceStoreStub) ListCustomerPriceSettings(resellerID, customerUserID uint) ([]resellerdomain.CustomerPriceSetting, error) {
	return append([]resellerdomain.CustomerPriceSetting(nil), s.settings...), nil
}
func (s *customerPriceStoreStub) ListCustomerPriceSettingsForPricing(resellerID, customerUserID uint, productIDs, skuIDs []uint) ([]resellerdomain.CustomerPriceSetting, error) {
	return append([]resellerdomain.CustomerPriceSetting(nil), s.settings...), nil
}
func (s *customerPriceStoreStub) UpsertCustomerPriceSetting(setting resellerdomain.CustomerPriceSetting) (*resellerdomain.CustomerPriceSetting, error) {
	setting.ID = 91
	s.settings = []resellerdomain.CustomerPriceSetting{setting}
	return &setting, nil
}
func (s *customerPriceStoreStub) DeleteCustomerPriceSetting(resellerID, customerUserID, productID, skuID uint) error {
	s.settings = nil
	return nil
}

type customerPriceUsersStub struct{ users map[uint]*userdomain.User }

func (s customerPriceUsersStub) GetByID(userID uint) (*userdomain.User, error) {
	user := s.users[userID]
	if user == nil {
		return nil, errors.New("not found")
	}
	return user, nil
}
func (s customerPriceUsersStub) List(filter usercontract.ListFilter) ([]userdomain.User, int64, error) {
	return nil, 0, nil
}

func newCustomerPriceServiceFixture() (*CustomerPriceService, *customerPriceStoreStub) {
	resellerID := uint(7)
	store := &customerPriceStoreStub{
		profile: &resellerdomain.Profile{
			ID: resellerID, UserID: 11, Status: resellerdomain.ProfileStatusActive,
			MaxMarkupPercent: money.FromDecimal(decimal.NewFromInt(100)),
		},
		product: &resellercontract.ProductSettingProductRow{
			Product: productdomain.Product{
				ID: 21, IsActive: true,
				SKUs: []productdomain.ProductSKU{{
					ID: 31, ProductID: 21, IsActive: true,
					PriceAmount:     money.FromDecimal(decimal.NewFromInt(120)),
					CostPriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
				}},
			},
			Settings: []resellerdomain.ProductSetting{{
				ID: 41, ResellerID: resellerID, ProductID: 21, SKUID: 31, IsListed: true,
				PricingMode:      resellerdomain.PricingModeFixedPrice,
				FixedPriceAmount: money.FromDecimal(decimal.NewFromInt(130)),
			}},
		},
	}
	users := customerPriceUsersStub{users: map[uint]*userdomain.User{
		51: {ID: 51, Email: "customer@example.com", Status: constants.UserStatusActive, RegistrationResellerID: &resellerID},
		52: {ID: 52, Email: "other@example.com", Status: constants.UserStatusActive},
	}}
	return NewCustomerPriceService(store, users), store
}

func TestCustomerPriceServiceSetsFixedSKUPriceWithinFloorAndRetail(t *testing.T) {
	service, store := newCustomerPriceServiceFixture()
	quote, err := service.Set(11, 51, 21, 31, decimal.NewFromInt(122))
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if quote.Setting == nil || quote.Setting.ID != 91 || quote.Setting.CustomerUserID != 51 {
		t.Fatalf("unexpected setting: %+v", quote.Setting)
	}
	if !quote.BasePrice.Equal(decimal.NewFromInt(120)) || !quote.RetailPrice.Equal(decimal.NewFromInt(130)) || !quote.GrossProfit.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("unexpected quote: %+v", quote)
	}
	list, err := service.List(11, 51)
	if err != nil || len(list.Settings) != 1 || len(store.settings) != 1 {
		t.Fatalf("unexpected list: result=%+v err=%v", list, err)
	}
}

func TestCustomerPriceServiceRejectsCrossResellerAndUnsafePrices(t *testing.T) {
	service, _ := newCustomerPriceServiceFixture()
	if _, err := service.Set(11, 52, 21, 31, decimal.NewFromInt(122)); !errors.Is(err, resellercontract.ErrCustomerNotFound) {
		t.Fatalf("cross-reseller error = %v", err)
	}
	if _, err := service.Set(11, 51, 21, 31, decimal.NewFromInt(119)); !errors.Is(err, resellercontract.ErrPriceBelowBase) {
		t.Fatalf("below-base error = %v", err)
	}
	if _, err := service.Set(11, 51, 21, 31, decimal.NewFromInt(131)); !errors.Is(err, resellercontract.ErrCustomerPriceAboveRetail) {
		t.Fatalf("above-retail error = %v", err)
	}
	if _, err := service.Set(11, 51, 21, 999, decimal.NewFromInt(122)); !errors.Is(err, resellercontract.ErrCustomerPriceInvalid) {
		t.Fatalf("invalid sku error = %v", err)
	}
}

func TestCustomerPriceServiceDeleteRestoresInheritance(t *testing.T) {
	service, store := newCustomerPriceServiceFixture()
	if _, err := service.Set(11, 51, 21, 31, decimal.NewFromInt(122)); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if err := service.Delete(11, 51, 21, 31); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(store.settings) != 0 {
		t.Fatalf("settings were not deleted: %+v", store.settings)
	}
}
