package invoicehttp

import "github.com/gin-gonic/gin"

func RegisterPublicRoutes(public *gin.RouterGroup, handler *Handler) {
	public.GET("/invoices/:request_no", handler.GetPublic)
}

func RegisterGuestRoutes(guest *gin.RouterGroup, handler *Handler) {
	guest.POST("/invoices", handler.CreateGuest)
}

func RegisterUserRoutes(user *gin.RouterGroup, handler *Handler) {
	user.POST("/invoices", handler.CreateUser)
}

func RegisterCallbackRoutes(api *gin.RouterGroup, handler *Handler) {
	api.POST("/invoices/payment/callback", handler.PaymentCallback)
}
