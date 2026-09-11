package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/invoice/domain"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type requestStoreStub struct {
	item             *domain.Request
	walletCalls      int
	walletUserID     uint
	walletResellerID *uint
}

func (s *requestStoreStub) Create(item *domain.Request) error { s.item = item; return nil }
func (s *requestStoreStub) Save(item *domain.Request) error   { s.item = item; return nil }
func (s *requestStoreStub) CreateAndPayWithWallet(item *domain.Request, userID uint, resellerID *uint) error {
	s.item = item
	s.walletCalls++
	s.walletUserID = userID
	s.walletResellerID = resellerID
	now := time.Now()
	item.Status = domain.StatusPendingIssue
	item.ProviderRef = "invoice:" + item.RequestNo + ":wallet"
	item.PaidAt = &now
	return nil
}
func (s *requestStoreStub) GetByRequestNo(string) (*domain.Request, error) {
	return s.item, nil
}
func (s *requestStoreStub) GetByOriginalOrder(_, _, _ string) (*domain.Request, error) {
	return s.item, nil
}
func (s *requestStoreStub) MarkPaid(_ string, providerRef string, paidAt time.Time) (bool, *domain.Request, error) {
	if s.item == nil {
		return false, nil, nil
	}
	if s.item.Status != domain.StatusPendingPayment {
		return false, s.item, nil
	}
	s.item.Status = domain.StatusPendingIssue
	s.item.ProviderRef = providerRef
	s.item.PaidAt = &paidAt
	return true, s.item, nil
}
func (s *requestStoreStub) SetFeishuSync(_ string, recordID, lastError string) error {
	if s.item != nil {
		s.item.FeishuRecordID = recordID
		s.item.FeishuLastError = lastError
	}
	return nil
}
func (s *requestStoreStub) ListPendingFeishu(_ int) ([]domain.Request, error) {
	if s.item == nil || s.item.PaidAt == nil || s.item.FeishuRecordID != "" {
		return nil, nil
	}
	return []domain.Request{*s.item}, nil
}

type channelStoreStub struct{ item *paymentdomain.PaymentChannel }

func (s channelStoreStub) GetByID(uint) (*paymentdomain.PaymentChannel, error) { return s.item, nil }

type gatewayStub struct {
	input    paymentcontract.GatewayCreateInput
	callback *paymentcontract.GatewayCallbackResult
}

func (g *gatewayStub) VerifyCallback(jsonmap.JSON, map[string][]string, []byte) (*paymentcontract.GatewayCallbackResult, error) {
	return g.callback, nil
}

func (g *gatewayStub) Type() string                              { return "epay" }
func (g *gatewayStub) ValidateConfig(jsonmap.JSON, string) error { return nil }
func (g *gatewayStub) CreatePayment(_ context.Context, _ jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	g.input = input
	return &paymentcontract.GatewayCreateResult{ProviderRef: "provider-1", QRCodeURL: "https://pay.example/qr"}, nil
}

type registryStub struct {
	gateway paymentcontract.GatewayProvider
}

func (r registryStub) Lookup(_, _ string) (paymentcontract.GatewayProvider, bool) {
	return r.gateway, true
}

