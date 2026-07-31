package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// A default tag on its own used to be read by nobody: no assignment, no OpenAPI
// entry, and no error either. Both outputs now come from the tag alone.
func TestDefaultTag_WithoutCheckTag(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Req struct {
	Sort string ` + "`payload:\"sort\" default:\"asc\"`" + `
	Page int    ` + "`payload:\"page\" default:\"1\"`" + `
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
	for _, want := range []string{"presentSort", `out.Sort = "asc"`, "presentPage", "out.Page = 1"} {
		if !strings.Contains(s, want) {
			t.Fatalf("generated code missing %q:\n%s", want, s)
		}
	}

	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	props := schemas["Req"].(map[string]any)["properties"].(map[string]any)
	if got := props["sort"].(map[string]any)["default"]; got != "asc" {
		t.Fatalf("sort default = %#v, want \"asc\"", got)
	}
	if got := props["page"].(map[string]any)["default"]; got != int64(1) {
		t.Fatalf("page default = %#v, want 1", got)
	}
}

func TestAnalyzePackage_InvalidDefaultFails(t *testing.T) {
	for name, src := range map[string]string{
		"unparsable": "package sample\n\ntype Bad struct {\n\tX int `default:\"nope\"`\n}\n",
		"composite":  "package sample\n\ntype Bad struct {\n\tX []string `payload:\"x\" default:\"a\"`\n}\n",
		"check_rule": "package sample\n\ntype Bad struct {\n\tX int `check:\"default=1\"`\n}\n",
		"file_upload": "package sample\n\nimport httpbind \"github.com/shibukawa/tinybind-go\"\n\n" +
			"type Bad struct {\n\tX httpbind.File `payload:\"x\" default:\"a\"`\n}\n",
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
