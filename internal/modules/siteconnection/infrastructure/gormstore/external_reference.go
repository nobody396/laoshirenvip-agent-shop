package gormstore

import (
	"errors"
	"fmt"
	"strings"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
	return kind == siteconnectiondomain.ExternalReferenceKindProduct || kind == siteconnectiondomain.ExternalReferenceKindSKU
}
