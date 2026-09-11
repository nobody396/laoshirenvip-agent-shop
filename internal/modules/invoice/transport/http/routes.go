package invoicehttp

import "github.com/gin-gonic/gin"

func RegisterPublicRoutes(public *gin.RouterGroup, handler *Handler) {
	public.GET("/invoices/:request_no", handler.GetPublic)
}

func RegisterGuestRoutes(guest *gin.RouterGroup, handler *Handler) {
	guest.POST("/invoices/preview", handler.PreviewGuest)
	guest.POST("/invoices", handler.CreateGuest)
	guest.POST("/invoices/gmshop/preview", handler.PreviewGMShop)
	guest.POST("/invoices/gmshop", handler.CreateGMShop)
}

func RegisterUserRoutes(user *gin.RouterGroup, handler *Handler) {
	user.POST("/invoices/preview", handler.PreviewUser)
	user.POST("/invoices", handler.CreateUser)
	user.POST("/invoices/recharge/preview", handler.PreviewRecharge)
	user.POST("/invoices/recharge", handler.CreateRecharge)
}

func RegisterCallbackRoutes(api *gin.RouterGroup, handler *Handler) {
	api.GET("/invoices/payment/callback", handler.PaymentCallback)
	api.POST("/invoices/payment/callback", handler.PaymentCallback)
}
