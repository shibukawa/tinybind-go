package httpbinder_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpbinder "github.com/shibukawa/httpbind-go"
)

func TestOpenAPIJSON_Unregistered(t *testing.T) {
	// Ensure handler responds (problem or empty path) without panicking when unset.
	// Other packages may have registered docs in the same process; re-register empty.
	httpbinder.RegisterOpenAPI(nil, nil)
	rec := httptest.NewRecorder()
	httpbinder.OpenAPIJSON(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code == http.StatusOK && strings.TrimSpace(rec.Body.String()) != "" {
		// if another test registered, still OK as long as valid response
		return
	}
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestOpenAPIJSON_Registered(t *testing.T) {
	doc := []byte(`{"openapi":"3.1.0","info":{"title":"t","version":"0"},"paths":{}}`)
	httpbinder.RegisterOpenAPI(doc, []byte("openapi: 3.1.0\n"))
	rec := httptest.NewRecorder()
	httpbinder.OpenAPIJSON(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"openapi":"3.1.0"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
	recY := httptest.NewRecorder()
	httpbinder.OpenAPIYAML(recY, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if recY.Code != http.StatusOK || !strings.Contains(recY.Body.String(), "openapi:") {
		t.Fatalf("yaml %d %s", recY.Code, recY.Body.String())
	}
}
