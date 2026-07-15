package httpbinder_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpbinder "github.com/shibukawa/httpbind-go"
)

func TestSwaggerUI_ServesHTML(t *testing.T) {
	h := httpbinder.SwaggerUI("/openapi.json")
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
	h := httpbinder.SwaggerUI("")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs/", nil))
	if !strings.Contains(rec.Body.String(), `url: "/openapi.json"`) {
		t.Fatalf("default spec url: %s", rec.Body.String())
	}
}
