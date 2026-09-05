package sharedstockhttp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type commodity struct {
	ID           uint   `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Cover        string `json:"cover"`
	Price        string `json:"price"`
	UserPrice    string `json:"user_price"`
	FactoryPrice string `json:"factory_price"`
	Stock        int    `json:"stock"`
	Config       string `json:"config"`
	DeliveryWay  int    `json:"delivery_way"`
	DraftStatus  int    `json:"draft_status"`
	Status       int    `json:"status"`
	APIStatus    int    `json:"api_status"`
	ContactType  int    `json:"contact_type"`
	PasswordMode int    `json:"password_status"`
	Minimum      int    `json:"minimum"`
	Maximum      int    `json:"maximum"`
}

type sharedCategory struct {
	ID       uint        `json:"id"`
	Name     string      `json:"name"`
	Children []commodity `json:"children"`
}

func (h *Handler) Connect(c *gin.Context) {
	id := userID(c)
	if id == 0 {
		failure(c, "商户ID不存在")
		return
	}
	name := "老实人 AI 伙伴"
	if settings, err := h.Settings.GetByKey(constants.SettingKeySiteConfig); err == nil {
		if brand, ok := settings["brand"].(map[string]any); ok {
			if value := strings.TrimSpace(fmt.Sprint(brand["site_name"])); value != "" {
				name = value
			}
		}
	}
	balance := "0.00"
	if account, err := h.Wallet.GetAccount(id); err == nil && account != nil {
		balance = account.Balance.StringFixed(2)
	}
	success(c, gin.H{"shopName": name, "balance": balance})
}

func (h *Handler) Items(c *gin.Context) {
	products, err := h.listProducts(c)
	if err != nil {
		failure(c, "读取商品失败")
		return
	}
	categories, err := h.Categories.List()
	if err != nil {
		failure(c, "读取分类失败")
		return
	}
	byCategory := make(map[uint][]commodity)
	for _, product := range products {
		byCategory[product.CategoryID] = append(byCategory[product.CategoryID], h.commodity(c, product))
	}
	result := make([]sharedCategory, 0, len(categories))
	for _, category := range categories {
		children := byCategory[category.ID]
		if len(children) == 0 {
			continue
		}
		result = append(result, sharedCategory{ID: category.ID, Name: localizedText(category.NameJSON), Children: children})
	}
	success(c, result)
}

func (h *Handler) Item(c *gin.Context) {
	product, err := h.productByCode(c.PostForm("code"))
	if err != nil || product == nil || !product.IsActive {
		failure(c, "商品不存在")
		return
	}
	success(c, h.commodity(c, *product))
}

// Legacy SharedStock's item endpoint returns the same category tree as items,
// narrowed to one product. Older ACG installations index data[0].children[0].
func (h *Handler) LegacyItem(c *gin.Context) {
	product, err := h.productByCode(c.PostForm("code"))
	if err != nil || product == nil || !product.IsActive {
		failure(c, "商品不存在")
		return
	}
	categoryName := "商品"
	if categories, err := h.Categories.List(); err == nil {
		for _, category := range categories {
			if category.ID == product.CategoryID {
				categoryName = localizedText(category.NameJSON)
				break
			}
		}
	}
	success(c, []sharedCategory{{
		ID: product.CategoryID, Name: categoryName, Children: []commodity{h.commodity(c, *product)},
	}})
}

func (h *Handler) Inventory(c *gin.Context) {
	code := c.PostForm("sharedCode")
	if code == "" {
		code = c.PostForm("shared_code")
	}
	product, err := h.productByCode(code)
	if err != nil || product == nil || !product.IsActive {
		failure(c, "商品不存在")
		return
	}
	sku, err := h.selectSKU(*product, c.PostForm("race"))
	if err != nil {
		failure(c, "商品规格不存在")
		return
	}
	price := h.skuPrice(c, product.ID, *sku)
	success(c, gin.H{
		"count": h.stock(*product, *sku), "delivery_way": 0, "draft_status": 0,
		"price": price, "user_price": price, "factory_price": price,
		"config": h.categoryConfig(c, *product), "is_category": len(product.SKUs) > 1,
	})
}

func (h *Handler) InventoryState(c *gin.Context) {
	code := c.PostForm("shared_code")
	product, err := h.productByCode(code)
	if err != nil || product == nil || !product.IsActive {
		failure(c, "商品不存在")
		return
	}
	sku, err := h.selectSKU(*product, c.PostForm("race"))
	if err != nil {
		failure(c, "商品规格不存在")
		return
	}
	quantity, _ := strconv.Atoi(c.PostForm("num"))
	if quantity < 1 || h.stock(*product, *sku) < quantity {
		failure(c, "库存不足")
		return
	}
	success(c, []any{})
}

func (h *Handler) Stock(c *gin.Context) {
	product, err := h.productByCode(c.PostForm("code"))
	if err != nil || product == nil || !product.IsActive {
		failure(c, "商品不存在")
		return
	}
	sku, err := h.selectSKU(*product, c.PostForm("race"))
	if err != nil {
		failure(c, "商品规格不存在")
		return
	}
	success(c, gin.H{"stock": h.stock(*product, *sku)})
}

func (h *Handler) Valuation(c *gin.Context) {
	product, err := h.productByCode(c.PostForm("code"))
	if err != nil || product == nil || !product.IsActive {
		failure(c, "商品不存在")
		return
	}
	sku, err := h.selectSKU(*product, c.PostForm("race"))
	if err != nil {
		failure(c, "商品规格不存在")
		return
	}
	quantity, _ := strconv.Atoi(c.PostForm("num"))
	if quantity < 1 {
		quantity = 1
	}
	amount, err := decimal.NewFromString(h.skuPrice(c, product.ID, *sku))
	if err != nil {
		failure(c, "商品价格错误")
		return
	}
	success(c, gin.H{"price": amount.Mul(decimal.NewFromInt(int64(quantity))).StringFixed(2), "currency_code": "CNY"})
}

func (h *Handler) listProducts(c *gin.Context) ([]productdomain.Product, error) {
	products, _, err := h.Products.ListForUpstreamSync(nil, false, 1, 1000)
	if err != nil {
		return nil, err
	}
	_ = h.Products.ApplyAutoStockCounts(products)
	return products, nil
}

func (h *Handler) productByCode(code string) (*productdomain.Product, error) {
	id, err := parseSharedCode(code)
	if err != nil {
		return nil, err
	}
	product, err := h.Products.GetAdminByID(strconv.FormatUint(uint64(id), 10))
	if err != nil || product == nil {
		return product, err
	}
	products := []productdomain.Product{*product}
	if err := h.Products.ApplyAutoStockCounts(products); err != nil {
		return nil, err
	}
	return &products[0], nil
}

func (h *Handler) commodity(c *gin.Context, product productdomain.Product) commodity {
	price := product.PriceAmount.StringFixed(2)
	stock := 0
	if len(product.SKUs) > 0 {
		price = ""
		for _, sku := range product.SKUs {
			if !sku.IsActive {
				continue
			}
			value := h.skuPrice(c, product.ID, sku)
			if price == "" || lessPrice(value, price) {
				price = value
			}
			stock += h.stock(product, sku)
		}
	}
	if price == "" {
		price = product.PriceAmount.StringFixed(2)
	}
	cover := ""
	if len(product.Images) > 0 {
		cover = product.Images[0]
	}
	return commodity{
		ID: product.ID, Code: sharedCode(product.ID), Name: localizedText(product.TitleJSON),
		Description: localizedText(product.DescriptionJSON), Cover: cover,
		Price: price, UserPrice: price, FactoryPrice: price, Stock: stock,
		Config: h.categoryConfig(c, product), DeliveryWay: 0, DraftStatus: 0,
		Status: 1, APIStatus: 1,
	}
}

func (h *Handler) categoryConfig(c *gin.Context, product productdomain.Product) string {
	if len(product.SKUs) <= 1 {
		return ""
	}
	rows := make([]string, 0, len(product.SKUs))
	for _, sku := range product.SKUs {
		if sku.IsActive {
			rows = append(rows, fmt.Sprintf("%s=%s", skuRace(sku), h.skuPrice(c, product.ID, sku)))
		}
	}
	sort.Strings(rows)
	return "[category]\n" + strings.Join(rows, "\n") + "\n[category_factory]\n" + strings.Join(rows, "\n")
}

func (h *Handler) selectSKU(product productdomain.Product, race string) (*productdomain.ProductSKU, error) {
	race = strings.TrimSpace(race)
	for index := range product.SKUs {
		sku := &product.SKUs[index]
		if !sku.IsActive {
			continue
		}
		if race == "" || race == skuRace(*sku) || race == sku.SKUCode || race == strconv.FormatUint(uint64(sku.ID), 10) {
			return sku, nil
		}
	}
	return nil, fmt.Errorf("sku not found")
}

func skuRace(sku productdomain.ProductSKU) string {
	if value, ok := sku.SpecValuesJSON["race"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if strings.TrimSpace(sku.SKUCode) != "" {
		return strings.TrimSpace(sku.SKUCode)
	}
	return strconv.FormatUint(uint64(sku.ID), 10)
}

func (h *Handler) skuPrice(c *gin.Context, productID uint, sku productdomain.ProductSKU) string {
	price := sku.PriceAmount.Decimal
	if id := userID(c); id > 0 {
		if user, err := h.Users.GetByID(id); err == nil && user != nil && user.MemberLevelID > 0 {
			if memberPrice, _ := h.MemberLevels.ResolveMemberPrice(user.MemberLevelID, productID, sku.ID, price); memberPrice.GreaterThan(decimal.Zero) {
				price = memberPrice
			}
		}
	}
	return price.Round(2).StringFixed(2)
}

func (h *Handler) stock(product productdomain.Product, sku productdomain.ProductSKU) int {
	switch product.FulfillmentType {
	case constants.FulfillmentTypeUpstream:
		if mapping, err := h.SKUMappings.GetByLocalSKUID(sku.ID); err == nil && mapping != nil {
			return mapping.UpstreamStock
		}
	case constants.FulfillmentTypeManual:
		return sku.ManualStockTotal
	default:
		return int(sku.AutoStockAvailable)
	}
	return 0
}

func lessPrice(left, right string) bool {
	a, errA := decimal.NewFromString(left)
	b, errB := decimal.NewFromString(right)
	return errA == nil && errB == nil && a.LessThan(b)
}
