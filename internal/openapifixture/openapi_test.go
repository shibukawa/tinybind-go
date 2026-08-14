package openapifixture_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/generator"
	_ "github.com/shibukawa/tinybind-go/internal/openapifixture" // register generated OpenAPI
)

func TestOpenAPIServe_JSON(t *testing.T) {
	// Real serve path after package init registered generated document.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	httpbind.OpenAPIJSON(rec, req)
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
	reg, err := httpbind.AssembleOpenAPI()
	if err != nil {
		t.Fatal(err)
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
