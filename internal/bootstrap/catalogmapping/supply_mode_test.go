package catalogmappingbootstrap

import (
	"errors"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	cardsecretgormstore "github.com/dujiao-next/internal/modules/cardsecret/infrastructure/gormstore"
	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestSupplyModeKeepsMappingWhileSwitchingInventoryAuthority(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:supply_mode?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
		&cardsecretdomain.Secret{},
	); err != nil {
		t.Fatal(err)
	}

	product := productdomain.Product{
		CategoryID:       1,
		Slug:             "mapped-product",
		TitleJSON:        jsonmap.JSON{"zh-CN": "Mapped product"},
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:     constants.ProductPurchaseGuest,
		StockDisplayMode: constants.ProductStockDisplayExact,
		FulfillmentType:  constants.FulfillmentTypeUpstream,
		IsMapped:         true,
		IsActive:         true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatal(err)
	}
	sku := productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     "SKU",
		PriceAmount: money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	mapping := mappingdomain.Mapping{
		ConnectionID: 1, LocalProductID: product.ID, UpstreamProductID: 2,
		UpstreamFulfillmentType: constants.FulfillmentTypeAuto,
		UpstreamStatus:          mappingdomain.UpstreamStatusActive, IsActive: true,
		LastSyncedAt: &now,
	}
	if err := db.Create(&mapping).Error; err != nil {
		t.Fatal(err)
	}
	skuMapping := mappingdomain.SKUMapping{
		ProductMappingID: mapping.ID, LocalSKUID: sku.ID, UpstreamSKUID: 3,
		UpstreamPrice: money.FromDecimal(decimal.NewFromInt(9)),
		UpstreamStock: 2, UpstreamIsActive: true,
	}
	if err := db.Create(&skuMapping).Error; err != nil {
		t.Fatal(err)
	}

	stock := cardsecretgormstore.New(db)
	service, err := New(Dependencies{
		Mappings:    mappinggormstore.NewMappingStore(db),
		SKUMappings: mappinggormstore.NewSKUMappingStore(db),
		Products:    productgormstore.NewProductStore(db),
		SKUs:        productgormstore.NewSKUStore(db),
		CardStock:   stock,
		Media:       noopLocalMediaRecorder{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetSupplyMode(mapping.ID, constants.FulfillmentTypeAuto); !errors.Is(err, mappingcontract.ErrLocalStockUnavailable) {
		t.Fatalf("without staged cards error = %v", err)
	}
	if err := stock.CreateBatch([]cardsecretdomain.Secret{{
		ProductID: product.ID, SKUID: sku.ID, Secret: "secret", Status: cardsecretdomain.StatusAvailable,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := service.SetSupplyMode(mapping.ID, constants.FulfillmentTypeAuto); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&product, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	if product.FulfillmentType != constants.FulfillmentTypeAuto || !product.IsMapped {
		t.Fatalf("local override lost mapping: %+v", product)
	}
	if resolved, err := service.ResolveFulfillmentType(sku.ID, 1, constants.FulfillmentTypeAuto); err != nil || resolved != constants.FulfillmentTypeAuto {
		t.Fatalf("owned stock should win: resolved=%q err=%v", resolved, err)
	}
	if err := db.Model(&cardsecretdomain.Secret{}).Where("product_id = ? AND sku_id = ?", product.ID, sku.ID).Update("status", cardsecretdomain.StatusUsed).Error; err != nil {
		t.Fatal(err)
	}
	if resolved, err := service.ResolveFulfillmentType(sku.ID, 1, constants.FulfillmentTypeAuto); err != nil || resolved != constants.FulfillmentTypeUpstream {
		t.Fatalf("empty owned stock should fall back upstream: resolved=%q err=%v", resolved, err)
	}
	if got, err := service.GetByID(mapping.ID); err != nil || got == nil {
		t.Fatalf("mapping removed after local switch: mapping=%+v err=%v", got, err)
	}
	if err := service.SetSupplyMode(mapping.ID, constants.FulfillmentTypeUpstream); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&product, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	if product.FulfillmentType != constants.FulfillmentTypeUpstream {
		t.Fatalf("fulfillment type = %q", product.FulfillmentType)
	}
}
