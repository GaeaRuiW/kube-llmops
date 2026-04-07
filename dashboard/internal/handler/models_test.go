package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListModels_NilClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/models", ListModels(nil))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/models", nil)
	r.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestGetModel_NilClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/models/:name", GetModel(nil))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/models/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestCreateModel_NotImplemented(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/models", CreateModel(nil))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/models", nil)
	r.ServeHTTP(w, req)
	if w.Code != 501 {
		t.Errorf("expected 501 (Not Implemented), got %d", w.Code)
	}
}
