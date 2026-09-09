package producthttp

import (
	"strings"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	domaincatalog "github.com/dujiao-next/internal/modules/catalog"
)

func (h *PublicHandler) decorateProductStock(product *productdomain.Product, item *publicProductView) {
	if product == nil || item == nil {
		return
	}

	stockStatus := constants.ProductStockStatusInStock
	manualAvailable := 0

	item.ManualStockAvailable = manualAvailable
	item.AutoStockTotal = 0
	item.AutoStockLocked = 0
	item.AutoStockSold = 0
	item.AutoStockAvailable = 0
	item.StockStatus = stockStatus
	item.IsSoldOut = false

	fulfillmentType := strings.TrimSpace(product.FulfillmentType)
	if fulfillmentType == "" {
		fulfillmentType = constants.FulfillmentTypeManual
	}

	// upstream 类型：根据 SKU 映射中的上游库存判断
	if fulfillmentType == constants.FulfillmentTypeUpstream {
		h.decorateUpstreamStock(product, item)
		return
	}

	if fulfillmentType == constants.FulfillmentTypeManual {
		hasActiveSKU := false
		hasUnlimitedSKU := false
		skuRemaining := 0
		for _, sku := range product.SKUs {
			if !sku.IsActive {
				continue
			}
			hasActiveSKU = true
			if sku.ManualStockTotal == constants.ManualStockUnlimited {
				hasUnlimitedSKU = true
				continue
			}
			if sku.ManualStockTotal > 0 {
				skuRemaining += sku.ManualStockTotal
			}
		}
		if hasActiveSKU {
			if hasUnlimitedSKU {
				item.ManualStockAvailable = constants.ManualStockUnlimited
				item.StockStatus = constants.ProductStockStatusUnlimited
				item.IsSoldOut = false
				return
			}
			manualAvailable = skuRemaining
		} else if product.ManualStockTotal == constants.ManualStockUnlimited {
			item.ManualStockAvailable = constants.ManualStockUnlimited
			item.StockStatus = constants.ProductStockStatusUnlimited
			item.IsSoldOut = false
			return
		} else {
			manualAvailable = product.ManualStockTotal
			if manualAvailable < 0 {
				manualAvailable = 0
			}
		}
		item.ManualStockAvailable = manualAvailable
		item.StockStatus = domaincatalog.StorefrontStockPolicy().Status(int64(manualAvailable))
		item.IsSoldOut = item.StockStatus == constants.ProductStockStatusOutOfStock
		return
	}

	autoAvailable := int64(0)
	autoTotal := int64(0)
	autoLocked := int64(0)
	autoSold := int64(0)
	for _, sku := range product.SKUs {
		if !sku.IsActive {
			continue
		}
		autoAvailable += sku.AutoStockAvailable
		autoTotal += sku.AutoStockTotal
		autoLocked += sku.AutoStockLocked
		autoSold += sku.AutoStockSold
	}
	item.AutoStockAvailable = autoAvailable
	item.AutoStockTotal = autoTotal
	item.AutoStockLocked = autoLocked
	item.AutoStockSold = autoSold
	if product.IsMapped {
		h.decorateLocalFirstStock(product, item)
		return
	}

	item.StockStatus = domaincatalog.StorefrontStockPolicy().Status(autoAvailable)
	item.IsSoldOut = item.StockStatus == constants.ProductStockStatusOutOfStock
}

// decorateLocalFirstStock advertises the larger available route while order
// creation still consumes local cards whenever they cover the requested
// quantity. A larger order can therefore fall back wholly to upstream stock.
func (h *PublicHandler) decorateLocalFirstStock(product *productdomain.Product, item *publicProductView) {
	mappings, err := h.mappings.ListByLocalProductIDs([]uint{product.ID})
	if err != nil || len(mappings) == 0 {
		item.StockStatus = domaincatalog.StorefrontStockPolicy().Status(item.AutoStockAvailable)
		item.IsSoldOut = item.StockStatus == constants.ProductStockStatusOutOfStock
		return
	}
	skuMappings := make([]mappingdomain.SKUMapping, 0)
	for _, mapping := range mappings {
		if !mapping.IsActive {
			continue
		}
		rows, loadErr := h.skuMappings.ListByProductMapping(mapping.ID)
		if loadErr != nil {
			continue
		}
		skuMappings = append(skuMappings, rows...)
	}
	byLocalSKU := make(map[uint]mappingdomain.SKUMapping, len(skuMappings))
	for _, row := range skuMappings {
		byLocalSKU[row.LocalSKUID] = row
	}
	var total int64
	unlimited := false
	for i := range item.Product.SKUs {
		sku := &item.Product.SKUs[i]
		if !sku.IsActive {
			continue
		}
		upstream, ok := byLocalSKU[sku.ID]
		if ok && upstream.UpstreamIsActive {
			sku.UpstreamStock = upstream.UpstreamStock
			if upstream.UpstreamStock < 0 {
				sku.AutoStockAvailable = -1
				unlimited = true
				continue
			}
			if int64(upstream.UpstreamStock) > sku.AutoStockAvailable {
				sku.AutoStockAvailable = int64(upstream.UpstreamStock)
			}
		}
		total += sku.AutoStockAvailable
	}
	if unlimited {
		item.AutoStockAvailable = -1
		item.StockStatus = constants.ProductStockStatusUnlimited
		item.IsSoldOut = false
		return
	}
	item.AutoStockAvailable = total
	item.StockStatus = domaincatalog.StorefrontStockPolicy().Status(total)
	item.IsSoldOut = item.StockStatus == constants.ProductStockStatusOutOfStock
}

