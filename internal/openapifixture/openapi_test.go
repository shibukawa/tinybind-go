package openapifixture_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/httpbind-go"
	"github.com/shibukawa/httpbind-go/generator"
	_ "github.com/shibukawa/httpbind-go/internal/openapifixture" // register generated OpenAPI
)

func TestOpenAPIServe_JSONAndYAML(t *testing.T) {
	// Real serve path after package init registered generated document.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	httpbinder.OpenAPIJSON(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type %q", ct)
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, rec.Body.String())
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("openapi: %#v", doc["openapi"])
	}
	paths := doc["paths"].(map[string]any)
	if paths["/orgs/{org_id}/users"] == nil {
		t.Fatalf("paths: %#v", paths)
	}

	recY := httptest.NewRecorder()
	httpbinder.OpenAPIYAML(recY, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if recY.Code != http.StatusOK {
		t.Fatalf("yaml status %d", recY.Code)
	}
	if ct := recY.Header().Get("Content-Type"); !strings.Contains(ct, "yaml") {
		t.Fatalf("yaml content-type %q", ct)
	}
	body := recY.Body.String()
	if !strings.Contains(body, "openapi: 3.1.0") && !strings.Contains(body, "openapi: \"3.1.0\"") {
		// our yaml writer emits: openapi: 3.1.0
		if !strings.HasPrefix(strings.TrimSpace(body), "components:") && !strings.Contains(body, "openapi:") {
			t.Fatalf("yaml body: %s", body[:min(200, len(body))])
		}
	}
	if !strings.Contains(body, "/orgs/{org_id}/users") {
		t.Fatalf("yaml missing path: %s", body[:min(400, len(body))])
	}
}

func TestOpenAPI_SourceOfTruthIsGoGeneration(t *testing.T) {
	// Rebuild from Go sources and compare key facts to the registered document
	// (which came from the same generator path, committed for tests).
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// package dir is openapifixture
	built, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatal(err)
	}
	builtJSON, err := built.JSON()
	if err != nil {
		t.Fatal(err)
	}
	reg := httpbinder.OpenAPIDocumentJSON()
	if len(reg) == 0 {
		t.Fatal("no registered openapi")
	}
	// Both must be OpenAPI 3.1 with same routes from handlers.go
	var a, b map[string]any
	if err := json.Unmarshal(builtJSON, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(reg, &b); err != nil {
		t.Fatal(err)
	}
	if a["openapi"] != "3.1.0" || b["openapi"] != "3.1.0" {
		t.Fatalf("versions a=%v b=%v", a["openapi"], b["openapi"])
	}
	ap := a["paths"].(map[string]any)
	bp := b["paths"].(map[string]any)
	for _, p := range []string{"/orgs/{org_id}/users", "/search", "/users/{org_id}"} {
		if ap[p] == nil || bp[p] == nil {
			t.Fatalf("path %s missing in generated=%v registered=%v", p, ap[p] != nil, bp[p] != nil)
		}
	}
	// Ensure no openapi.yaml exists as primary input in this package
	if _, err := os.Stat(filepath.Join(dir, "openapi.yaml")); err == nil {
		t.Fatal("handwritten openapi.yaml must not be package source of truth")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
