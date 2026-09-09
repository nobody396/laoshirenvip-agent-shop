package invoicehttp

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterCallbackRoutesAcceptsEpayGetAndPost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCallbackRoutes(router.Group("/api/v1"), &Handler{})

	want := map[string]bool{"GET": false, "POST": false}
	for _, route := range router.Routes() {
		if route.Path == "/api/v1/invoices/payment/callback" {
			if _, ok := want[route.Method]; ok {
				want[route.Method] = true
			}
		}
	}
	for method, found := range want {
		if !found {
			t.Fatalf("%s invoice callback route is not registered", method)
		}
	}
}
