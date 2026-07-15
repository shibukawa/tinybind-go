package generator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/httpbind-go/generator"
)

func TestBuildOpenAPI_FromFixtureGoSource(t *testing.T) {
	dir := filepath.Join("..", "internal", "openapifixture")
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("openapi version: %#v", doc["openapi"])
	}

	raw, err := doc.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}

	paths, _ := root["paths"].(map[string]any)
	postPath, ok := paths["/orgs/{org_id}/users"].(map[string]any)
	if !ok {
		t.Fatalf("missing path /orgs/{{org_id}}/users in %s", string(raw))
	}
	post, ok := postPath["post"].(map[string]any)
	if !ok {
		t.Fatal("missing post operation")
	}

	// parameters: path org_id, header Authorization, query name/email
	params, _ := post["parameters"].([]any)
	inName := map[string]string{}
	for _, p := range params {
		m := p.(map[string]any)
		inName[m["name"].(string)] = m["in"].(string)
	}
	if inName["org_id"] != "path" {
		t.Fatalf("org_id param: %#v", inName)
	}
	if inName["Authorization"] != "header" {
		t.Fatalf("Authorization param: %#v", inName)
	}
	if inName["name"] != "query" || inName["email"] != "query" {
		t.Fatalf("input query params: %#v", inName)
	}

	// body media types for input fields
	rb, _ := post["requestBody"].(map[string]any)
	content, _ := rb["content"].(map[string]any)
	for _, mt := range []string{
		"application/json",
		"application/x-www-form-urlencoded",
		"multipart/form-data",
	} {
		if _, ok := content[mt]; !ok {
			t.Fatalf("missing request body media type %s", mt)
		}
	}
	// body properties only name/email (input), not path/header
	jsonSchema := content["application/json"].(map[string]any)["schema"].(map[string]any)
	props := jsonSchema["properties"].(map[string]any)
	if _, ok := props["name"]; !ok {
		t.Fatalf("body props: %#v", props)
	}
	if _, ok := props["org_id"]; ok {
		t.Fatalf("path field must not be in body: %#v", props)
	}

	// 200 + CreateUserResponse
	resps := post["responses"].(map[string]any)
	ok200 := resps["200"].(map[string]any)
	schema := ok200["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	if schema["$ref"] != "#/components/schemas/CreateUserResponse" {
		t.Fatalf("200 schema: %#v", schema)
	}
	// Validation + Conflict
	if resps["400"] == nil || resps["409"] == nil {
		t.Fatalf("error responses: %#v", resps)
	}
	ct400 := resps["400"].(map[string]any)["content"].(map[string]any)
	if _, ok := ct400["application/problem+json"]; !ok {
		t.Fatalf("400 media: %#v", ct400)
	}

	// search: query-only keyword/page, payload filter body-only
	search := paths["/search"].(map[string]any)["get"].(map[string]any)
	sparams := search["parameters"].([]any)
	for _, p := range sparams {
		m := p.(map[string]any)
		if m["name"] == "filter" {
			t.Fatal("payload filter must not be query param")
		}
		if m["in"] != "query" {
			t.Fatalf("search param %#v", m)
		}
	}
	sbody := search["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	sprops := sbody["properties"].(map[string]any)
	if _, ok := sprops["filter"]; !ok {
		t.Fatalf("filter in body: %#v", sprops)
	}
	if _, ok := sprops["keyword"]; ok {
		t.Fatalf("query field must not be in body: %#v", sprops)
	}
}

func TestBuildOpenAPI_YAMLRequiredList(t *testing.T) {
	dir := t.TempDir()
	src := `package sample

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type Req struct {
	Name string ` + "`payload:\"name\" check:\"required\"`" + `
}

type Resp struct {
	OK bool ` + "`json:\"ok\"`" + `
}

func init() {
	http.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {
		_, err := httpbinder.Bind[Req](r)
		if err != nil {
			httpbinder.WriteError(w, r, err)
			return
		}
		_ = httpbinder.Write[Resp](w, r, Resp{OK: true})
	})
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	y, err := doc.YAML()
	if err != nil {
		t.Fatal(err)
	}
	ys := string(y)
	if strings.Contains(ys, `required: "[name]"`) || strings.Contains(ys, `required: "[`) {
		t.Fatalf("invalid required YAML scalar:\n%s", ys)
	}
	if !strings.Contains(ys, "required:\n") || !strings.Contains(ys, "- name\n") {
		t.Fatalf("expected required YAML list:\n%s", ys)
	}
}

func TestGenerateOpenAPI_EmitsRegisterNotHandwrittenYAML(t *testing.T) {
	srcDir := filepath.Join("..", "internal", "openapifixture")
	out := t.TempDir()
	path, err := generator.GenerateOpenAPI(srcDir, out, "httpbinder_openapi_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	code := string(data)
	if !strings.Contains(code, "RegisterOpenAPI") {
		t.Fatalf("missing RegisterOpenAPI:\n%s", code)
	}
	if !strings.Contains(code, `"openapi": "3.1.0"`) && !strings.Contains(code, `\"openapi\": \"3.1.0\"`) {
		t.Fatalf("missing openapi 3.1 in embed:\n%s", code[:min(500, len(code))])
	}
	// must not be loading a .yaml/.json file as primary input
	if strings.Contains(code, "os.ReadFile") || strings.Contains(code, "openapi.yaml") {
		t.Fatal("generated openapi must not read handwritten yaml as source")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
