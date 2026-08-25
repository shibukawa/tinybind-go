package generator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// bindOneRequest plans a request struct holding exactly the fields given, so a
// refusal names them and nothing else can mask it.
func bindOneRequest(t *testing.T, fields string) (string, error) {
	t.Helper()
	src := "package main\n\nimport \"github.com/shibukawa/tinybind-go\"\n\n" +
		"type Req struct {\n\t" + fields + "\n}\n\n" +
		"var _ = func() (Req, error) { return httpbind.Bind[Req](nil) }\n\nfunc main() {}\n"
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackageWithOptions(dir, generator.DefaultOptions())
	if err != nil {
		return "", err
	}
	code, err := generator.Emit(plan)
	if err != nil {
		return "", err
	}
	return string(code), nil
}

// A repeated key is the array a URL carries and a checkbox group submits, so a
// slice of scalar behind an explicit query tag binds every value it holds.
func TestQueryTagBindsASliceOfScalar(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		want        []string
	}{
		{
			name:  "string slice",
			field: "Tags []string `query:\"tag\"`",
			want: []string{
				`for _, qv := range httpbind.QueryLookupAll(queryVals, "tag")`,
				"out.Tags = append(out.Tags, qv)",
			},
		},
		{
			name:  "int slice",
			field: "Sizes []int `query:\"size\"`",
			want: []string{
				`for _, qv := range httpbind.QueryLookupAll(queryVals, "size")`,
				"httpbind.ParseInt(qv)",
				"out.Sizes = append(out.Sizes, v)",
				`httpbind.BindError("size", "query", "invalid int")`,
			},
		},
		{
			name:  "sized int slice narrows at its declared width",
			field: "IDs []int32 `query:\"id\"`",
			want: []string{
				"httpbind.ParseIntBits(qv, 32)",
				"out.IDs = append(out.IDs, int32(v))",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, err := bindOneRequest(t, tc.field)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(code, want) {
					t.Fatalf("generated source is missing %q:\n%s", want, code)
				}
			}
		})
	}
}

// The array spelling stops at the query. A path segment carries one value, and
// a repeated header or cookie is a question about those protocols.
func TestSliceIsStillRefusedOutsideTheQuery(t *testing.T) {
	for _, field := range []string{
		"Tags []string `path:\"tag\"`",
		"Tags []string `header:\"X-Tag\"`",
		"Tags []string `cookie:\"tag\"`",
	} {
		if _, err := bindOneRequest(t, field); err == nil {
			t.Errorf("%s was accepted, want the payload/input refusal", field)
		}
	}
}

// An object still needs a document, so widening the query did not widen that.
func TestQueryTagStillRefusesASliceOfStruct(t *testing.T) {
	fields := "Items []Item `query:\"item\"`\n}\n\ntype Item struct {\n\tName string `json:\"name\"`"
	if _, err := bindOneRequest(t, fields); err == nil {
		t.Error("a slice of struct bound from the query, want a refusal")
	}
}

// An untagged slice is the JSON case and must keep coming from the body, which
// is what makes the query widening additive.
func TestUntaggedSliceStaysBodyBound(t *testing.T) {
	code, err := bindOneRequest(t, "Tags []string")
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if strings.Contains(code, "QueryLookupAll") {
		t.Errorf("an untagged slice read the query:\n%s", code)
	}
}

// The binder reads an array there, so the document has to say array. Nothing
// else is written: form/explode-true is the OpenAPI default for a query
// parameter, and that is already the repeated spelling.
func TestOpenAPIDocumentsARepeatedQueryParameterAsAnArray(t *testing.T) {
	src := "package main\n\nimport \"github.com/shibukawa/tinybind-go\"\n\n" +
		"// Req is the filter.\ntype Req struct {\n\tTags []string `query:\"tag\"`\n}\n\n" +
		"func handler(w http.ResponseWriter, r *http.Request) {\n\t_, _ = httpbind.Bind[Req](r)\n}\n\n" +
		"func main() { http.HandleFunc(\"GET /search\", handler) }\n"
	src = strings.Replace(src, "import \"github.com/shibukawa/tinybind-go\"",
		"import (\n\t\"net/http\"\n\n\t\"github.com/shibukawa/tinybind-go\"\n)", 1)

	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	got := string(encoded)
	for _, want := range []string{`"name":"tag"`, `"in":"query"`, `"type":"array"`, `"items":{"type":"string"}`} {
		if !strings.Contains(got, want) {
			t.Fatalf("document is missing %q:\n%s", want, got)
		}
	}
}
