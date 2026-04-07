package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthInjector modifies the request before forwarding (e.g. adds auth headers).
type AuthInjector func(c *gin.Context, req *http.Request)

// NewReverseProxy creates a Gin handler that proxies requests to targetURL,
// stripping the given prefix from the path.
func NewReverseProxy(targetURL, stripPrefix string, authInject AuthInjector) gin.HandlerFunc {
	target, _ := url.Parse(targetURL)

	return func(c *gin.Context) {
		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.Host = target.Host

				// Strip prefix from path
				path := c.Request.URL.Path
				if stripPrefix != "" {
					path = strings.TrimPrefix(path, stripPrefix)
					if path == "" {
						path = "/"
					}
				}
				req.URL.Path = path
				req.URL.RawQuery = c.Request.URL.RawQuery

				// Inject auth headers
				if authInject != nil {
					authInject(c, req)
				}
			},
			ModifyResponse: func(resp *http.Response) error {
				// Remove X-Frame-Options to allow iframe embedding
				resp.Header.Del("X-Frame-Options")
				resp.Header.Del("Content-Security-Policy")
				// Rewrite absolute-path Location headers to include the proxy prefix
				// e.g. upstream returns "Location: /login" → "/services/grafana/login"
				if loc := resp.Header.Get("Location"); loc != "" && stripPrefix != "" {
					if strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, stripPrefix) {
						resp.Header.Set("Location", stripPrefix+loc)
					}
				}
				return nil
			},
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
