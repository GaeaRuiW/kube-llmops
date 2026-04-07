package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestReverseProxy_RewritesBaseHref(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><head><base href="/" /><script src="public/app.js"></script></head><body>Hi</body></html>`))
	}))
	defer backend.Close()

	r := gin.New()
	r.Any("/services/grafana/*path", NewReverseProxy(backend.URL, "/services/grafana", nil))

	frontend := httptest.NewServer(r)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/services/grafana/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	html := string(body)
	if !strings.Contains(html, `<base href="/services/grafana/" />`) {
		t.Errorf("expected rewritten <base> tag, got:\n%s", html)
	}
	if !strings.Contains(html, `data-proxy-rewriter`) {
		t.Errorf("expected URL rewriter script injection, got:\n%s", html)
	}
}

func TestReverseProxy_InjectsBaseForNextJS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><head><link href="/_next/static/chunks/foo.css" rel="stylesheet"/><script src="/_next/static/chunks/bar.js"></script></head><body></body></html>`))
	}))
	defer backend.Close()

	r := gin.New()
	r.Any("/services/dify/*path", NewReverseProxy(backend.URL, "/services/dify", nil))

	frontend := httptest.NewServer(r)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/services/dify/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	html := string(body)
	// Should inject <base> since there wasn't one
	if !strings.Contains(html, `<base href="/services/dify/"`) {
		t.Errorf("expected injected <base> tag, got:\n%s", html)
	}
	// Should rewrite absolute /_next/ paths
	if strings.Contains(html, `"/_next/static`) {
		t.Errorf("expected /_next/ paths to be rewritten, but found unrewritten path in:\n%s", html)
	}
	if !strings.Contains(html, `"/services/dify/_next/static`) {
		t.Errorf("expected rewritten /_next/ path, got:\n%s", html)
	}
}

func TestReverseProxy_NoRewriteForNonHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jsContent := `var x = "/_next/foo"; console.log(x);`
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Write([]byte(jsContent))
	}))
	defer backend.Close()

	r := gin.New()
	r.Any("/services/dify/*path", NewReverseProxy(backend.URL, "/services/dify", nil))

	frontend := httptest.NewServer(r)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/services/dify/script.js")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body) != jsContent {
		t.Errorf("JS content should not be rewritten, got: %s", string(body))
	}
}
