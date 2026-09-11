package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

const (
	TypeOrdinary        = "ordinary"
	TypeSpecial         = "special"
	InvoiceItemName     = "生产生活服务信息系统服务"
	PaymentMethodAlipay = "alipay"
	PaymentMethodWallet = "wallet"

	OrdinaryRateBPS = 300

	StatusPendingPayment = "pending_payment"
	StatusPendingIssue   = "pending_issue"
	StatusPendingEmail   = "pending_email"
	StatusCompleted      = "completed"
	StatusEmailFailed    = "email_failed"
	StatusCancelled      = "cancelled"
)

var ErrInvalidInvoiceType = errors.New("invalid invoice type")

// Request is the single durable record for invoice intake, payment, Feishu
// handoff, and final PDF email delivery.
type Request struct {
	ID                 uint         `gorm:"primarykey" json:"id"`
	RequestNo          string       `gorm:"uniqueIndex;size:64;not null" json:"request_no"`
	Source             string       `gorm:"index;uniqueIndex:idx_invoice_original_order;size:32;not null" json:"source"`
	SourceHost         string       `gorm:"index;uniqueIndex:idx_invoice_original_order;size:255;not null" json:"source_host"`
	OriginalOrderID    *uint        `gorm:"index" json:"original_order_id,omitempty"`
	OriginalOrderNo    string       `gorm:"index;uniqueIndex:idx_invoice_original_order;size:100;not null" json:"original_order_no"`
	SupplementOrderID  *uint        `gorm:"uniqueIndex" json:"supplement_order_id,omitempty"`
	InvoiceType        string       `gorm:"index;size:16;not null" json:"invoice_type"`
	RateBPS            int          `gorm:"not null" json:"rate_bps"`
	OriginalAmount     money.Amount `gorm:"type:decimal(20,2);not null" json:"original_amount"`
	InvoiceFeeAmount   money.Amount `gorm:"type:decimal(20,2);not null" json:"invoice_fee_amount"`
	InvoiceTotalAmount money.Amount `gorm:"type:decimal(20,2);not null" json:"invoice_total_amount"`
	PaymentChannelID   uint         `gorm:"index;not null" json:"payment_channel_id"`
	PaymentFeeRate     money.Amount `gorm:"type:decimal(6,2);not null" json:"payment_fee_rate"`
	PaymentFeeAmount   money.Amount `gorm:"type:decimal(20,2);not null" json:"payment_fee_amount"`
	PaymentAmount      money.Amount `gorm:"type:decimal(20,2);not null" json:"payment_amount"`
	ProviderRef        string       `gorm:"index;size:100" json:"provider_ref,omitempty"`
	PayURL             string       `gorm:"type:text" json:"pay_url,omitempty"`
	QRCode             string       `gorm:"type:text" json:"qr_code,omitempty"`
	BuyerTitle         string       `gorm:"size:255;not null" json:"buyer_title"`
	TaxNumber          string       `gorm:"size:64;not null" json:"tax_number"`
	CompanyAddress     string       `gorm:"size:500" json:"company_address,omitempty"`
	CompanyPhone       string       `gorm:"size:64" json:"company_phone,omitempty"`
	BankName           string       `gorm:"size:255" json:"bank_name,omitempty"`
	BankAccount        string       `gorm:"size:100" json:"bank_account,omitempty"`
	RecipientEmail     string       `gorm:"index;size:320;not null" json:"recipient_email"`
	Status             string       `gorm:"index;size:32;not null" json:"status"`
	FeishuRecordID     string       `gorm:"size:100" json:"feishu_record_id,omitempty"`
	FeishuLastError    string       `gorm:"size:500" json:"feishu_last_error,omitempty"`
	InvoiceNumber      string       `gorm:"size:64" json:"invoice_number,omitempty"`
	InvoiceDate        *time.Time   `json:"invoice_date,omitempty"`
	InvoiceFileToken   string       `gorm:"size:255" json:"invoice_file_token,omitempty"`
	InvoiceFileName    string       `gorm:"size:255" json:"invoice_file_name,omitempty"`
	EmailSentAt        *time.Time   `gorm:"index" json:"email_sent_at,omitempty"`
	EmailLastError     string       `gorm:"size:500" json:"email_last_error,omitempty"`
	PaidAt             *time.Time   `gorm:"index" json:"paid_at,omitempty"`
	CreatedAt          time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt          time.Time    `gorm:"index" json:"updated_at"`
}

func (Request) TableName() string { return "invoice_requests" }

func RateBPS(invoiceType string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(invoiceType)) {
	case TypeOrdinary:
		return OrdinaryRateBPS, nil
	default:
		return 0, ErrInvalidInvoiceType
	}
}

// CalculateAmounts calculates the 3% service fee from the face amount entered
// by the applicant. The face amount itself is not increased by that fee.
func CalculateAmounts(invoiceAmount money.Amount, invoiceType string) (fee money.Amount, total money.Amount, rateBPS int, err error) {
	if invoiceAmount.Decimal.LessThanOrEqual(decimal.Zero) {
		return money.Amount{}, money.Amount{}, 0, errors.New("invoice amount must be positive")
	}
	rateBPS, err = RateBPS(invoiceType)
	if err != nil {
		return money.Amount{}, money.Amount{}, 0, err
	}
	feeValue := invoiceAmount.Decimal.Mul(decimal.NewFromInt(int64(rateBPS))).Div(decimal.NewFromInt(10_000)).Round(2)
	fee = money.FromDecimal(feeValue)
	total = money.FromDecimal(invoiceAmount.Decimal)
	return fee, total, rateBPS, nil
}

// CalculatePaymentAmount applies the selected channel's percentage snapshot to
// the invoice supplement. It intentionally matches Dujiao-Next's existing
// customer-surcharge rule: base + round(base * rate / 100).
func CalculatePaymentAmount(invoiceFee money.Amount, channelRate money.Amount) (paymentFee money.Amount, paymentTotal money.Amount, err error) {
	if invoiceFee.Decimal.LessThanOrEqual(decimal.Zero) || channelRate.Decimal.IsNegative() || channelRate.Decimal.GreaterThan(decimal.NewFromInt(100)) {
		return money.Amount{}, money.Amount{}, errors.New("invalid payment amount or channel rate")
	}
	feeValue := invoiceFee.Decimal.Mul(channelRate.Decimal).Div(decimal.NewFromInt(100)).Round(2)
	paymentFee = money.FromDecimal(feeValue)
	paymentTotal = money.FromDecimal(invoiceFee.Decimal.Add(feeValue))
	return paymentFee, paymentTotal, nil
}
