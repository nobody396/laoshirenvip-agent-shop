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

func TestGMShopManualRoutesAreSeparate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterGuestRoutes(router.Group("/api/v1/guest"), &Handler{})
	found := 0
	for _, r := range router.Routes() {
		if r.Method == "POST" && (r.Path == "/api/v1/guest/invoices/gmshop/manual" || r.Path == "/api/v1/guest/invoices/gmshop/manual/preview") {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("expected two dedicated GMShop manual routes, got %d", found)
	}
}
