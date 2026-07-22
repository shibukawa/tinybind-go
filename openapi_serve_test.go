package httpbind_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
)

func TestOpenAPIJSON_Unregistered(t *testing.T) {
	// Ensure handler responds (problem or empty path) without panicking when unset.
	// Other packages may have registered docs in the same process; re-register empty.
	httpbind.RegisterOpenAPI(nil, nil)
	rec := httptest.NewRecorder()
	httpbind.OpenAPIJSON(rec, httptest.NewRequest(http.MethodGet, "/", nil))
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
	httpbind.RegisterOpenAPI(doc, []byte("openapi: 3.1.0\n"))
	rec := httptest.NewRecorder()
	httpbind.OpenAPIJSON(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"openapi": "3.1.0"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
	recY := httptest.NewRecorder()
	httpbind.OpenAPIYAML(recY, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if recY.Code != http.StatusOK || !strings.Contains(recY.Body.String(), "openapi:") {
		t.Fatalf("yaml %d %s", recY.Code, recY.Body.String())
	}
}

func TestOpenAPIFragmentsAggregateAcrossPackagesDeterministically(t *testing.T) {
	httpbind.ResetOpenAPIFragments()
	t.Cleanup(httpbind.ResetOpenAPIFragments)
	if err := httpbind.SetOpenAPIInfo(httpbind.OpenAPIInfo{Title: "Modular API", Version: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	framework := []byte(`{
		"openapi":"3.1.0",
		"paths":{"/health":{"get":{"responses":{"200":{"description":"OK"}}}}},
		"components":{"schemas":{"Config":{"type":"object","properties":{"healthy":{"type":"boolean"}}},"ProblemDetails":{"type":"object"}}}
	}`)
	application := []byte(`{
		"openapi":"3.1.0",
		"paths":{"/users":{"post":{"responses":{"201":{"description":"Created"}},"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Config"}}}}}}},
		"components":{"schemas":{"Config":{"type":"object","properties":{"name":{"type":"string"}}},"ProblemDetails":{"type":"object"}}}
	}`)

	httpbind.RegisterOpenAPIFragment("example/framework/health", framework)
	httpbind.RegisterOpenAPIFragment("example/app/users", application)
	firstJSON, firstYAML, err := httpbind.AssembleOpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(firstJSON, &doc); err != nil {
		t.Fatal(err)
	}
	paths := doc["paths"].(map[string]any)
	if paths["/health"] == nil || paths["/users"] == nil {
		t.Fatalf("merged paths=%v", paths)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	qualifiedConfigCount := 0
	for name := range schemas {
		if strings.HasSuffix(name, "__Config") {
			qualifiedConfigCount++
		}
	}
	if qualifiedConfigCount != 2 {
		t.Fatalf("qualified Config schemas=%d in %v", qualifiedConfigCount, schemas)
	}
	if _, ok := schemas["ProblemDetails"]; !ok {
		t.Fatalf("identical shared schema was not deduplicated: %v", schemas)
	}
	userPathJSON, err := json.Marshal(paths["/users"])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(userPathJSON, []byte(`#/components/schemas/Config"`)) || !bytes.Contains(userPathJSON, []byte("__Config")) {
		t.Fatalf("component reference was not rewritten: %s", userPathJSON)
	}
	info := doc["info"].(map[string]any)
	if info["title"] != "Modular API" || info["version"] != "1.2.3" {
		t.Fatalf("application info=%v", info)
	}
	if !bytes.Contains(firstYAML, []byte("/health:")) || !bytes.Contains(firstYAML, []byte("/users:")) {
		t.Fatalf("merged YAML:\n%s", firstYAML)
	}

	// Reversing registration order must not alter serialized output.
	httpbind.ResetOpenAPIFragments()
	if err := httpbind.SetOpenAPIInfo(httpbind.OpenAPIInfo{Title: "Modular API", Version: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	httpbind.RegisterOpenAPIFragment("example/app/users", application)
	httpbind.RegisterOpenAPIFragment("example/framework/health", framework)
	secondJSON, secondYAML, err := httpbind.AssembleOpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) || !bytes.Equal(firstYAML, secondYAML) {
		t.Fatal("OpenAPI output depends on registration order")
	}
}

func TestOpenAPIFragmentConflictsAreReported(t *testing.T) {
	httpbind.ResetOpenAPIFragments()
	t.Cleanup(httpbind.ResetOpenAPIFragments)
	httpbind.RegisterOpenAPIFragment("example/framework", []byte(`{"paths":{"/health":{"get":{"operationId":"frameworkHealth"}}}}`))
	httpbind.RegisterOpenAPIFragment("example/app", []byte(`{"paths":{"/health":{"get":{"operationId":"appHealth"}}}}`))
	if _, _, err := httpbind.AssembleOpenAPI(); err == nil || !strings.Contains(err.Error(), "conflicting OpenAPI operation") {
		t.Fatalf("conflict error=%v", err)
	}
}

func TestOpenAPIFragmentIdentityConflictIsReported(t *testing.T) {
	httpbind.ResetOpenAPIFragments()
	t.Cleanup(httpbind.ResetOpenAPIFragments)
	httpbind.RegisterOpenAPIFragment("example/module", []byte(`{"paths":{"/a":{}}}`))
	httpbind.RegisterOpenAPIFragment("example/module", []byte(`{"paths":{"/b":{}}}`))
	if _, _, err := httpbind.AssembleOpenAPI(); err == nil || !strings.Contains(err.Error(), "conflicting OpenAPI fragment ID") {
		t.Fatalf("identity error=%v", err)
	}
}
