package proxy

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
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
				// Rewrite HTML responses to fix asset paths for iframe embedding
				if stripPrefix != "" {
					ct := resp.Header.Get("Content-Type")
					if strings.Contains(ct, "text/html") {
						if err := rewriteHTMLBody(resp, stripPrefix); err != nil {
							return err
						}
					}
				}
				return nil
			},
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// rewriteHTMLBody modifies HTML responses so that assets and API calls
// from within an iframe resolve through the proxy prefix instead of the
// dashboard origin root.
func rewriteHTMLBody(resp *http.Response, prefix string) error {
	var body []byte
	var err error

	// Handle gzip-compressed responses
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, gErr := gzip.NewReader(resp.Body)
		if gErr != nil {
			return gErr
		}
		body, err = io.ReadAll(gr)
		gr.Close()
		resp.Header.Del("Content-Encoding")
	} else {
		body, err = io.ReadAll(resp.Body)
	}
	resp.Body.Close()
	if err != nil {
		return err
	}

	s := string(body)

	// 1. Fix existing <base href="/"> tags (Grafana, MinIO, etc.)
	//    Handle all common variants: <base href="/" />, <base href="/">, <base href="/"/>
	s = strings.Replace(s, `<base href="/" />`, `<base href="`+prefix+`/" />`, 1)
	s = strings.Replace(s, `<base href="/">`, `<base href="`+prefix+`/">`, 1)
	s = strings.Replace(s, `<base href="/"/>`, `<base href="`+prefix+`/"/>`, 1)

	// 2. Inject <base> tag if none exists (Next.js apps: Dify, Langfuse)
	if !strings.Contains(s, "<base ") {
		baseTag := `<base href="` + prefix + `/" />`
		s = strings.Replace(s, "<head>", "<head>"+baseTag, 1)
		// Also handle <head ...> with attributes
		if !strings.Contains(s, "<base ") {
			s = strings.Replace(s, "<Head>", "<Head>"+baseTag, 1)
		}
	}

	// 3. Rewrite absolute /_next/ paths in HTML (Next.js: Dify, Langfuse)
	s = strings.ReplaceAll(s, `"/_next/`, `"`+prefix+`/_next/`)
	s = strings.ReplaceAll(s, `'/_next/`, `'`+prefix+`/_next/`)

	// 4. Inject URL rewriter script to intercept dynamic fetch/XHR/script loading
	//    This runs before any other scripts to ensure all requests go through the proxy.
	rewriter := buildURLRewriterScript(prefix)
	if idx := strings.Index(s, "<script"); idx > 0 {
		s = s[:idx] + rewriter + s[idx:]
	} else if strings.Contains(s, "</head>") {
		s = strings.Replace(s, "</head>", rewriter+"</head>", 1)
	}

	resp.Body = io.NopCloser(strings.NewReader(s))
	resp.ContentLength = int64(len(s))
	resp.Header.Set("Content-Length", strconv.Itoa(len(s)))
	return nil
}

// buildURLRewriterScript returns a <script> block that intercepts fetch, XHR,
// and dynamic script/link element creation so that absolute paths within an
// iframe are automatically prefixed with the proxy path.
func buildURLRewriterScript(prefix string) string {
	return `<script data-proxy-rewriter>
(function(){
var P="` + prefix + `";
function rw(u){
  if(typeof u!=="string")return u;
  if(u.charAt(0)==="/"&&u.indexOf(P)!==0&&u.indexOf("/services/")!==0){return P+u;}
  return u;
}
/* fetch */
var oF=window.fetch;
window.fetch=function(u,o){
  if(typeof u==="string"){u=rw(u);}
  else if(u instanceof Request){u=new Request(rw(u.url),u);}
  return oF.call(this,u,o);
};
/* XMLHttpRequest */
var oX=XMLHttpRequest.prototype.open;
XMLHttpRequest.prototype.open=function(){
  if(typeof arguments[1]==="string"){arguments[1]=rw(arguments[1]);}
  return oX.apply(this,arguments);
};
/* dynamic script/link elements */
var oC=document.createElement.bind(document);
document.createElement=function(tag){
  var el=oC(tag);
  var t=tag.toLowerCase();
  if(t==="script"){
    var sd=Object.getOwnPropertyDescriptor(HTMLScriptElement.prototype,"src");
    if(sd&&sd.set){var ss=sd.set;Object.defineProperty(el,"src",{set:function(v){ss.call(this,rw(v));},get:function(){return this.getAttribute("src")||"";},configurable:true});}
  }
  if(t==="link"){
    var ld=Object.getOwnPropertyDescriptor(HTMLLinkElement.prototype,"href");
    if(ld&&ld.set){var ls=ld.set;Object.defineProperty(el,"href",{set:function(v){ls.call(this,rw(v));},get:function(){return this.getAttribute("href")||"";},configurable:true});}
  }
  return el;
};
})();
</script>
`
}
