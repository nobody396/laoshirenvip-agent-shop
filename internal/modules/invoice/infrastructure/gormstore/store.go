package gormstore

import (
	"errors"

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
