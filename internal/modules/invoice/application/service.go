package application

import (
	"context"
	"errors"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/invoice/domain"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/shared/serial"
)

const InvoicePaymentChannelID uint = 2

var (
	ErrInvalidInput       = errors.New("invalid invoice input")
	ErrAlreadyRequested   = errors.New("invoice already requested")
	ErrPaymentUnavailable = errors.New("invoice payment unavailable")
)

type Store interface {
	Create(*domain.Request) error
	Save(*domain.Request) error
	GetByRequestNo(string) (*domain.Request, error)
	GetByOriginalOrder(source, sourceHost, orderNo string) (*domain.Request, error)
	MarkPaid(requestNo, providerRef string, paidAt time.Time) (bool, *domain.Request, error)
	SetFeishuSync(requestNo, recordID, lastError string) error
}

func (s *Service) HandlePaymentCallback(form map[string][]string, body []byte) (*domain.Request, bool, error) {
	requestNo := strings.TrimSpace(firstFormValue(form, "out_trade_no"))
	if requestNo == "" {
		return nil, false, ErrInvalidInput
	}
	request, err := s.store.GetByRequestNo(requestNo)
	if err != nil || request == nil {
		return nil, false, ErrInvalidInput
	}
	channel, err := s.channels.GetByID(request.PaymentChannelID)
	if err != nil || channel == nil {
		return nil, false, ErrPaymentUnavailable
	}
	provider, ok := s.gateways.Lookup(channel.ProviderType, channel.ChannelType)
	if !ok {
		return nil, false, ErrPaymentUnavailable
	}
	verifier, ok := provider.(paymentcontract.GatewayCallbackVerifier)
	if !ok {
		return nil, false, ErrPaymentUnavailable
	}
	result, err := verifier.VerifyCallback(channel.ConfigJSON, form, body)
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(result.OrderNo) != request.RequestNo || result.Status != constants.PaymentStatusSuccess || !result.Amount.Decimal.Equal(request.PaymentAmount.Decimal) || strings.ToUpper(strings.TrimSpace(result.Currency)) != "CNY" {
		return nil, false, ErrPaymentUnavailable
	}
	paidAt := time.Now()
	if result.PaidAt != nil {
		paidAt = *result.PaidAt
	}
	changed, paidRequest, err := s.store.MarkPaid(request.RequestNo, strings.TrimSpace(result.ProviderRef), paidAt)
	if err != nil || paidRequest == nil {
		return paidRequest, !changed, err
	}
	if s.paidSink != nil && paidRequest.FeishuRecordID == "" {
		recordID, syncErr := s.paidSink.UpsertPaidRequest(context.Background(), paidRequest)
		if syncErr != nil {
			paidRequest.FeishuLastError = syncErr.Error()
			_ = s.store.SetFeishuSync(paidRequest.RequestNo, "", syncErr.Error())
		} else {
			paidRequest.FeishuRecordID = recordID
			paidRequest.FeishuLastError = ""
			_ = s.store.SetFeishuSync(paidRequest.RequestNo, recordID, "")
		}
	}
	return paidRequest, !changed, nil
}

