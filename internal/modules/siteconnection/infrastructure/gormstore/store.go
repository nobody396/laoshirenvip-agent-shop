package gormstore

import (
	"errors"
	"fmt"
	"strings"
	"time"

	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 是 SiteConnection 的 GORM 持久化实现。
type Store struct {
	db *gorm.DB
}

var _ siteconnectioncontract.Repository = (*Store)(nil)

// New 创建连接仓库。
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// GetByID 根据 ID 获取
func (r *Store) GetByID(id uint) (*siteconnectiondomain.Connection, error) {
	var conn siteconnectiondomain.Connection
	if err := r.db.Where("deleted_at IS NULL").First(&conn, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conn, nil
}

// GetByApiKey 根据 ApiKey 获取连接
func (r *Store) GetByApiKey(apiKey string) (*siteconnectiondomain.Connection, error) {
	var conn siteconnectiondomain.Connection
	if err := r.db.Where("deleted_at IS NULL AND api_key = ?", apiKey).First(&conn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conn, nil
}

// Create 创建连接
func (r *Store) Create(conn *siteconnectiondomain.Connection) error {
	return r.db.Create(conn).Error
}

// Update 更新连接
func (r *Store) Update(conn *siteconnectiondomain.Connection) error {
	return r.db.Save(conn).Error
}

// Delete 软删除连接
func (r *Store) Delete(id uint) error {
	return r.db.Model(&siteconnectiondomain.Connection{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

// List 列表查询
func (r *Store) List(filter siteconnectioncontract.ListFilter) ([]siteconnectiondomain.Connection, int64, error) {
	var conns []siteconnectiondomain.Connection
	var total int64

	q := r.db.Model(&siteconnectiondomain.Connection{}).Where("deleted_at IS NULL")
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	q = q.Order("created_at DESC")
	if filter.Page > 0 && filter.PageSize > 0 {
		q = q.Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize)
	}

	if err := q.Find(&conns).Error; err != nil {
		return nil, 0, err
	}

	return conns, total, nil
}

// ListActive 获取所有启用的连接
func (r *Store) ListActive() ([]siteconnectiondomain.Connection, error) {
	var conns []siteconnectiondomain.Connection
	if err := r.db.Where("deleted_at IS NULL AND status = ?", "active").Find(&conns).Error; err != nil {
		return nil, err
	}
	return conns, nil
}

type ExternalReferenceStore struct {
	db *gorm.DB
}

func NewExternalReferenceStore(db *gorm.DB) *ExternalReferenceStore {
	return &ExternalReferenceStore{db: db}
}

func (s *ExternalReferenceStore) Resolve(connectionID uint, kind, externalKey string) (uint, error) {
	kind = strings.TrimSpace(kind)
	externalKey = strings.TrimSpace(externalKey)
	if connectionID == 0 || !validExternalReferenceKind(kind) || externalKey == "" || len(externalKey) > 1024 {
		return 0, fmt.Errorf("invalid external reference")
	}
	reference := siteconnectiondomain.ExternalReference{
		ConnectionID: connectionID,
		Kind:         kind,
		ExternalKey:  externalKey,
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "connection_id"}, {Name: "kind"}, {Name: "external_key"}},
		DoNothing: true,
	}).Create(&reference).Error; err != nil {
		return 0, err
	}
	if reference.ID != 0 {
		return reference.ID, nil
	}
	if err := s.db.Where("connection_id = ? AND kind = ? AND external_key = ?", connectionID, kind, externalKey).
		First(&reference).Error; err != nil {
		return 0, err
	}
	return reference.ID, nil
}

func (s *ExternalReferenceStore) Lookup(connectionID uint, kind string, id uint) (string, error) {
	if connectionID == 0 || id == 0 || !validExternalReferenceKind(kind) {
		return "", fmt.Errorf("invalid external reference lookup")
	}
	var reference siteconnectiondomain.ExternalReference
	if err := s.db.Where("id = ? AND connection_id = ? AND kind = ?", id, connectionID, kind).
		First(&reference).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return reference.ExternalKey, nil
}

func validExternalReferenceKind(kind string) bool {
	return kind == siteconnectiondomain.ExternalReferenceKindProduct ||
		kind == siteconnectiondomain.ExternalReferenceKindSKU ||
		kind == siteconnectiondomain.ExternalReferenceKindCategory ||
		kind == siteconnectiondomain.ExternalReferenceKindOrder
}
