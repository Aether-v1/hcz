package orderhttp

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUserOrderAndAfterSaleRoutesRegisterTogether(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	user := router.Group("/api/v1")
	RegisterUserReadRoutes(user, &UserHandler{})
	RegisterUserCancelRoute(user, &UserHandler{})
	RegisterUserAfterSaleRoutes(user, &AfterSaleHandler{})

	want := map[string]bool{
		"GET /api/v1/orders/:order_no":             false,
		"POST /api/v1/orders/:order_no/cancel":     false,
		"GET /api/v1/orders/:order_no/after-sale":  false,
		"POST /api/v1/orders/:order_no/after-sale": false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("missing route %s", route)
		}
	}
}
