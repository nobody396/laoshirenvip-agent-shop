package wallethttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type tenantWalletServiceStub struct{}

func (tenantWalletServiceStub) GetAccount(uint) (*walletdomain.Account, error) {
	return &walletdomain.Account{Balance: money.FromDecimal(decimal.NewFromInt(999))}, nil
}
func (tenantWalletServiceStub) GetResellerAccount(resellerID, userID uint) (*walletdomain.ResellerAccount, error) {
	return &walletdomain.ResellerAccount{ResellerID: resellerID, UserID: userID, Balance: money.FromDecimal(decimal.NewFromInt(33))}, nil
}
func (tenantWalletServiceStub) ListTransactions(uint, int, int) ([]walletdomain.Transaction, int64, error) {
	return nil, 0, nil
}
func (tenantWalletServiceStub) ListResellerTransactions(uint, uint, int, int) ([]walletdomain.ResellerTransaction, int64, error) {
	return nil, 0, nil
}
func (tenantWalletServiceStub) ListUserRechargeOrders(uint, int, int, string, string) ([]walletdomain.RechargeOrder, int64, error) {
	return nil, 0, nil
}
func (tenantWalletServiceStub) StatsUserRechargeOrders(uint, string) (map[string]int64, error) {
	return nil, nil
}
func (tenantWalletServiceStub) GetRechargeOrderByRechargeNo(uint, string) (*walletdomain.RechargeOrder, error) {
	return nil, nil
}
func (tenantWalletServiceStub) GetRechargeOrderByPaymentIDAndUser(uint, uint) (*walletdomain.RechargeOrder, error) {
	return nil, nil
}

type tenantPaymentServiceStub struct{ channelCalls *int }

func (s tenantPaymentServiceStub) GetAvailableWalletRechargeChannels(money.Amount, *userdomain.User) ([]map[string]interface{}, error) {
	if s.channelCalls != nil {
		(*s.channelCalls)++
	}
	return nil, nil
}
func (tenantPaymentServiceStub) CreateWalletRechargePayment(CreateRechargePaymentInput) (*CreateRechargePaymentResult, error) {
	return nil, nil
}

func TestResellerWalletHasNoSelfRechargeChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	calls := 0
	handler := NewUserHandler(tenantWalletServiceStub{}, tenantPaymentServiceStub{channelCalls: &calls}, tenantUserReaderStub{}, nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/wallet/payment-channels", strings.NewReader(`{"amount":"10"}`))
	req.Header.Set("Content-Type", "application/json")
	tenant := resellercontract.ResellerTenantContext("nova.example.test", 7, 70, "nova.example.test")
	c.Request = req.WithContext(resellercontract.WithTenantContext(context.Background(), tenant))
	c.Set("user_id", uint(22))

	handler.GetPaymentChannels(c)
	if calls != 0 {
		t.Fatalf("tenant wallet queried global payment channels %d times", calls)
	}
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"data":[]`) {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
func (tenantPaymentServiceStub) GetPayment(uint) (*paymentdomain.Payment, error) { return nil, nil }
func (tenantPaymentServiceStub) CapturePayment(CapturePaymentInput) (*paymentdomain.Payment, error) {
	return nil, nil
}

type tenantUserReaderStub struct{}

func (tenantUserReaderStub) GetByID(uint) (*userdomain.User, error) { return &userdomain.User{}, nil }

func TestGetWalletUsesTheCurrentResellerTenantAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewUserHandler(tenantWalletServiceStub{}, tenantPaymentServiceStub{}, tenantUserReaderStub{}, nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/wallet", nil)
	tenant := resellercontract.ResellerTenantContext("nova.example.test", 7, 70, "nova.example.test")
	req = req.WithContext(resellercontract.WithTenantContext(context.Background(), tenant))
	c.Request = req
	c.Set("user_id", uint(22))

	handler.GetWallet(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Balance money.Amount `json:"balance"`
			Scope   string       `json:"scope"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.Balance.String() != "33.00" || payload.Data.Scope != "reseller" {
		t.Fatalf("unexpected tenant wallet response: %+v", payload.Data)
	}
}
