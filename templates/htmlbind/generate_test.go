package htmlbind_test

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

func TestGenerateFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "templates", "htmlbind")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var cases []string
	for _, entry := range entries {
		if entry.IsDir() {
			cases = append(cases, entry.Name())
		}
	}
	sort.Strings(cases)
	if len(cases) == 0 {
		t.Fatal("no HTML generator fixtures found")
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			inputPath := filepath.Join(dir, "input.txt")
			outputPath := filepath.Join(dir, "output.go")
			input, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatal(err)
			}
			got, err := htmlbind.Generate(inputPath, input, htmlbind.GenerateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(outputPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("generated Go mismatch\n--- want ---\n%s--- got ---\n%s", want, got)
			}
			runtimeTest, err := os.ReadFile(filepath.Join(dir, "runtime_test.go"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"net/http", "http.ResponseWriter", "runtimehtmlbind"} {
				if bytes.Contains(got, []byte(forbidden)) {
					t.Fatalf("generated component references %s:\n%s", forbidden, got)
				}
			}
			typeCheckGenerated(t, outputPath, got, declarationsOnly(t, runtimeTest))
			runGeneratedTests(t, got, runtimeTest)
		})
	}
}

func runGeneratedTests(t *testing.T, generated, runtimeTest []byte) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string][]byte{
		"go.mod": []byte("module generatedfixture\n\ngo 1.26\n\n" +
			"require github.com/shibukawa/tinybind-go v0.0.0\n\n" +
			"replace github.com/shibukawa/tinybind-go => " + root + "\n"),
		"generated.go": generated,
	}
	if len(runtimeTest) > 0 {
		files["runtime_test.go"] = runtimeTest
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("go", "test", "-mod=mod", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run generated Go: %v\n%s", err, output)
	}
}

func typeCheckGenerated(t *testing.T, filename string, source, companion []byte) {
	t.Helper()
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, filename, source, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse generated Go: %v", err)
	}
	parsed := []*ast.File{file}
	if len(companion) > 0 {
		companionFile, err := parser.ParseFile(files, "companion.go", companion, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse generated Go companion: %v", err)
		}
		parsed = append(parsed, companionFile)
	}
	config := types.Config{Importer: importer.Default(), Error: func(err error) { t.Errorf("generated Go type error: %v", err) }}
	if _, err := config.Check(file.Name.Name, files, parsed, nil); err != nil {
		t.Fatalf("type-check generated Go: %v", err)
	}
}

