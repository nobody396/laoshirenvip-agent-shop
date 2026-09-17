package gormstore

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/invoice/domain"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) *Store {
	if db == nil {
		panic("invoice store: db is nil")
	}
	return &Store{db: db}
}

func (s *Store) MarkPaid(requestNo, providerRef string, paidAt time.Time) (bool, *domain.Request, error) {
	result := s.db.Model(&domain.Request{}).
		Where("request_no = ? AND status = ?", requestNo, domain.StatusPendingPayment).
		Updates(map[string]interface{}{
			"status":       domain.StatusPendingIssue,
			"provider_ref": providerRef,
			"paid_at":      paidAt,
			"updated_at":   paidAt,
		})
	if result.Error != nil {
		return false, nil, result.Error
	}
	request, err := s.GetByRequestNo(requestNo)
	return result.RowsAffected == 1, request, err
}

func (s *Store) CreateAndPayWithWallet(request *domain.Request, userID uint, resellerID *uint) error {
	return s.payWithWallet(request, userID, resellerID, func(tx *gorm.DB) error { return tx.Create(request).Error })
}

// ReviseAndPayWithWallet rewrites an unpaid request and pays it from the wallet atomically.
func (s *Store) ReviseAndPayWithWallet(previousRequestNo string, request *domain.Request, userID uint, resellerID *uint) error {
	return s.payWithWallet(request, userID, resellerID, func(tx *gorm.DB) error { return reviseUnpaid(tx, previousRequestNo, request) })
}

func (s *Store) payWithWallet(request *domain.Request, userID uint, resellerID *uint, write func(*gorm.DB) error) error {
	if request == nil || userID == 0 || !request.PaymentAmount.Decimal.IsPositive() {
		return walletcontract.ErrInvalidAmount
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		reference := fmt.Sprintf("invoice:%s:wallet", strings.TrimSpace(request.RequestNo))
		request.Status = domain.StatusPendingIssue
		request.PaymentChannelID = 0
		request.ProviderRef = reference
		request.PayURL = ""
		request.QRCode = ""
		request.PaidAt = &now
		request.UpdatedAt = now
		if err := write(tx); err != nil {
			return err
		}

		amount := request.PaymentAmount.Decimal.Round(2)
		currency := constants.SiteCurrencyDefault
		if resellerID == nil || *resellerID == 0 {
			return debitMainWallet(tx, request, userID, amount, currency, reference, now)
		}
		return debitResellerWallet(tx, request, *resellerID, userID, amount, currency, reference, now)
	})
}

func debitMainWallet(tx *gorm.DB, request *domain.Request, userID uint, amount decimal.Decimal, currency, reference string, now time.Time) error {
	var account walletdomain.Account
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND deleted_at IS NULL", userID).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return walletcontract.ErrAccountNotFound
		}
		return err
	}
	before := account.Balance.Decimal.Round(2)
	if before.LessThan(amount) {
		return walletcontract.ErrInsufficientBalance
	}
	after := before.Sub(amount).Round(2)
	account.Balance = money.FromDecimal(after)
	account.UpdatedAt = now
	if err := tx.Save(&account).Error; err != nil {
		return walletcontract.ErrAccountUpdateFailed
	}
	entry := &walletdomain.Transaction{
		UserID: userID, OrderID: request.OriginalOrderID,
		Type: constants.WalletTxnTypeInvoicePay, Direction: constants.WalletTxnDirectionOut,
		Amount: money.FromDecimal(amount), BalanceBefore: money.FromDecimal(before), BalanceAfter: money.FromDecimal(after),
		Currency: currency, Reference: reference, Remark: "开票补款钱包支付", CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(entry).Error; err != nil {
		return walletcontract.ErrTransactionCreateFailed
	}
	return nil
}

func debitResellerWallet(tx *gorm.DB, request *domain.Request, resellerID, userID uint, amount decimal.Decimal, currency, reference string, now time.Time) error {
	var account walletdomain.ResellerAccount
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("reseller_id = ? AND user_id = ? AND deleted_at IS NULL", resellerID, userID).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return walletcontract.ErrAccountNotFound
		}
		return err
	}
	before := account.Balance.Decimal.Round(2)
	if before.LessThan(amount) {
		return walletcontract.ErrInsufficientBalance
	}
	after := before.Sub(amount).Round(2)
	account.Balance = money.FromDecimal(after)
	account.UpdatedAt = now
	if err := tx.Save(&account).Error; err != nil {
		return walletcontract.ErrAccountUpdateFailed
	}
	entry := &walletdomain.ResellerTransaction{
		ResellerID: resellerID, UserID: userID, OrderID: request.OriginalOrderID,
		Type: constants.WalletTxnTypeInvoicePay, Direction: constants.WalletTxnDirectionOut,
		Amount: money.FromDecimal(amount), BalanceBefore: money.FromDecimal(before), BalanceAfter: money.FromDecimal(after),
		Currency: currency, Reference: reference, Remark: "开票补款钱包支付", CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(entry).Error; err != nil {
		return walletcontract.ErrTransactionCreateFailed
	}
	return nil
}

