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
	createCalls      int
	reviseCalls      int
	walletUserID     uint
	walletResellerID *uint
}

func (s *requestStoreStub) Create(item *domain.Request) error {
	s.item = item
	s.createCalls++
	return nil
}
func (s *requestStoreStub) SavePayment(item *domain.Request) error { s.item = item; return nil }
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
func (s *requestStoreStub) RevisePending(previous string, item *domain.Request) error {
	if s.item == nil || s.item.RequestNo != previous || s.item.Status != domain.StatusPendingPayment || s.item.PaidAt != nil {
		return domain.ErrRequestNotRevisable
	}
	s.item = item
	s.reviseCalls++
	return nil
}
func (s *requestStoreStub) ReviseAndPayWithWallet(previous string, item *domain.Request, userID uint, resellerID *uint) error {
	if err := s.RevisePending(previous, item); err != nil {
		return err
	}
	s.createCalls--
	return s.CreateAndPayWithWallet(item, userID, resellerID)
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
	createErr   error
	createCalls int
	input       paymentcontract.GatewayCreateInput
	callback    *paymentcontract.GatewayCallbackResult
}

func (g *gatewayStub) VerifyCallback(jsonmap.JSON, map[string][]string, []byte) (*paymentcontract.GatewayCallbackResult, error) {
	return g.callback, nil
}

