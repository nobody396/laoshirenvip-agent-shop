package producthttp

import (
	"testing"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
)

type localFirstMappingReader struct{ mapping *mappingdomain.Mapping }

func (r localFirstMappingReader) GetByLocalProductID(uint) (*mappingdomain.Mapping, error) {
	return r.mapping, nil
}

type localFirstSKUMappingReader struct{ rows []mappingdomain.SKUMapping }

func (r localFirstSKUMappingReader) ListByProductMapping(uint) ([]mappingdomain.SKUMapping, error) {
	return r.rows, nil
}

func TestDecorateProductStock_AutoSkipsInactiveSKUs(t *testing.T) {
	h := &PublicHandler{}
	product := &productdomain.Product{
		ID:              1,
		FulfillmentType: constants.FulfillmentTypeAuto,
		SKUs: []productdomain.ProductSKU{
			{
				ID:                 11,
				SKUCode:            productdomain.DefaultSKUCode,
				IsActive:           true,
				AutoStockAvailable: 2,
				AutoStockTotal:     3,
				AutoStockLocked:    1,
				AutoStockSold:      4,
			},
			{
				ID:                 12,
				SKUCode:            "DISABLED",
				IsActive:           false,
				AutoStockAvailable: 100,
				AutoStockTotal:     120,
				AutoStockLocked:    20,
				AutoStockSold:      50,
			},
		},
	}

	item := publicProductView{Product: *product}
	h.decorateProductStock(product, &item)

	if item.AutoStockAvailable != 2 {
		t.Fatalf("expected auto_stock_available=2, got %d", item.AutoStockAvailable)
	}
	if item.AutoStockTotal != 3 {
		t.Fatalf("expected auto_stock_total=3, got %d", item.AutoStockTotal)
	}
	if item.AutoStockLocked != 1 {
		t.Fatalf("expected auto_stock_locked=1, got %d", item.AutoStockLocked)
	}
	if item.AutoStockSold != 4 {
		t.Fatalf("expected auto_stock_sold=4, got %d", item.AutoStockSold)
	}
	if item.IsSoldOut {
		t.Fatalf("expected product not sold out when active sku has stock")
	}
}

func TestDecorateProductStock_MappedAutoAdvertisesLargerFallbackRoute(t *testing.T) {
	h := &PublicHandler{
		mappings: localFirstMappingReader{mapping: &mappingdomain.Mapping{ID: 7}},
		skuMappings: localFirstSKUMappingReader{
			rows: []mappingdomain.SKUMapping{
				{
					ProductMappingID: 7,
					LocalSKUID:       11,
					UpstreamStock:    9,
					UpstreamIsActive: true,
				},
			},
		},
	}
	product := &productdomain.Product{
		ID: 1, IsMapped: true, FulfillmentType: constants.FulfillmentTypeAuto,
		SKUs: []productdomain.ProductSKU{
			{
				ID: 11, IsActive: true, AutoStockAvailable: 2, AutoStockTotal: 2,
			},
		},
	}
	item := publicProductView{Product: *product}
	h.decorateProductStock(product, &item)
	if item.AutoStockAvailable != 9 || item.Product.SKUs[0].AutoStockAvailable != 9 {
		t.Fatalf("expected upstream capacity 9 with local stock still preferred, got product=%d sku=%d", item.AutoStockAvailable, item.Product.SKUs[0].AutoStockAvailable)
	}
	if item.IsSoldOut {
		t.Fatal("mapped local-first product should remain purchasable")
	}
}