// decorateUpstreamStock 根据 SKU 映射的上游库存信息填充商品及 SKU 级库存状态
func (h *PublicHandler) decorateUpstreamStock(product *productdomain.Product, item *publicProductView) {
	// A product may group SKU variants from several upstream connections.
	mappings, err := h.mappings.ListByLocalProductIDs([]uint{product.ID})
	if err != nil || len(mappings) == 0 {
		// 没有映射记录，降级为显示有库存（避免误售罄）
		item.Product.FulfillmentType = constants.FulfillmentTypeManual
		item.StockStatus = constants.ProductStockStatusInStock
		item.IsSoldOut = false
		return
	}

	// 根据上游原始交付类型设置展示类型：auto 还是 manual
	displayType := constants.FulfillmentTypeManual
	for _, mapping := range mappings {
		if mapping.IsActive && mapping.UpstreamFulfillmentType == constants.FulfillmentTypeAuto {
			displayType = constants.FulfillmentTypeAuto
			break
		}
	}
	item.Product.FulfillmentType = displayType

	// Load all SKU mappings across the product's active sources.
	skuMappings := make([]mappingdomain.SKUMapping, 0)
	for _, mapping := range mappings {
		if !mapping.IsActive {
			continue
		}
		rows, loadErr := h.skuMappings.ListByProductMapping(mapping.ID)
		if loadErr != nil {
			continue
		}
		skuMappings = append(skuMappings, rows...)
	}
	if len(skuMappings) == 0 {
		item.StockStatus = constants.ProductStockStatusInStock
		item.IsSoldOut = false
		return
	}

	// 按本地 SKU ID 索引映射
	skuMappingByLocal := make(map[uint]*mappingdomain.SKUMapping, len(skuMappings))
	for i := range skuMappings {
		skuMappingByLocal[skuMappings[i].LocalSKUID] = &skuMappings[i]
	}

	// 填充每个 SKU 的上游库存，同时汇总商品级状态
	hasUnlimited := false
	totalStock := 0
	hasActiveMapping := false

	for i := range item.Product.SKUs {
		sku := &item.Product.SKUs[i]
		sm, ok := skuMappingByLocal[sku.ID]
		if !ok || !sm.UpstreamIsActive {
			sku.UpstreamStock = 0
			continue
		}
		hasActiveMapping = true
		sku.UpstreamStock = sm.UpstreamStock

		// 根据展示类型填充对应的库存字段，让前端详情页的库存判断逻辑正确工作
		if displayType == constants.FulfillmentTypeAuto {
			if sm.UpstreamStock == -1 {
				sku.AutoStockAvailable = -1 // 前端对 auto 类型 -1 不做特殊处理，但总量为负时不限购
			} else {
				sku.AutoStockAvailable = int64(sm.UpstreamStock)
			}
		} else {
			if sm.UpstreamStock == -1 {
				sku.ManualStockTotal = constants.ManualStockUnlimited
			} else {
				sku.ManualStockTotal = sm.UpstreamStock
			}
		}

		if sm.UpstreamStock == -1 {
			hasUnlimited = true
		} else {
			totalStock += sm.UpstreamStock
		}
	}

	if !hasActiveMapping {
		item.StockStatus = constants.ProductStockStatusOutOfStock
		item.IsSoldOut = true
		return
	}

	if hasUnlimited {
		if displayType == constants.FulfillmentTypeAuto {
			item.AutoStockAvailable = -1
		} else {
			item.ManualStockAvailable = constants.ManualStockUnlimited
		}
		item.StockStatus = constants.ProductStockStatusUnlimited
		item.IsSoldOut = false
		return
	}

	if displayType == constants.FulfillmentTypeAuto {
		item.AutoStockAvailable = int64(totalStock)
	} else {
		item.ManualStockAvailable = totalStock
	}

	item.StockStatus = domaincatalog.StorefrontStockPolicy().Status(int64(totalStock))
	item.IsSoldOut = item.StockStatus == constants.ProductStockStatusOutOfStock
}
