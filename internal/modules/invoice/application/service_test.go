package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/invoice/domain"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type requestStoreStub struct{ item *domain.Request }

func (s *requestStoreStub) Create(item *domain.Request) error { s.item = item; return nil }
func (s *requestStoreStub) Save(item *domain.Request) error   { s.item = item; return nil }
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
	input paymentcontract.GatewayCreateInput
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

func TestCreateSnapshotsInvoiceAndFourPercentPaymentAmounts(t *testing.T) {
	store := &requestStoreStub{}
	gateway := &gatewayStub{}
	service := NewService(store, channelStoreStub{item: &paymentdomain.PaymentChannel{
		ID: 2, ProviderType: "epay", ChannelType: "alipay", InteractionMode: "qr", IsActive: true,
		FeeRate: money.FromDecimal(decimal.RequireFromString("4.00")),
	}}, registryStub{gateway: gateway}, "https://lsrai.shop")

	request, err := service.Create(context.Background(), CreateInput{
		Source: "dujiao", SourceHost: "vip.lsrai.shop", OriginalOrderNo: "DJ-1",
		OriginalAmount: money.FromDecimal(decimal.RequireFromString("630.00")), InvoiceType: domain.TypeOrdinary,
		BuyerTitle: "示例公司", TaxNumber: "91350000TEST", RecipientEmail: "finance@example.com", ClientIP: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if request.InvoiceFeeAmount.String() != "18.90" || request.InvoiceTotalAmount.String() != "648.90" {
		t.Fatalf("unexpected invoice amounts: %+v", request)
	}
	if request.PaymentFeeRate.String() != "4.00" || request.PaymentFeeAmount.String() != "0.76" || request.PaymentAmount.String() != "19.66" {
		t.Fatalf("unexpected payment amounts: %+v", request)
	}
	if gateway.input.OrderNo != request.RequestNo || gateway.input.Amount.String() != "19.66" || gateway.input.NotifyURL != "https://lsrai.shop/api/v1/invoices/payment/callback" {
		t.Fatalf("unexpected gateway input: %+v", gateway.input)
	}
	if gateway.input.ReturnURL != "https://lsrai.shop/invoice?request_no="+request.RequestNo {
		t.Fatalf("unexpected return URL: %s", gateway.input.ReturnURL)
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