func firstFormValue(form map[string][]string, key string) string {
	values := form[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

type ChannelStore interface {
	GetByID(uint) (*paymentdomain.PaymentChannel, error)
}

type Service struct {
	store    Store
	channels ChannelStore
	gateways paymentcontract.GatewayRegistry
	baseURL  string
	paidSink PaidSink
}

type PaidSink interface {
	UpsertPaidRequest(context.Context, *domain.Request) (string, error)
}

func NewService(store Store, channels ChannelStore, gateways paymentcontract.GatewayRegistry, baseURL string) *Service {
	if store == nil || channels == nil || gateways == nil {
		panic("invoice service: required dependency is nil")
	}
	return &Service{store: store, channels: channels, gateways: gateways, baseURL: strings.TrimRight(baseURL, "/")}
}

func (s *Service) SetPaidSink(sink PaidSink) { s.paidSink = sink }

func (s *Service) Get(requestNo string) (*domain.Request, error) {
	requestNo = strings.TrimSpace(requestNo)
	if requestNo == "" {
		return nil, ErrInvalidInput
	}
	return s.store.GetByRequestNo(requestNo)
}

type CreateInput struct {
	Source          string
	SourceHost      string
	OriginalOrderID *uint
	OriginalOrderNo string
	OriginalAmount  money.Amount
	InvoiceType     string
	BuyerTitle      string
	TaxNumber       string
	CompanyAddress  string
	CompanyPhone    string
	BankName        string
	BankAccount     string
	RecipientEmail  string
	ClientIP        string
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*domain.Request, error) {
	input.Source = strings.ToLower(strings.TrimSpace(input.Source))
	input.SourceHost = strings.ToLower(strings.TrimSpace(input.SourceHost))
	input.OriginalOrderNo = strings.TrimSpace(input.OriginalOrderNo)
	input.BuyerTitle = strings.TrimSpace(input.BuyerTitle)
	input.TaxNumber = strings.TrimSpace(input.TaxNumber)
	input.RecipientEmail = strings.ToLower(strings.TrimSpace(input.RecipientEmail))
	if (input.Source != "dujiao" && input.Source != "gmshop") || input.SourceHost == "" || input.OriginalOrderNo == "" || input.BuyerTitle == "" || input.TaxNumber == "" || input.ClientIP == "" {
		return nil, ErrInvalidInput
	}
	if address, err := mail.ParseAddress(input.RecipientEmail); err != nil || !strings.EqualFold(address.Address, input.RecipientEmail) {
		return nil, ErrInvalidInput
	}
	if existing, err := s.store.GetByOriginalOrder(input.Source, input.SourceHost, input.OriginalOrderNo); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrAlreadyRequested
	}

	invoiceFee, invoiceTotal, rateBPS, err := domain.CalculateAmounts(input.OriginalAmount, input.InvoiceType)
	if err != nil {
		return nil, ErrInvalidInput
	}
	channel, err := s.channels.GetByID(InvoicePaymentChannelID)
	if err != nil || channel == nil || !channel.IsActive || channel.ProviderType != "epay" || channel.ChannelType != "alipay" {
		return nil, ErrPaymentUnavailable
	}
	paymentFee, paymentAmount, err := domain.CalculatePaymentAmount(invoiceFee, channel.FeeRate)
	if err != nil {
		return nil, ErrPaymentUnavailable
	}

	now := time.Now()
	request := &domain.Request{
		RequestNo:          serial.Generate("INV"),
		Source:             input.Source,
		SourceHost:         input.SourceHost,
		OriginalOrderID:    input.OriginalOrderID,
		OriginalOrderNo:    input.OriginalOrderNo,
		InvoiceType:        strings.ToLower(strings.TrimSpace(input.InvoiceType)),
		RateBPS:            rateBPS,
		OriginalAmount:     input.OriginalAmount,
		InvoiceFeeAmount:   invoiceFee,
		InvoiceTotalAmount: invoiceTotal,
		PaymentChannelID:   channel.ID,
		PaymentFeeRate:     channel.FeeRate,
		PaymentFeeAmount:   paymentFee,
		PaymentAmount:      paymentAmount,
		BuyerTitle:         input.BuyerTitle,
		TaxNumber:          input.TaxNumber,
		CompanyAddress:     strings.TrimSpace(input.CompanyAddress),
		CompanyPhone:       strings.TrimSpace(input.CompanyPhone),
		BankName:           strings.TrimSpace(input.BankName),
		BankAccount:        strings.TrimSpace(input.BankAccount),
		RecipientEmail:     input.RecipientEmail,
		Status:             domain.StatusPendingPayment,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if request.InvoiceType == domain.TypeSpecial && (request.CompanyAddress == "" || request.CompanyPhone == "" || request.BankName == "" || request.BankAccount == "") {
		return nil, ErrInvalidInput
	}
	if err := s.store.Create(request); err != nil {
		return nil, err
	}

	provider, ok := s.gateways.Lookup(channel.ProviderType, channel.ChannelType)
	if !ok {
		return nil, ErrPaymentUnavailable
	}
	notifyURL, returnURL, err := s.paymentURLs(request.RequestNo)
	if err != nil {
		return nil, ErrPaymentUnavailable
	}
	result, err := provider.CreatePayment(ctx, channel.ConfigJSON, paymentcontract.GatewayCreateInput{
		OrderNo:     request.RequestNo,
		Subject:     "电子发票补款",
		Amount:      request.PaymentAmount,
		Currency:    "CNY",
		Email:       request.RecipientEmail,
		NotifyURL:   notifyURL,
		ReturnURL:   returnURL,
		ClientIP:    input.ClientIP,
		ChannelType: channel.ChannelType,
		Extra:       jsonmap.JSON{"interaction_mode": channel.InteractionMode},
	})
	if err != nil {
		return request, ErrPaymentUnavailable
	}
	request.ProviderRef = strings.TrimSpace(result.ProviderRef)
	request.PayURL = strings.TrimSpace(result.RedirectURL)
	request.QRCode = strings.TrimSpace(result.QRCodeURL)
	request.UpdatedAt = time.Now()
	if err := s.store.Save(request); err != nil {
		return nil, err
	}
	return request, nil
}

func (s *Service) paymentURLs(requestNo string) (string, string, error) {
	base, err := url.Parse(s.baseURL)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return "", "", ErrPaymentUnavailable
	}
	notify := *base
	notify.Path = "/api/v1/invoices/payment/callback"
	notify.RawQuery = ""
	ret := *base
	ret.Path = "/invoice/requests/" + url.PathEscape(requestNo)
	ret.RawQuery = ""
	return notify.String(), ret.String(), nil
}