// declarationsOnly keeps only a fixture companion's helper declarations, so a
// companion written against the tinybind runtime can still supply the external
// functions the generated code calls. Running the companion itself is covered by
// runGeneratedTests, which compiles it against the real module.
func declarationsOnly(t *testing.T, source []byte) []byte {
	t.Helper()
	if len(source) == 0 {
		return nil
	}
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "companion.go", source, parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	kept := file.Decls[:0]
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && strings.HasPrefix(function.Name.Name, "Test") {
			continue
		}
		kept = append(kept, declaration)
	}
	file.Decls = kept
	for _, imported := range append([]*ast.ImportSpec(nil), file.Imports...) {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if astutil.UsesImport(file, path) {
			continue
		}
		alias := ""
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		astutil.DeleteNamedImport(files, file, alias, path)
	}
	var out bytes.Buffer
	if err := format.Node(&out, files, file); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestGenerateDiagnostics(t *testing.T) {
	tests := []struct{ name, source, want string }{
		{"unknown identifier", `component Bad(): html {<p>{missing}</p>}`, "unknown identifier missing"},
		{"wrong condition", `component Bad(name: string): html {{if name}x{/if}}`, "if condition must be bool"},
		{"unsafe script", `component Bad(value: string): html {<script>{value}</script>}`, "html:script requires"},
		{"unsafe raw context", `component Bad(value: string): html {<p title={RawHTML(value)}>x</p>}`, "cannot insert trusted_html"},
		{"optional raw input", `component Bad(value: string?): html {{RawHTML(value)}}`, "RawHTML expects string"},
		{"url type", `component Bad(value: string): html {<a href={value}>x</a>}`, "requires url"},
		{"optional mixed attribute", `component Bad(value: string?): html {<p title="prefix {value}">x</p>}`, "optional expression must be the entire attribute"},
		{"unsafe json field", `type Payload { target: url } component Bad(value: Payload): html {<script>{JsonForScript(value)}</script>}`, "not statically serializable"},
		{"noncomparable values", `component Bad(left: string[], right: string[]): html {{if left == right}x{/if}}`, "values are not comparable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := htmlbind.Generate("invalid.txt", []byte(test.source), htmlbind.GenerateOptions{Package: "invalid"})
			if err == nil || !bytes.Contains([]byte(err.Error()), []byte(test.want)) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestGenerateManglesGoKeywords(t *testing.T) {
	source := []byte(`package type
export component Keyword(type: string): html {<p>{type}</p>}`)
	generated, err := htmlbind.Generate("keywords.txt", source, htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"package _type", "Type string", "_type := _tinybindParams.Type"} {
		if !bytes.Contains(generated, []byte(want)) {
			t.Fatalf("generated Go does not mangle keywords (%q missing):\n%s", want, generated)
		}
	}
	typeCheckGenerated(t, "keywords.go", generated, nil)
}

func TestGenerateDiagnosticIncludesPosition(t *testing.T) {
	source := []byte("component Bad(): html {\n<p>\n{missing}\n</p>\n}")
	_, err := htmlbind.Generate("position.txt", source, htmlbind.GenerateOptions{})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("position.txt:3:2:")) {
		t.Fatalf("error = %v, want filename:line:col", err)
	}
}

func TestGenerateParameterShapes(t *testing.T) {
	source := []byte(`package pages

type User { name: string }

export component None(): html {<hr />}

export component One(user: User): html {<p>{user.name}</p>}

export component Many(user: User, tone: string, count: int): html {
<p class={tone}>{user.name}{count}</p>
}`)
	generated, err := htmlbind.Generate("shapes.tb.html", source, htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"type NoneParams struct{}",
		"func None(w io.Writer, _tinybindParams NoneParams) error",
		"type OneParams struct {\n\tUser User\n}",
		"func One(w io.Writer, _tinybindParams OneParams) error",
		"type ManyParams struct {\n\tUser  User\n\tTone  string\n\tCount int\n}",
		"func Many(w io.Writer, _tinybindParams ManyParams) error",
	}
	for _, fragment := range want {
		if !bytes.Contains(generated, []byte(fragment)) {
			t.Fatalf("generated Go missing %q:\n%s", fragment, generated)
		}
	}
	typeCheckGenerated(t, "shapes.go", generated, nil)
}

func TestGenerateComponentRendersToAnyWriter(t *testing.T) {
	source := []byte(`package pages
export component Page(title: string): html {<title>{title}</title>}`)
	generated, err := htmlbind.Generate("page.tb.html", source, htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	companion := []byte(`package pages

import (
	"io"
	"strings"
	"testing"
)

// render mirrors how a caller stores a component: the shape is the whole
// contract, with no HTTP type involved.
var render func(io.Writer, PageParams) error = Page

func TestRenderToBuilder(t *testing.T) {
	var out strings.Builder
	if err := render(&out, PageParams{Title: "a<b"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "<title>a&lt;b</title>" {
		t.Fatalf("rendered %q", out.String())
	}
}`)
	typeCheckGenerated(t, "page.go", generated, companion)
	runGeneratedTests(t, generated, companion)
}

func TestGenerateRejectsParamsTypeConflict(t *testing.T) {
	source := []byte(`package pages

type CardParams { name: string }

export component Card(value: string): html {<p>{value}</p>}`)
	_, err := htmlbind.Generate("conflict.tb.html", source, htmlbind.GenerateOptions{})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("CardParams")) {
		t.Fatalf("error = %v, want a CardParams conflict diagnostic", err)
	}
}
