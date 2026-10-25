package httpbind_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/httpbind"
)

func TestSwaggerUI_ServesHTML(t *testing.T) {
	h := httpbind.SwaggerUI("/openapi.json")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/openapi.json") {
		t.Fatalf("missing spec url in body")
	}
	if !strings.Contains(body, "SwaggerUIBundle") {
		t.Fatalf("missing swagger ui bootstrap")
	}
	if !strings.Contains(body, "swagger-ui-dist") {
		t.Fatalf("missing CDN assets")
	}
}

func TestSwaggerUI_DefaultSpecURL(t *testing.T) {
	h := httpbind.SwaggerUI("")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs/", nil))
	if !strings.Contains(rec.Body.String(), `url: "/openapi.json"`) {
		t.Fatalf("default spec url: %s", rec.Body.String())
	}
}

func TestSwaggerUI_EscapesInlineScriptURL(t *testing.T) {
	h := httpbind.SwaggerUI(`</script><script>alert(1)</script>`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs/", nil))
	body := rec.Body.String()
	if strings.Contains(body, `</script><script>alert(1)</script>`) {
		t.Fatalf("unescaped script URL: %s", body)
	}
	if !strings.Contains(body, `\u003c/script\u003e`) {
		t.Fatalf("escaped URL missing: %s", body)
	}
}
