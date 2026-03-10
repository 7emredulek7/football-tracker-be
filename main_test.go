package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/user/7aside-tracker/routes"
)

func TestHealthCheckRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	routes.SetupRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected to get status %d but instead got %d", http.StatusOK, w.Code)
	}
}
