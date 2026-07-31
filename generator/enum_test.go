package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

func TestParseEnumTag(t *testing.T) {
	e, err := generator.ParseEnumTag("asc, desc", "string")
	if err != nil {
		t.Fatal(err)
	}
	if !e.Set || len(e.Values) != 2 || e.Values[0] != "asc" || e.Values[1] != "desc" {
		t.Fatalf("%+v", e)
	}

	// Values are still checked against the field type, as they were in check.
	if _, err := generator.ParseEnumTag("1,2", "int"); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ raw, kind string }{
		{"", "string"},
		{"a,,b", "string"},
		{"true,maybe", "bool"},
		{"1,two", "int"},
		{"a,b", "file"},
		{"a,b", generator.KindStruct},
		{"a,b", generator.KindRestAny},
	} {
		if _, err := generator.ParseEnumTag(tc.raw, tc.kind); err == nil {
			t.Fatalf("expected error for enum %q on %s", tc.raw, tc.kind)
		}
	}
}

func TestEnumTag_ValidatesAndDocuments(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Req struct {
	Sort string ` + "`query:\"sort\" enum:\"asc,desc\" default:\"asc\"`" + `
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

	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(code)
	for _, want := range []string{"enumOK", "must be one of: asc, desc", `out.Sort = "asc"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("generated code missing %q:\n%s", want, s)
		}
	}

	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	sortSchema := schemas["Req"].(map[string]any)["properties"].(map[string]any)["sort"].(map[string]any)
	enums, ok := sortSchema["enum"].([]any)
	if !ok || len(enums) != 2 || enums[0] != "asc" || enums[1] != "desc" {
		t.Fatalf("enum schema = %#v", sortSchema["enum"])
	}

	// An enum can reject a value, so the validation response has to be documented
	// even though no check tag is present.
	post := doc["paths"].(map[string]any)["/x"].(map[string]any)["post"].(map[string]any)
	responses := post["responses"].(map[string]any)
	if validation, ok := responses["400"].(map[string]any); !ok || validation["description"] != "Validation" {
		t.Fatalf("enum-only field must document a validation response: %#v", responses)
	}
}

func TestAnalyzePackage_InvalidEnumFails(t *testing.T) {
	for name, src := range map[string]string{
		"empty":      "package sample\n\ntype Bad struct {\n\tX string `enum:\"\"`\n}\n",
		"type":       "package sample\n\ntype Bad struct {\n\tX int `enum:\"1,two\"`\n}\n",
		"composite":  "package sample\n\ntype Bad struct {\n\tX []string `payload:\"x\" enum:\"a,b\"`\n}\n",
		"check_rule": "package sample\n\ntype Bad struct {\n\tX string `check:\"enum=a|b\"`\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeTempModule(t, dir)
			if err := os.WriteFile(filepath.Join(dir, "t.go"), []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			tidyTempModule(t, dir)
			if _, err := generator.AnalyzePackage(dir); err == nil {
				t.Fatal("expected analyze error")
			}
		})
	}
}
