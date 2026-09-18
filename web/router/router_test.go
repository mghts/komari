package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPluginRoutesRemovedAndOtherFeaturesRetained(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)
	routes := map[string]bool{}
	for _, route := range r.Routes() {
		if strings.Contains(route.Path, "/plugin") {
			t.Fatalf("plugin route remains: %s %s", route.Method, route.Path)
		}
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		"GET /api/admin/theme/market/catalog", "GET /api/admin/theme/list",
		"POST /api/admin/task/exec", "GET /api/admin/client/:uuid/terminal",
		"GET /api/admin/clipboard", "GET /api/admin/notification/traffic-report/",
		"GET /api/admin/ping/", "POST /api/rpc2", "GET /api/clients/v2/rpc",
	} {
		if !routes[route] {
			t.Errorf("retained route missing: %s", route)
		}
	}
	for _, path := range []string{"/api/plugin/demo/index.html", "/api/admin/plugin/list", "/api/admin/plugin/market/catalog"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
			if w.Code != http.StatusNotFound {
				t.Errorf("%s %s: got %d, want 404", method, path, w.Code)
			}
		}
	}
}
