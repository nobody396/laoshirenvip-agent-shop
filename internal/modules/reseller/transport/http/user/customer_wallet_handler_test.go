package userhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	resellerapp "github.com/dujiao-next/internal/modules/reseller/application"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type customerWalletServiceStub struct {
	input resellerapp.CustomerWalletTopUpInput
}

func (s *customerWalletServiceStub) TopUp(input resellerapp.CustomerWalletTopUpInput) (*resellerapp.CustomerWalletTopUpResult, error) {
	s.input = input
	return &resellerapp.CustomerWalletTopUpResult{
		OwnerWallet:    &walletdomain.Account{Balance: money.FromDecimal(decimal.NewFromInt(90))},
		CustomerWallet: &walletdomain.ResellerAccount{Balance: money.FromDecimal(decimal.NewFromInt(10))},
		CustomerTransaction: &walletdomain.ResellerTransaction{
			ID: 91, Amount: money.FromDecimal(decimal.NewFromInt(10)),
			BalanceBefore: money.FromDecimal(decimal.Zero), BalanceAfter: money.FromDecimal(decimal.NewFromInt(10)),
		},
	}, nil
}
func (s *customerWalletServiceStub) ListCustomers(uint, int, int, string) ([]resellerapp.CustomerWalletListRow, int64, error) {
	return nil, 0, nil
}
func (s *customerWalletServiceStub) ListCustomerTransactions(uint, uint, int, int) ([]walletdomain.ResellerTransaction, int64, error) {
	return nil, 0, nil
}

func TestCustomerWalletTopUpUsesAuthenticatedResellerOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &customerWalletServiceStub{}
	handler := NewUserCustomerWalletHandler(service)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/reseller/customers/22/wallet/topup", strings.NewReader(`{"amount":"10","request_id":"topup-http-001","remark":"微信收款"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "22"}}
	c.Set("user_id", uint(11))

	handler.TopUp(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.input.OwnerUserID != 11 || service.input.CustomerUserID != 22 || service.input.Reference != "topup-http-001" || service.input.Amount.String() != "10.00" {
		t.Fatalf("unexpected top-up input: %+v", service.input)
	}
	for _, expected := range []string{`"transaction_id":91`, `"balance_before":"0.00"`, `"balance_after":"10.00"`, `"owner_wallet_balance":"90.00"`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("response missing %s: %s", expected, recorder.Body.String())
		}
	}
}
