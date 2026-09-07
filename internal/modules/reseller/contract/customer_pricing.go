package contract

import (
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
)

// CustomerPriceSettingStore owns authenticated per-customer SKU prices.
type CustomerPriceSettingStore interface {
	WithinCustomerPriceSettingTransaction(func(CustomerPriceSettingStore) error) error
	GetProfileByUserID(userID uint) (*resellerdomain.Profile, error)
	GetProductWithSettings(resellerID, productID uint) (*ProductSettingProductRow, error)
	ListCustomerPriceSettings(resellerID, customerUserID uint) ([]resellerdomain.CustomerPriceSetting, error)
	ListCustomerPriceSettingsForPricing(resellerID, customerUserID uint, productIDs, skuIDs []uint) ([]resellerdomain.CustomerPriceSetting, error)
	UpsertCustomerPriceSetting(setting resellerdomain.CustomerPriceSetting) (*resellerdomain.CustomerPriceSetting, error)
	DeleteCustomerPriceSetting(resellerID, customerUserID, productID, skuID uint) error
}
