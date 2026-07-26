package generator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
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

func TestBuildOpenAPI_GodocBecomesDocumentation(t *testing.T) {
	dir := filepath.Join("..", "internal", "openapifixture")
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	raw, err := doc.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	paths := root["paths"].(map[string]any)

	// Handler godoc: first sentence is the summary, the rest the description.
	post := paths["/orgs/{org_id}/users"].(map[string]any)["post"].(map[string]any)
	if post["summary"] != "createUserHandler creates one organization user." {
		t.Fatalf("operation summary: %#v", post["summary"])
	}
	description, _ := post["description"].(string)
	if !strings.Contains(description, "Input is accepted from the JSON body") {
		t.Fatalf("operation description: %#v", post["description"])
	}
	if strings.Contains(description, "creates one organization user") {
		t.Fatalf("summary sentence must not repeat in description: %q", description)
	}
	if _, ok := post["deprecated"]; ok {
		t.Fatalf("operation must not be deprecated: %#v", post["deprecated"])
	}

	// A "Deprecated:" paragraph marks the operation deprecated.
	deprecated := paths["/users/{org_id}"].(map[string]any)["get"].(map[string]any)
	if deprecated["deprecated"] != true {
		t.Fatalf("deprecated operation: %#v", deprecated)
	}

	// Type godoc documents the schema, field godoc its properties.
	schema := root["components"].(map[string]any)["schemas"].(map[string]any)["CreateUserRequest"].(map[string]any)
	if schema["description"] != "CreateUserRequest exercises default input, path, and header for OpenAPI mapping." {
		t.Fatalf("schema description: %#v", schema["description"])
	}
	props := schema["properties"].(map[string]any)
	name := props["name"].(map[string]any)
	if name["description"] != "Name is the display name of the new user." {
		t.Fatalf("property description: %#v", name["description"])
	}
	// A line comment documents the field too.
	orgID := props["org_id"].(map[string]any)
	if orgID["description"] != "OrgID owns the created user." {
		t.Fatalf("line-comment property description: %#v", orgID["description"])
	}

	// Parameters carry the doc on the parameter object, not inside its schema.
	var found bool
	for _, p := range post["parameters"].([]any) {
		param := p.(map[string]any)
		if param["name"] != "org_id" {
			continue
		}
		found = true
		if param["description"] != "OrgID owns the created user." {
			t.Fatalf("parameter description: %#v", param["description"])
		}
		if _, ok := param["schema"].(map[string]any)["description"]; ok {
			t.Fatalf("parameter schema must not duplicate the description: %#v", param["schema"])
		}
	}
	if !found {
		t.Fatalf("org_id parameter missing: %#v", post["parameters"])
	}
}

func TestBuildOpenAPI_RequiredIsJSONArray(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Req struct {
	Name string ` + "`payload:\"name\" check:\"required\"`" + `
}

type Resp struct {
	OK bool ` + "`json:\"ok\"`" + `
}

func init() {
	http.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {
		_, err := httpbind.Bind[Req](r)
		if err != nil {
			httpbind.WriteError(w, r, err)
			return
		}
		_ = httpbind.Write[Resp](w, r, Resp{OK: true})
	})
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	paths := doc["paths"].(map[string]any)
	post := paths["/x"].(map[string]any)["post"].(map[string]any)
	responses := post["responses"].(map[string]any)
	validation, ok := responses["400"].(map[string]any)
	if !ok || validation["description"] != "Validation" {
		t.Fatalf("check validation response missing: %#v", responses)
	}
	raw, err := doc.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	schema := root["components"].(map[string]any)["schemas"].(map[string]any)["Req"].(map[string]any)
	required, ok := schema["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "name" {
		t.Fatalf("required must be a JSON array of names: %#v", schema["required"])
	}
}

func TestGenerateOpenAPI_EmitsRegisterNotHandwrittenYAML(t *testing.T) {
	srcDir := filepath.Join("..", "internal", "openapifixture")
	out := t.TempDir()
	path, err := generator.GenerateOpenAPI(srcDir, out, "tinybind_openapi_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	code := string(data)
	if !strings.Contains(code, `RegisterOpenAPIFragment("github.com/shibukawa/tinybind-go/internal/openapifixture"`) {
		t.Fatalf("missing package-qualified RegisterOpenAPIFragment:\n%s", code)
	}
	if !strings.Contains(code, `\"paths\"`) || !strings.Contains(code, `\"components\"`) {
		t.Fatalf("missing package fragment data in embed:\n%s", code[:min(500, len(code))])
	}
	if strings.Contains(code, `\"info\"`) || strings.Contains(code, `\"openapi\"`) {
		t.Fatalf("package fragment must not own final document metadata:\n%s", code[:min(500, len(code))])
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