func TestPublicProductResponseStatusModeMasksExactStock(t *testing.T) {
	h := &PublicHandler{}
	product := &productdomain.Product{
		ID:               1,
		FulfillmentType:  constants.FulfillmentTypeManual,
		StockDisplayMode: constants.ProductStockDisplayStatus,
		SKUs: []productdomain.ProductSKU{
			{
				ID:               11,
				SKUCode:          productdomain.DefaultSKUCode,
				IsActive:         true,
				ManualStockTotal: 37,
			},
		},
	}

	resp, err := h.decoratePublicProduct(product, nil)
	if err != nil {
		t.Fatalf("decoratePublicProduct failed: %v", err)
	}

	if resp.StockDisplayMode != constants.ProductStockDisplayStatus {
		t.Fatalf("expected stock_display_mode=status, got %q", resp.StockDisplayMode)
	}
	if !resp.StockQuantityHidden {
		t.Fatalf("expected product stock quantity to be hidden")
	}
	if resp.ManualStockAvailable == 37 {
		t.Fatalf("expected product manual stock to be masked, got exact value %d", resp.ManualStockAvailable)
	}
	if resp.StockDisplay != constants.ProductStockStatusInStock {
		t.Fatalf("expected product stock display in_stock, got %q", resp.StockDisplay)
	}
	if len(resp.SKUs) != 1 {
		t.Fatalf("expected one sku, got %d", len(resp.SKUs))
	}
	sku := resp.SKUs[0]
	if !sku.StockQuantityHidden {
		t.Fatalf("expected sku stock quantity to be hidden")
	}
	if sku.ManualStockTotal == 37 {
		t.Fatalf("expected sku manual stock to be masked, got exact value %d", sku.ManualStockTotal)
	}
	if sku.StockStatus != constants.ProductStockStatusInStock {
		t.Fatalf("expected sku stock status in_stock, got %q", sku.StockStatus)
	}
	if sku.IsSoldOut {
		t.Fatalf("expected sku to remain purchasable")
	}
}

func TestPublicProductResponseRangeModeReturnsBucketOnly(t *testing.T) {
	h := &PublicHandler{}
	product := &productdomain.Product{
		ID:               1,
		FulfillmentType:  constants.FulfillmentTypeManual,
		StockDisplayMode: constants.ProductStockDisplayRange,
		SKUs: []productdomain.ProductSKU{
			{
				ID:               11,
				SKUCode:          productdomain.DefaultSKUCode,
				IsActive:         true,
				ManualStockTotal: 42,
			},
		},
	}

	resp, err := h.decoratePublicProduct(product, nil)
	if err != nil {
		t.Fatalf("decoratePublicProduct failed: %v", err)
	}

	if resp.StockDisplay != constants.ProductStockDisplayRange21To50 {
		t.Fatalf("expected product stock range 21-50, got %q", resp.StockDisplay)
	}
	if resp.StockRangeMin == nil || *resp.StockRangeMin != 21 {
		t.Fatalf("expected range min 21, got %+v", resp.StockRangeMin)
	}
	if resp.StockRangeMax == nil || *resp.StockRangeMax != 50 {
		t.Fatalf("expected range max 50, got %+v", resp.StockRangeMax)
	}
	if resp.ManualStockAvailable == 42 {
		t.Fatalf("expected product manual stock to be masked, got exact value %d", resp.ManualStockAvailable)
	}
	if len(resp.SKUs) != 1 {
		t.Fatalf("expected one sku, got %d", len(resp.SKUs))
	}
	sku := resp.SKUs[0]
	if sku.StockDisplay != constants.ProductStockDisplayRange21To50 {
		t.Fatalf("expected sku stock range 21-50, got %q", sku.StockDisplay)
	}
	if sku.StockRangeMin == nil || *sku.StockRangeMin != 21 {
		t.Fatalf("expected sku range min 21, got %+v", sku.StockRangeMin)
	}
	if sku.StockRangeMax == nil || *sku.StockRangeMax != 50 {
		t.Fatalf("expected sku range max 50, got %+v", sku.StockRangeMax)
	}
	if sku.ManualStockTotal == 42 {
		t.Fatalf("expected sku manual stock to be masked, got exact value %d", sku.ManualStockTotal)
	}
}