func (s *Store) SetFeishuSync(requestNo, recordID, lastError string) error {
	return s.db.Model(&domain.Request{}).Where("request_no = ?", requestNo).Updates(map[string]interface{}{
		"feishu_record_id":  recordID,
		"feishu_last_error": lastError,
		"updated_at":        time.Now(),
	}).Error
}

func (s *Store) ListPendingFeishu(limit int) ([]domain.Request, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var requests []domain.Request
	err := s.db.Where("paid_at IS NOT NULL AND feishu_record_id = '' AND status IN ?", []string{domain.StatusPendingIssue, domain.StatusEmailFailed}).Order("paid_at ASC").Limit(limit).Find(&requests).Error
	return requests, err
}

func (s *Store) ClaimEmail(requestNo, invoiceNumber string, invoiceDate *time.Time) (bool, *domain.Request, error) {
	now := time.Now()
	result := s.db.Model(&domain.Request{}).
		Where("request_no = ? AND status IN ?", requestNo, []string{domain.StatusPendingIssue, domain.StatusEmailFailed}).
		Updates(map[string]interface{}{"status": domain.StatusPendingEmail, "invoice_number": invoiceNumber, "invoice_date": invoiceDate, "email_last_error": "", "updated_at": now})
	if result.Error != nil {
		return false, nil, result.Error
	}
	request, err := s.GetByRequestNo(requestNo)
	return result.RowsAffected == 1, request, err
}

func (s *Store) FinishEmail(requestNo string, success bool, lastError string, at time.Time) error {
	updates := map[string]interface{}{"updated_at": at, "email_last_error": lastError}
	if success {
		updates["status"] = domain.StatusCompleted
		updates["email_sent_at"] = at
	} else {
		updates["status"] = domain.StatusEmailFailed
	}
	return s.db.Model(&domain.Request{}).Where("request_no = ? AND status = ?", requestNo, domain.StatusPendingEmail).Updates(updates).Error
}

func (s *Store) Create(request *domain.Request) error { return s.db.Create(request).Error }

// RevisePending replaces an unpaid request's applicant data and pricing in place.
func (s *Store) RevisePending(previousRequestNo string, request *domain.Request) error {
	return reviseUnpaid(s.db, previousRequestNo, request)
}

func reviseUnpaid(tx *gorm.DB, previousRequestNo string, request *domain.Request) error {
	result := tx.Model(&domain.Request{}).
		Where("request_no = ? AND status = ? AND paid_at IS NULL", previousRequestNo, domain.StatusPendingPayment).
		Updates(map[string]interface{}{
			"request_no":           request.RequestNo,
			"original_order_id":    request.OriginalOrderID,
			"original_amount":      request.OriginalAmount,
			"rate_bps":             request.RateBPS,
			"invoice_fee_amount":   request.InvoiceFeeAmount,
			"invoice_total_amount": request.InvoiceTotalAmount,
			"payment_channel_id":   request.PaymentChannelID,
			"payment_fee_rate":     request.PaymentFeeRate,
			"payment_fee_amount":   request.PaymentFeeAmount,
			"payment_amount":       request.PaymentAmount,
			"provider_ref":         request.ProviderRef,
			"pay_url":              request.PayURL,
			"qr_code":              request.QRCode,
			"buyer_title":          request.BuyerTitle,
			"tax_number":           request.TaxNumber,
			"recipient_email":      request.RecipientEmail,
			"status":               request.Status,
			"paid_at":              request.PaidAt,
			"updated_at":           request.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrRequestNotRevisable
	}
	return nil
}

// SavePayment never overwrites a callback that already marked the invoice paid.
func (s *Store) SavePayment(request *domain.Request) error {
	return s.db.Model(&domain.Request{}).Where("request_no = ? AND status = ? AND paid_at IS NULL", request.RequestNo, domain.StatusPendingPayment).Updates(map[string]interface{}{"provider_ref": request.ProviderRef, "pay_url": request.PayURL, "qr_code": request.QRCode, "updated_at": time.Now()}).Error
}

func (s *Store) GetByRequestNo(requestNo string) (*domain.Request, error) {
	var request domain.Request
	err := s.db.Where("request_no = ?", requestNo).First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &request, err
}

func (s *Store) GetByOriginalOrder(source, sourceHost, orderNo string) (*domain.Request, error) {
	var request domain.Request
	err := s.db.Where("source = ? AND source_host = ? AND original_order_no = ?", source, sourceHost, orderNo).First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &request, err
}
