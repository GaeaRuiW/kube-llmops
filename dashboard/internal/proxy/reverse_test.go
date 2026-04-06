package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestReverseProxy_StripPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test backend that echoes the received path
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	}))
	defer backend.Close()

	r := gin.New()
	r.Any("/services/test/*path", NewReverseProxy(backend.URL, "/services/test", nil))

	// Use a real HTTP test server to avoid CloseNotifier issues with httptest.ResponseRecorder
	frontend := httptest.NewServer(r)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/services/test/api/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "/api/health" {
		t.Errorf("expected /api/health, got %s", string(body))
	}
}

func TestReverseProxy_AuthInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var receivedHeader string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom")
		w.Write([]byte("ok"))
	}))
	defer backend.Close()

	r := gin.New()
	customAuth := func(c *gin.Context, req *http.Request) {
		req.Header.Set("X-Custom", "injected")
	}
	r.Any("/proxy/*path", NewReverseProxy(backend.URL, "/proxy", customAuth))

	frontend := httptest.NewServer(r)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/proxy/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if receivedHeader != "injected" {
		t.Errorf("expected 'injected', got '%s'", receivedHeader)
	}
}

func TestReverseProxy_RemovesXFrameOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Write([]byte("ok"))
	}))
	defer backend.Close()

	r := gin.New()
	r.Any("/proxy/*path", NewReverseProxy(backend.URL, "/proxy", nil))

	frontend := httptest.NewServer(r)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/proxy/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if xfo := resp.Header.Get("X-Frame-Options"); xfo != "" {
		t.Errorf("X-Frame-Options should be stripped, got '%s'", xfo)
	}
}