func TestCreateUsesDeclaredInvoiceAmountAndPassesAlipayFeeToApplicant(t *testing.T) {
	store := &requestStoreStub{}
	gateway := &gatewayStub{}
	service := NewService(store, channelStoreStub{item: &paymentdomain.PaymentChannel{
		ID: 2, ProviderType: "epay", ChannelType: "alipay", InteractionMode: "qr", IsActive: true,
		FeeRate: money.FromDecimal(decimal.RequireFromString("4.00")),
	}}, registryStub{gateway: gateway}, "https://lsrai.shop")

	request, err := service.Create(context.Background(), CreateInput{
		Source: "dujiao", SourceHost: "vip.lsrai.shop", OriginalOrderNo: "DJ-1",
		OriginalAmount: money.FromDecimal(decimal.RequireFromString("117.00")), InvoiceAmount: money.FromDecimal(decimal.RequireFromString("130.00")), InvoiceType: domain.TypeOrdinary,
		BuyerTitle: "示例公司", TaxNumber: "91350000TEST", RecipientEmail: "finance@example.com", ClientIP: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if request.OriginalAmount.String() != "117.00" || request.InvoiceFeeAmount.String() != "3.90" || request.InvoiceTotalAmount.String() != "130.00" {
		t.Fatalf("unexpected invoice amounts: %+v", request)
	}
	if request.PaymentFeeRate.String() != "4.00" || request.PaymentFeeAmount.String() != "0.16" || request.PaymentAmount.String() != "4.06" {
		t.Fatalf("unexpected payment amounts: %+v", request)
	}
	if gateway.input.OrderNo != request.RequestNo || gateway.input.Amount.String() != "4.06" || gateway.input.NotifyURL != "https://lsrai.shop/api/v1/invoices/payment/callback" {
		t.Fatalf("unexpected gateway input: %+v", gateway.input)
	}
	if gateway.input.ReturnURL != "https://lsrai.shop/invoice?request_no="+request.RequestNo {
		t.Fatalf("unexpected return URL: %s", gateway.input.ReturnURL)
	}
}

func TestCreateWithWalletPaysExactSupplementWithoutGateway(t *testing.T) {
	store := &requestStoreStub{}
	gateway := &gatewayStub{}
	service := NewService(store, channelStoreStub{}, registryStub{gateway: gateway}, "https://lsrai.shop")
	resellerID := uint(18)
	request, err := service.Create(context.Background(), CreateInput{
		Source: "dujiao", SourceHost: "agi.lsrai.shop", OriginalOrderNo: "DJ-WALLET-1",
		OriginalAmount: money.FromDecimal(decimal.RequireFromString("117.00")), InvoiceAmount: money.FromDecimal(decimal.RequireFromString("130.00")), InvoiceType: domain.TypeOrdinary,
		BuyerTitle: "示例公司", TaxNumber: "91350000TEST", RecipientEmail: "finance@example.com", ClientIP: "127.0.0.1",
		PaymentMethod: domain.PaymentMethodWallet, UserID: 7, WalletResellerID: &resellerID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if request.Status != domain.StatusPendingIssue || request.PaymentAmount.String() != "3.90" || request.InvoiceTotalAmount.String() != "130.00" {
		t.Fatalf("unexpected wallet invoice: %+v", request)
	}
	if request.PaymentFeeRate.String() != "0.00" || request.PaymentFeeAmount.String() != "0.00" || request.PaymentChannelID != 0 {
		t.Fatalf("wallet invoice must not use a payment channel fee: %+v", request)
	}
	if store.walletCalls != 1 || store.walletUserID != 7 || store.walletResellerID == nil || *store.walletResellerID != resellerID {
		t.Fatalf("wallet payment scope mismatch: calls=%d user=%d reseller=%v", store.walletCalls, store.walletUserID, store.walletResellerID)
	}
	if gateway.input.OrderNo != "" {
		t.Fatalf("wallet payment unexpectedly called gateway: %+v", gateway.input)
	}
}

func TestCreateManualInvoiceNeedsNoPlatformOrder(t *testing.T) {
	store := &requestStoreStub{}
	service := NewService(store, channelStoreStub{}, registryStub{gateway: &gatewayStub{}}, "https://lsrai.shop")
	request, err := service.Create(context.Background(), CreateInput{
		Source: "manual", SourceHost: "lsrai.shop",
		InvoiceAmount: money.FromDecimal(decimal.RequireFromString("130.00")), InvoiceType: domain.TypeOrdinary,
		BuyerTitle: "示例公司", TaxNumber: "91350000TEST", RecipientEmail: "finance@example.com", ClientIP: "127.0.0.1",
		PaymentMethod: domain.PaymentMethodWallet, UserID: 7,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if request.Source != "manual" || request.OriginalOrderID != nil || !strings.HasPrefix(request.OriginalOrderNo, "OFFLINE-") {
		t.Fatalf("manual invoice audit reference mismatch: %+v", request)
	}
	if request.OriginalAmount.String() != "130.00" || request.InvoiceTotalAmount.String() != "130.00" || request.PaymentAmount.String() != "3.90" {
		t.Fatalf("manual invoice amounts mismatch: %+v", request)
	}
}

func TestCreateRejectsDuplicateOriginalOrder(t *testing.T) {
	store := &requestStoreStub{item: &domain.Request{RequestNo: "INV-OLD"}}
	service := NewService(store, channelStoreStub{}, registryStub{gateway: &gatewayStub{}}, "https://lsrai.shop")
	_, err := service.Create(context.Background(), CreateInput{
		Source: "dujiao", SourceHost: "lsrai.shop", OriginalOrderNo: "DJ-1",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(1)), InvoiceType: domain.TypeOrdinary,
		BuyerTitle: "示例公司", TaxNumber: "91350000TEST", RecipientEmail: "finance@example.com", ClientIP: "127.0.0.1",
	})
	if !errors.Is(err, ErrAlreadyRequested) {
		t.Fatalf("Create() error = %v, want ErrAlreadyRequested", err)
	}
}

func TestCreateRejectsNewSpecialInvoiceRequests(t *testing.T) {
	store := &requestStoreStub{}
	service := NewService(store, channelStoreStub{}, registryStub{gateway: &gatewayStub{}}, "https://lsrai.shop")
	_, err := service.Create(context.Background(), CreateInput{
		Source: "dujiao", SourceHost: "lsrai.shop", OriginalOrderNo: "DJ-SPECIAL",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(750)), InvoiceType: domain.TypeSpecial,
		BuyerTitle: "示例公司", TaxNumber: "91350000TEST", RecipientEmail: "finance@example.com", ClientIP: "127.0.0.1",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Create() error = %v, want ordinary-only rejection", err)
	}
}

type paidSinkStub struct{ calls int }

func (s *paidSinkStub) UpsertPaidRequest(context.Context, *domain.Request) (string, error) {
	s.calls++
	return "rec-1", nil
}

func TestPaymentCallbackMarksPaidAndSyncsFeishuOnce(t *testing.T) {
	request := &domain.Request{RequestNo: "INV-1", Status: domain.StatusPendingPayment, PaymentChannelID: 2, PaymentAmount: money.FromDecimal(decimal.RequireFromString("19.66"))}
	store := &requestStoreStub{item: request}
	gateway := &gatewayStub{callback: &paymentcontract.GatewayCallbackResult{OrderNo: "INV-1", ProviderRef: "trade-1", Status: "success", Amount: request.PaymentAmount, Currency: "CNY"}}
	service := NewService(store, channelStoreStub{item: &paymentdomain.PaymentChannel{ID: 2, ProviderType: "epay", ChannelType: "alipay"}}, registryStub{gateway: gateway}, "https://lsrai.shop")
	sink := &paidSinkStub{}
	service.SetPaidSink(sink)
	for index := 0; index < 2; index++ {
		paid, duplicate, err := service.HandlePaymentCallback(map[string][]string{"out_trade_no": {"INV-1"}}, nil)
		if err != nil || paid == nil {
			t.Fatalf("callback %d failed: %v", index, err)
		}
		if duplicate != (index == 1) {
			t.Fatalf("callback %d duplicate=%v", index, duplicate)
		}
	}
	if store.item.Status != domain.StatusPendingIssue || store.item.FeishuRecordID != "rec-1" || sink.calls != 1 {
		t.Fatalf("unexpected callback result: request=%+v sink_calls=%d", store.item, sink.calls)
	}
}
