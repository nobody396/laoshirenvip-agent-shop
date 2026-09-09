package mappingreader

import (
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
)

type ProductSource interface {
	GetByID(id uint) (*mappingdomain.Mapping, error)
}

type SKUReaderSource interface {
	GetByLocalSKUID(skuID uint) (*mappingdomain.SKUMapping, error)
}

type SKUReader struct {
	skus     SKUReaderSource
	products ProductSource
}

var _ procurementcontract.SKUMappingReader = (*SKUReader)(nil)

func NewSKUs(skus SKUReaderSource, products ProductSource) *SKUReader {
	return &SKUReader{skus: skus, products: products}
}

func (r *SKUReader) FindConnectionID(skuID uint) (uint, bool, error) {
	sku, err := r.skus.GetByLocalSKUID(skuID)
	if err != nil || sku == nil {
		return 0, false, err
	}
	mapping, err := r.products.GetByID(sku.ProductMappingID)
	if err != nil || mapping == nil {
		return 0, false, err
	}
	return mapping.ConnectionID, true, nil
}

func (r *SKUReader) FindUpstreamSKUID(skuID uint) (uint, bool, error) {
	mapping, err := r.skus.GetByLocalSKUID(skuID)
	if err != nil || mapping == nil {
		return 0, false, err
	}
	return mapping.UpstreamSKUID, true, nil
}
