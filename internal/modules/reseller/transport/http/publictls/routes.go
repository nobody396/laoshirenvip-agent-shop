package publictlshttp

import "github.com/gin-gonic/gin"

func RegisterRoutes(public gin.IRoutes, handler *Handler) {
	if public == nil || handler == nil {
		panic("reseller public tls routes: required dependency is nil")
	}
	public.GET("/reseller-domain/tls-allow", handler.Allow)
}
