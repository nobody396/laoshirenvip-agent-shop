package gormstore

import (
	"errors"
	"time"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	"gorm.io/gorm"
)

// ListProductSettingsForPricing 批量获取分销定价所需的商品级与 SKU 级配置。
func (r *Store) ListProductSettingsForPricing(resellerID uint, productIDs []uint, skuIDs []uint) ([]resellerdomain.ProductSetting, error) {
	if resellerID == 0 || len(productIDs) == 0 {
		return []resellerdomain.ProductSetting{}, nil
	}
	productIDs = uniqueUintSlice(productIDs)
	skuIDs = uniqueUintSlice(skuIDs)

	query := r.db.Where("reseller_id = ? AND product_id IN ? AND deleted_at IS NULL", resellerID, productIDs)
	if len(skuIDs) > 0 {
		query = query.Where("(sku_id = 0 OR sku_id IN ?)", skuIDs)
	} else {
		query = query.Where("sku_id = 0")
	}

	var rows []resellerdomain.ProductSetting
	if err := query.Order("product_id ASC, sku_id ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListHiddenProductIDs 返回分销前台列表应在查询层排除的商品 ID。
func (r *Store) ListHiddenProductIDs(resellerID uint) ([]uint, error) {
	if resellerID == 0 {
		return []uint{}, nil
	}

	hidden := map[uint]struct{}{}
	var productHidden []uint
	if err := r.db.Model(&resellerdomain.ProductSetting{}).
		Where("reseller_id = ? AND sku_id = 0 AND is_listed = ? AND deleted_at IS NULL", resellerID, false).
		Pluck("product_id", &productHidden).Error; err != nil {
		return nil, err
	}
	for _, id := range productHidden {
		if id != 0 {
			hidden[id] = struct{}{}
		}
	}

	var skuHidden []uint
	if err := r.db.Model(&productdomain.ProductSKU{}).
		Select("product_skus.product_id").
		Joins(
			"JOIN reseller_product_settings rps ON rps.product_id = product_skus.product_id AND rps.sku_id = product_skus.id AND rps.reseller_id = ? AND rps.is_listed = ? AND rps.deleted_at IS NULL",
			resellerID,
			false,
		).
		Where("product_skus.is_active = ? AND product_skus.deleted_at IS NULL", true).
		Group("product_skus.product_id").
		Having("COUNT(product_skus.id) = (SELECT COUNT(1) FROM product_skus ps2 WHERE ps2.product_id = product_skus.product_id AND ps2.is_active = ? AND ps2.deleted_at IS NULL)", true).
		Pluck("product_skus.product_id", &skuHidden).Error; err != nil {
		return nil, err
	}
	for _, id := range skuHidden {
		if id != 0 {
			hidden[id] = struct{}{}
		}
	}

	ids := make([]uint, 0, len(hidden))
	for id := range hidden {
		ids = append(ids, id)
	}
	return ids, nil
}

func uniqueUintSlice(values []uint) []uint {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (r *Store) ListCustomerPriceSettings(resellerID, customerUserID uint) ([]resellerdomain.CustomerPriceSetting, error) {
	if resellerID == 0 || customerUserID == 0 {
		return []resellerdomain.CustomerPriceSetting{}, nil
	}
	var rows []resellerdomain.CustomerPriceSetting
	err := r.db.Where("reseller_id = ? AND customer_user_id = ? AND deleted_at IS NULL", resellerID, customerUserID).
		Order("product_id ASC, sku_id ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *Store) ListCustomerPriceSettingsForPricing(resellerID, customerUserID uint, productIDs, skuIDs []uint) ([]resellerdomain.CustomerPriceSetting, error) {
	if resellerID == 0 || customerUserID == 0 || len(productIDs) == 0 || len(skuIDs) == 0 {
		return []resellerdomain.CustomerPriceSetting{}, nil
	}
	var rows []resellerdomain.CustomerPriceSetting
	err := r.db.Model(&resellerdomain.CustomerPriceSetting{}).
		Joins("JOIN users customer_price_user ON customer_price_user.id = reseller_customer_price_settings.customer_user_id AND customer_price_user.deleted_at IS NULL").
		Where("customer_price_user.registration_reseller_id = ? AND customer_price_user.status = ?", resellerID, constants.UserStatusActive).
		Where(
			"reseller_customer_price_settings.reseller_id = ? AND reseller_customer_price_settings.customer_user_id = ? AND reseller_customer_price_settings.product_id IN ? AND reseller_customer_price_settings.sku_id IN ? AND reseller_customer_price_settings.deleted_at IS NULL",
			resellerID, customerUserID, uniqueUintSlice(productIDs), uniqueUintSlice(skuIDs),
		).Order("reseller_customer_price_settings.product_id ASC, reseller_customer_price_settings.sku_id ASC, reseller_customer_price_settings.id ASC").Find(&rows).Error
	return rows, err
}

func (r *Store) UpsertCustomerPriceSetting(setting resellerdomain.CustomerPriceSetting) (*resellerdomain.CustomerPriceSetting, error) {
	if setting.ResellerID == 0 || setting.CustomerUserID == 0 || setting.ProductID == 0 || setting.SKUID == 0 {
		return nil, errors.New("reseller customer price scope is invalid")
	}
	var existing resellerdomain.CustomerPriceSetting
	err := r.db.Unscoped().Where(
		"reseller_id = ? AND customer_user_id = ? AND product_id = ? AND sku_id = ?",
		setting.ResellerID, setting.CustomerUserID, setting.ProductID, setting.SKUID,
	).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := r.db.Create(&setting).Error; err != nil {
			return nil, err
		}
		return &setting, nil
	}
	existing.FixedPriceAmount = setting.FixedPriceAmount
	existing.CreatedByUserID = setting.CreatedByUserID
	existing.DeletedAt = nil
	if err := r.db.Model(&resellerdomain.CustomerPriceSetting{}).Where("id = ?", existing.ID).Select("*").Updates(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (r *Store) DeleteCustomerPriceSetting(resellerID, customerUserID, productID, skuID uint) error {
	if resellerID == 0 || customerUserID == 0 || productID == 0 || skuID == 0 {
		return nil
	}
	now := time.Now()
	return r.db.Model(&resellerdomain.CustomerPriceSetting{}).
		Where("reseller_id = ? AND customer_user_id = ? AND product_id = ? AND sku_id = ? AND deleted_at IS NULL", resellerID, customerUserID, productID, skuID).
		Updates(map[string]interface{}{"deleted_at": &now, "updated_at": now}).Error
}
