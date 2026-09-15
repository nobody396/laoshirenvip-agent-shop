package application

import "testing"

func TestBuildOrderResultRejectsSaleDisabledProductAndSKU(t *testing.T) {
	for _, fixture := range []orderPurchaseQuantityLimitFixture{
		{dsnPrefix: "order_product_sale_disabled", categorySlug: "c-product-disabled", productSlug: "product-disabled", requestQuantity: 1, expectedErr: ErrProductNotAvailable, productSaleDisabled: true},
		{dsnPrefix: "order_sku_sale_disabled", categorySlug: "c-sku-disabled", productSlug: "sku-disabled", requestQuantity: 1, expectedErr: ErrProductSKUInvalid, skuSaleDisabled: true},
	} {
		assertBuildOrderResultRejectsPurchaseQuantity(t, fixture)
	}
}
