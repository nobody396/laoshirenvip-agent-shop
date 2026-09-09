package gormstore

import (
	"errors"
	"time"

	"github.com/dujiao-next/internal/modules/invoice/domain"
	"gorm.io/gorm"
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

func (s *Store) Save(request *domain.Request) error { return s.db.Save(request).Error }

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
