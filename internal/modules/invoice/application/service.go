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
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/shared/serial"
	"github.com/shopspring/decimal"
)

const InvoicePaymentChannelID uint = 2

var (
	ErrInvalidInput       = errors.New("invalid invoice input")
	ErrAlreadyRequested   = errors.New("invoice already requested")
	ErrPaymentUnavailable = errors.New("invoice payment unavailable")
	ErrWalletInsufficient = errors.New("invoice wallet balance insufficient")
)

type Store interface {
	Create(*domain.Request) error
	CreateAndPayWithWallet(*domain.Request, uint, *uint) error
	Save(*domain.Request) error
	GetByRequestNo(string) (*domain.Request, error)
	GetByOriginalOrder(source, sourceHost, orderNo string) (*domain.Request, error)
	MarkPaid(requestNo, providerRef string, paidAt time.Time) (bool, *domain.Request, error)
	SetFeishuSync(requestNo, recordID, lastError string) error
	ListPendingFeishu(limit int) ([]domain.Request, error)
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
	s.syncPaidRequest(paidRequest)
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

func (s *Service) SyncPendingFeishu(ctx context.Context) error {
	if s.paidSink == nil {
		return nil
	}
	requests, err := s.store.ListPendingFeishu(50)
	if err != nil {
		return err
	}
	for index := range requests {
		request := &requests[index]
		recordID, syncErr := s.paidSink.UpsertPaidRequest(ctx, request)
		if syncErr != nil {
			_ = s.store.SetFeishuSync(request.RequestNo, "", syncErr.Error())
			continue
		}
		if err := s.store.SetFeishuSync(request.RequestNo, recordID, ""); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Get(requestNo string) (*domain.Request, error) {
	requestNo = strings.TrimSpace(requestNo)
	if requestNo == "" {
		return nil, ErrInvalidInput
	}
	return s.store.GetByRequestNo(requestNo)
}

type CreateInput struct {
	Source           string
	SourceHost       string
	OriginalOrderID  *uint
	OriginalOrderNo  string
	OriginalAmount   money.Amount
	InvoiceAmount    money.Amount
	InvoiceType      string
	BuyerTitle       string
	TaxNumber        string
	RecipientEmail   string
	ClientIP         string
	PaymentMethod    string
	UserID           uint
	WalletResellerID *uint
}

type AmountPreview struct {
	InvoiceFeeAmount   money.Amount
	InvoiceTotalAmount money.Amount
	RateBPS            int
	PaymentFeeRate     money.Amount
	PaymentFeeAmount   money.Amount
	PaymentAmount      money.Amount
}

func (s *Service) PreviewAmounts(invoiceAmount money.Amount, paymentMethod string) (AmountPreview, error) {
	preview, _, err := s.calculateAmounts(invoiceAmount, paymentMethod)
	return preview, err
}

func (s *Service) calculateAmounts(invoiceAmount money.Amount, paymentMethod string) (AmountPreview, *paymentdomain.PaymentChannel, error) {
	paymentMethod = strings.ToLower(strings.TrimSpace(paymentMethod))
	if paymentMethod == "" {
		paymentMethod = domain.PaymentMethodAlipay
	}
	if paymentMethod != domain.PaymentMethodAlipay && paymentMethod != domain.PaymentMethodWallet {
		return AmountPreview{}, nil, ErrInvalidInput
	}

	invoiceFee, invoiceTotal, rateBPS, err := domain.CalculateAmounts(invoiceAmount, domain.TypeOrdinary)
	if err != nil {
		return AmountPreview{}, nil, ErrInvalidInput
	}

	feeRate := money.FromDecimal(decimal.Zero)
	var channel *paymentdomain.PaymentChannel
	if paymentMethod == domain.PaymentMethodAlipay {
		channel, err = s.channels.GetByID(InvoicePaymentChannelID)
		if err != nil || channel == nil || !channel.IsActive || channel.ProviderType != "epay" || channel.ChannelType != "alipay" {
			return AmountPreview{}, nil, ErrPaymentUnavailable
		}
		feeRate = channel.FeeRate
	}
	paymentFee, paymentAmount, err := domain.CalculatePaymentAmount(invoiceFee, feeRate)
	if err != nil {
		return AmountPreview{}, nil, ErrPaymentUnavailable
	}
	return AmountPreview{
		InvoiceFeeAmount: invoiceFee, InvoiceTotalAmount: invoiceTotal, RateBPS: rateBPS,
		PaymentFeeRate: feeRate, PaymentFeeAmount: paymentFee, PaymentAmount: paymentAmount,
	}, channel, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*domain.Request, error) {
	requestNo := serial.Generate("INV")
	input.Source = strings.ToLower(strings.TrimSpace(input.Source))
	input.SourceHost = strings.ToLower(strings.TrimSpace(input.SourceHost))
	input.OriginalOrderNo = strings.TrimSpace(input.OriginalOrderNo)
	input.BuyerTitle = strings.TrimSpace(input.BuyerTitle)
	input.TaxNumber = strings.TrimSpace(input.TaxNumber)
	input.RecipientEmail = strings.ToLower(strings.TrimSpace(input.RecipientEmail))
	input.InvoiceType = strings.ToLower(strings.TrimSpace(input.InvoiceType))
	if input.InvoiceType == "" {
		input.InvoiceType = domain.TypeOrdinary
	}
	input.PaymentMethod = strings.ToLower(strings.TrimSpace(input.PaymentMethod))
	if input.PaymentMethod == "" {
		input.PaymentMethod = domain.PaymentMethodAlipay
	}
	if input.Source == "manual" {
		input.OriginalOrderNo = "OFFLINE-" + requestNo
		input.OriginalAmount = input.InvoiceAmount
	}
	if (input.Source != "dujiao" && input.Source != "dujiao_recharge" && input.Source != "gmshop" && input.Source != "manual") || input.SourceHost == "" || input.OriginalOrderNo == "" || input.BuyerTitle == "" || input.TaxNumber == "" || input.ClientIP == "" {
		return nil, ErrInvalidInput
	}
	if input.PaymentMethod != domain.PaymentMethodAlipay && input.PaymentMethod != domain.PaymentMethodWallet {
		return nil, ErrInvalidInput
	}
	if input.InvoiceType != domain.TypeOrdinary {
		return nil, ErrInvalidInput
	}
	if input.PaymentMethod == domain.PaymentMethodWallet && (input.UserID == 0 || input.Source == "gmshop") {
		return nil, ErrInvalidInput
	}
	if address, err := mail.ParseAddress(input.RecipientEmail); err != nil || !strings.EqualFold(address.Address, input.RecipientEmail) {
		return nil, ErrInvalidInput
	}
	if input.Source != "manual" {
		if existing, err := s.store.GetByOriginalOrder(input.Source, input.SourceHost, input.OriginalOrderNo); err != nil {
			return nil, err
		} else if existing != nil {
			return nil, ErrAlreadyRequested
		}
	}

	amounts, channel, err := s.calculateAmounts(input.InvoiceAmount, input.PaymentMethod)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	channelID := uint(0)
	if channel != nil {
		channelID = channel.ID
	}
	request := &domain.Request{
		RequestNo:          requestNo,
		Source:             input.Source,
		SourceHost:         input.SourceHost,
		OriginalOrderID:    input.OriginalOrderID,
		OriginalOrderNo:    input.OriginalOrderNo,
		InvoiceType:        domain.TypeOrdinary,
		RateBPS:            amounts.RateBPS,
		OriginalAmount:     input.OriginalAmount,
		InvoiceFeeAmount:   amounts.InvoiceFeeAmount,
		InvoiceTotalAmount: amounts.InvoiceTotalAmount,
		PaymentChannelID:   channelID,
		PaymentFeeRate:     amounts.PaymentFeeRate,
		PaymentFeeAmount:   amounts.PaymentFeeAmount,
		PaymentAmount:      amounts.PaymentAmount,
		BuyerTitle:         input.BuyerTitle,
		TaxNumber:          input.TaxNumber,
		RecipientEmail:     input.RecipientEmail,
		Status:             domain.StatusPendingPayment,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if input.PaymentMethod == domain.PaymentMethodWallet {
		if err := s.store.CreateAndPayWithWallet(request, input.UserID, input.WalletResellerID); err != nil {
			if errors.Is(err, walletcontract.ErrInsufficientBalance) || errors.Is(err, walletcontract.ErrAccountNotFound) {
				return nil, ErrWalletInsufficient
			}
			return nil, err
		}
		s.syncPaidRequest(request)
		return request, nil
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

func (s *Service) syncPaidRequest(request *domain.Request) {
	if s == nil || s.paidSink == nil || request == nil || request.FeishuRecordID != "" {
		return
	}
	recordID, err := s.paidSink.UpsertPaidRequest(context.Background(), request)
	if err != nil {
		request.FeishuLastError = err.Error()
		_ = s.store.SetFeishuSync(request.RequestNo, "", err.Error())
		return
	}
	request.FeishuRecordID = recordID
	request.FeishuLastError = ""
	_ = s.store.SetFeishuSync(request.RequestNo, recordID, "")
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
	ret.Path = "/invoice"
	ret.RawQuery = url.Values{"request_no": []string{requestNo}}.Encode()
	return notify.String(), ret.String(), nil
}