func (g *gatewayStub) Type() string                              { return "epay" }
func (g *gatewayStub) ValidateConfig(jsonmap.JSON, string) error { return nil }
func (g *gatewayStub) CreatePayment(_ context.Context, _ jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	g.input = input
	g.createCalls++
	if g.createErr != nil {
		return nil, g.createErr
	}
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
	store := &requestStoreStub{item: &domain.Request{RequestNo: "INV-OLD", Status: domain.StatusPendingIssue}}
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

func TestGMShopInvoiceReturnsToItsOwnWebsite(t *testing.T) {
	store := &requestStoreStub{}
	gateway := &gatewayStub{}
	service := NewService(store, channelStoreStub{item: &paymentdomain.PaymentChannel{
		ID: 2, ProviderType: "epay", ChannelType: "alipay", InteractionMode: "qr", IsActive: true,
		FeeRate: money.FromDecimal(decimal.RequireFromString("4.00")),
	}}, registryStub{gateway: gateway}, "https://lsrai.shop")

	request, err := service.Create(context.Background(), CreateInput{
		Source: "gmshop", SourceHost: "laoshirenvip.com", OriginalOrderNo: "DJ-1",
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
	if gateway.input.ReturnURL != "https://laoshirenvip.com/invoice?request_no="+request.RequestNo {
		t.Fatalf("unexpected return URL: %s", gateway.input.ReturnURL)
	}
}

func TestGMShopRetryResumesSameFrozenInvoiceWithoutDuplicate(t *testing.T) {
	store := &requestStoreStub{}
	gateway := &gatewayStub{createErr: errors.New("upstream unavailable")}
	channel := &paymentdomain.PaymentChannel{ID: 2, ProviderType: "epay", ChannelType: "alipay", InteractionMode: "qr", IsActive: true, FeeRate: money.FromDecimal(decimal.NewFromInt(4))}
	service := NewService(store, channelStoreStub{item: channel}, registryStub{gateway: gateway}, "https://lsrai.shop")
	input := CreateInput{Source: "gmshop", SourceHost: "laoshirenvip.com", OriginalOrderNo: "GM-RETRY", OriginalAmount: money.FromDecimal(decimal.NewFromInt(685)), InvoiceAmount: money.FromDecimal(decimal.NewFromInt(685)), BuyerTitle: "Example", TaxNumber: "TEST", RecipientEmail: "buyer@example.com", ClientIP: "127.0.0.1"}
	first, err := service.Create(context.Background(), input)
	if !errors.Is(err, ErrPaymentUnavailable) || first == nil {
		t.Fatalf("expected saved pending invoice and gateway failure: %v", err)
	}
	gateway.createErr = nil
	// Retrying must not re-price a stored payment after channel pricing changes.
	channel.FeeRate = money.FromDecimal(decimal.NewFromInt(20))
	retried, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if retried.RequestNo != first.RequestNo || store.createCalls != 1 || gateway.input.OrderNo != first.RequestNo || gateway.input.Amount.String() != "21.37" {
		t.Fatal("retry changed invoice/payment identity or amount")
	}
	calls := gateway.createCalls
	if _, err = service.Create(context.Background(), input); err != nil || gateway.createCalls != calls {
		t.Fatal("ready payment must be reused")
	}
	retried.Status = domain.StatusPendingIssue
	paidAt := time.Now()
	retried.PaidAt = &paidAt
	if _, err = service.Create(context.Background(), input); err != nil || gateway.createCalls != calls {
		t.Fatal("paid invoice must not create another payment")
	}
	input.RecipientEmail = "other@example.com"
	if _, err = service.Create(context.Background(), input); !errors.Is(err, ErrAlreadyRequested) {
		t.Fatal("paid invoice must not be revised")
	}
}

func unpaidRevisionService(t *testing.T) (*Service, *requestStoreStub, *gatewayStub, CreateInput, *domain.Request) {
	t.Helper()
	store := &requestStoreStub{}
	gateway := &gatewayStub{}
	channel := &paymentdomain.PaymentChannel{ID: 2, ProviderType: "epay", ChannelType: "alipay", InteractionMode: "qr", IsActive: true, FeeRate: money.FromDecimal(decimal.NewFromInt(4))}
	service := NewService(store, channelStoreStub{item: channel}, registryStub{gateway: gateway}, "https://lsrai.shop")
	input := CreateInput{Source: "gmshop", SourceHost: "laoshirenvip.com", OriginalOrderNo: "GM-EDIT", OriginalAmount: money.FromDecimal(decimal.NewFromInt(1800)), InvoiceAmount: money.FromDecimal(decimal.NewFromInt(1900)), BuyerTitle: "Example", TaxNumber: "TEST", RecipientEmail: "buyer@example.com", ClientIP: "127.0.0.1"}
	first, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	return service, store, gateway, input, first
}

func TestUnpaidInvoiceAmountRevisionReissuesPayment(t *testing.T) {
	service, store, gateway, input, first := unpaidRevisionService(t)
	firstNo, firstID := first.RequestNo, first.ID
	input.InvoiceAmount = money.FromDecimal(decimal.NewFromInt(1957))
	input.BuyerTitle = "Example Two"
	revised, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if store.createCalls != 1 || store.reviseCalls != 1 || revised.ID != firstID || revised.RequestNo == firstNo {
		t.Fatalf("revision must reuse the row under a new payment number: %+v", revised)
	}
	if revised.InvoiceTotalAmount.String() != "1957.00" || revised.BuyerTitle != "Example Two" || revised.PaymentAmount.String() != "61.06" {
		t.Fatalf("revision amounts mismatch: %+v", revised)
	}
	if gateway.createCalls != 2 || gateway.input.OrderNo != revised.RequestNo || gateway.input.Amount.String() != "61.06" {
		t.Fatalf("revised payment not issued: %+v", gateway.input)
	}
}

func TestUnpaidInvoiceTitleRevisionKeepsPaymentLink(t *testing.T) {
	service, store, gateway, input, first := unpaidRevisionService(t)
	firstNo, firstPay := first.RequestNo, first.QRCode
	input.TaxNumber = "TEST-2"
	input.RecipientEmail = "Other@Example.com"
	revised, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if store.reviseCalls != 1 || gateway.createCalls != 1 || revised.RequestNo != firstNo || revised.QRCode != firstPay {
		t.Fatalf("same payment amount must keep the issued link: %+v", revised)
	}
	if revised.TaxNumber != "TEST-2" || revised.RecipientEmail != "other@example.com" {
		t.Fatalf("applicant data not revised: %+v", revised)
	}
}

func TestUnpaidDujiaoInvoiceCanSwitchToWallet(t *testing.T) {
	store := &requestStoreStub{item: &domain.Request{RequestNo: "INV-OLD", Status: domain.StatusPendingPayment, PaymentChannelID: 2, PayURL: "https://pay.example/old"}}
	service := NewService(store, channelStoreStub{}, registryStub{gateway: &gatewayStub{}}, "https://lsrai.shop")
	request, err := service.Create(context.Background(), CreateInput{
		Source: "dujiao", SourceHost: "lsrai.shop", OriginalOrderNo: "DJ-1",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)), InvoiceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		BuyerTitle: "示例公司", TaxNumber: "91350000TEST", RecipientEmail: "finance@example.com", ClientIP: "127.0.0.1",
		PaymentMethod: domain.PaymentMethodWallet, UserID: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.reviseCalls != 1 || store.walletCalls != 1 || request.Status != domain.StatusPendingIssue || request.PayURL != "" || request.PaymentAmount.String() != "3.00" {
		t.Fatalf("unpaid request not paid from wallet: %+v", request)
	}
}
