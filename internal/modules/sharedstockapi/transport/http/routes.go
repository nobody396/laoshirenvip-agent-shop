package sharedstockhttp

import "github.com/gin-gonic/gin"

func RegisterRoutes(engine *gin.Engine, handler *Handler, auth gin.HandlerFunc, rateLimit gin.HandlerFunc) {
	if engine == nil || handler == nil || auth == nil || rateLimit == nil {
		panic("sharedstock routes: required dependency is nil")
	}
	registerCommodity := func(group *gin.RouterGroup) {
		group.Use(rateLimit, auth)
		group.POST("/items", handler.Items)
		group.POST("/item", handler.Item)
		group.POST("/inventory", handler.Inventory)
		group.POST("/inventoryState", handler.InventoryState)
		group.POST("/stock", handler.Stock)
		group.POST("/valuation", handler.Valuation)
		group.POST("/trade", handler.Trade)
		group.POST("/query", handler.Query)
		group.POST("/query/:tradeNo", handler.Query)
	}
	authentication := engine.Group("/shared/authentication")
	authentication.Use(rateLimit, auth)
	authentication.POST("/connect", handler.Connect)
	registerCommodity(engine.Group("/shared/commodity"))

	legacy := engine.Group("/plugin/SharedStock/api")
	legacy.Use(rateLimit, auth)
	legacy.POST("/connect", handler.Connect)
	legacy.POST("/items", handler.Items)
	legacy.POST("/item", handler.LegacyItem)
	legacy.POST("/inventory", handler.Inventory)
	legacy.POST("/inventoryState", handler.InventoryState)
	legacy.POST("/stock", handler.Stock)
	legacy.POST("/valuation", handler.Valuation)
	legacy.POST("/trade", handler.Trade)
	legacy.POST("/query", handler.Query)
	legacy.POST("/query/:tradeNo", handler.Query)
}
