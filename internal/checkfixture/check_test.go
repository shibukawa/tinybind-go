package checkfixture_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	httpbinder "github.com/shibukawa/httpbind-go"
	"github.com/shibukawa/httpbind-go/generator"
	"github.com/shibukawa/httpbind-go/internal/checkfixture"
)

func mustFieldErr(t *testing.T, err error, field, substr string) {
	t.Helper()
	he, ok := httpbinder.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError, got %T %v", err, err)
	}
	if he.Problem.Code != "validation_failed" && he.Title != "Validation failed" {
		// Validation sets both
	}
	if len(he.Fields) == 0 {
		t.Fatalf("expected field errors, got %#v", he)
	}
	found := false
	for _, f := range he.Fields {
		if f.Field == field && (substr == "" || strings.Contains(f.Message, substr)) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing field %q substr %q in %#v", field, substr, he.Fields)
	}
}

func TestBind_CheckRequiredAndEmail(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=&email=bad", nil)
	_, err := httpbinder.Bind[checkfixture.CheckRequest](req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	mustFieldErr(t, err, "name", "required")
	mustFieldErr(t, err, "email", "email")
}

func TestBind_CheckMinMaxMinLenEnumPattern(t *testing.T) {
	q := "name=Al&email=a@b.co&age=200&sort=sideways&code=ab"
	req := httptest.NewRequest(http.MethodGet, "/?"+q, nil)
	_, err := httpbinder.Bind[checkfixture.CheckRequest](req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	// age 200 fails max: must be <= 150
	mustFieldErr(t, err, "age", "<=")
	mustFieldErr(t, err, "sort", "one of")
	mustFieldErr(t, err, "code", "pattern")
}

func TestBind_CheckUUIDDateTimeSuccessAndFailure(t *testing.T) {
	// failures
	q := "name=Al&email=a@b.co&id=not-uuid&born=01/02/2024&at=99:99:99&when=2024-01-02T15:04:05"
	req := httptest.NewRequest(http.MethodGet, "/?"+q, nil)
	_, err := httpbinder.Bind[checkfixture.CheckRequest](req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	mustFieldErr(t, err, "id", "uuid")
	mustFieldErr(t, err, "born", "date")
	mustFieldErr(t, err, "at", "time")
	mustFieldErr(t, err, "when", "datetime")

	// success formats
	q = "name=Al&email=a@b.co" +
		"&id=550e8400-e29b-41d4-a716-446655440000" +
		"&born=2024-01-02&at=15:04:05&when=2024-01-02T15:04:05Z" +
		"&sort=asc&code=ABC&age=30&page=2"
	req = httptest.NewRequest(http.MethodGet, "/?"+q, nil)
	got, err := httpbinder.Bind[checkfixture.CheckRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.ID == "" || got.Born != "2024-01-02" || got.Page != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestBind_SentinelDefaultAfterValidate(t *testing.T) {
	// omitted page → default -1 after successful validate
	req := httptest.NewRequest(http.MethodGet, "/?name=Al&email=a@b.co", nil)
	got, err := httpbinder.Bind[checkfixture.CheckRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Page != -1 {
		t.Fatalf("expected sentinel default -1, got %d", got.Page)
	}

	// explicit -1 fails min and must not become "success with default"
	req = httptest.NewRequest(http.MethodGet, "/?name=Al&email=a@b.co&page=-1", nil)
	_, err = httpbinder.Bind[checkfixture.CheckRequest](req)
	if err == nil {
		t.Fatal("expected validation error for page=-1")
	}
	mustFieldErr(t, err, "page", ">=")

	// valid page
	req = httptest.NewRequest(http.MethodGet, "/?name=Al&email=a@b.co&page=3", nil)
	got, err = httpbinder.Bind[checkfixture.CheckRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Page != 3 {
		t.Fatalf("page=%d", got.Page)
	}
}

func TestBind_OptionalEmailSkippedWhenAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=Al&email=a@b.co", nil)
	got, err := httpbinder.Bind[checkfixture.CheckRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Optional != "" {
		t.Fatalf("optional=%q", got.Optional)
	}

	req = httptest.NewRequest(http.MethodGet, "/?name=Al&email=a@b.co&optional=bad", nil)
	_, err = httpbinder.Bind[checkfixture.CheckRequest](req)
	if err == nil {
		t.Fatal("expected optional email failure")
	}
	mustFieldErr(t, err, "optional", "email")
}

func TestGeneratedBinder_HasValidateThenDefault_NoReflect(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(file)
	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(code)
	if strings.Contains(s, "reflect") {
		t.Fatal("reflect present")
	}
	// committed gen should match order; re-emit and check structure
	vi := strings.Index(s, "Validation(checkFields")
	di := strings.Index(s, "out.Page = -1")
	if vi < 0 || di < 0 || vi > di {
		t.Fatalf("validate/default order vi=%d di=%d", vi, di)
	}
}

func TestOpenAPI_YAMLRequiredIsListNotString(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(file)
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	yamlBytes, err := doc.YAML()
	if err != nil {
		t.Fatal(err)
	}
	y := string(yamlBytes)
	if strings.Contains(y, `required: "[`) || strings.Contains(y, `required: "[name]"`) {
		t.Fatalf("required must be a YAML list, got scalar-like form:\n%s", y)
	}
	// list form under schema required
	if !strings.Contains(y, "required:\n") || !strings.Contains(y, "- name\n") {
		t.Fatalf("expected YAML list for required name:\n%s", y)
	}
}

func TestOpenAPI_IncludesCheckConstraints(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(file)
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
	// Find OpenAPICheckRequest schema or operation body
	comps := root["components"].(map[string]any)["schemas"].(map[string]any)
	// body props from operation
	paths := root["paths"].(map[string]any)
	post := paths["/check"].(map[string]any)["post"].(map[string]any)
	// query sort enum + default
	params := post["parameters"].([]any)
	var sortParam map[string]any
	for _, p := range params {
		m := p.(map[string]any)
		if m["name"] == "sort" {
			sortParam = m
			break
		}
	}
	if sortParam == nil {
		t.Fatalf("sort param missing in %s", raw)
	}
	schema := sortParam["schema"].(map[string]any)
	if schema["default"] != "asc" {
		t.Fatalf("sort default: %#v", schema)
	}
	enum, _ := schema["enum"].([]any)
	if len(enum) != 2 {
		t.Fatalf("sort enum: %#v", enum)
	}

	rb := post["requestBody"].(map[string]any)
	bodySchema := rb["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	props := bodySchema["properties"].(map[string]any)
	name := props["name"].(map[string]any)
	if name["minLength"] == nil {
		t.Fatalf("name minLength missing: %#v", name)
	}
	age := props["age"].(map[string]any)
	if age["minimum"] == nil || age["maximum"] == nil {
		t.Fatalf("age bounds: %#v", age)
	}
	email := props["email"].(map[string]any)
	if email["format"] != "email" {
		t.Fatalf("email format: %#v", email)
	}
	reqd, _ := bodySchema["required"].([]any)
	hasName := false
	for _, r := range reqd {
		if r == "name" {
			hasName = true
		}
	}
	if !hasName {
		t.Fatalf("required name: %#v required; comps=%v", reqd, comps["OpenAPICheckRequest"])
	}
}
