package htmlbind_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

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
	// The package name is mangled; the parameter becomes an exported struct
	// field, which can never collide with a keyword.
	if !bytes.Contains(generated, []byte("package _type")) || !bytes.Contains(generated, []byte("Type string")) {
		t.Fatalf("generated Go does not mangle keywords:\n%s", generated)
	}
}

func TestGenerateDiagnosticIncludesPosition(t *testing.T) {
	source := []byte("component Bad(): html {\n<p>\n{missing}\n</p>\n}")
	_, err := htmlbind.Generate("position.txt", source, htmlbind.GenerateOptions{})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("position.txt:3:2:")) {
		t.Fatalf("error = %v, want filename:line:col", err)
	}
}

func TestSlotDiagnostics(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "undeclared parameter",
			source: "component A(): html {\n<slot name=\"head\" />\n}\n",
			want:   "slot head has no matching parameter",
		},
		{
			name:   "required marker disagrees with type",
			source: "component A(children: html?): html {\n<slot required />\n}\n",
			want:   "required slot children must be declared html, not html?",
		},
		{
			name:   "missing required marker",
			source: "component A(children: html): html {\n<slot />\n}\n",
			want:   "add the required attribute",
		},
		{
			name:   "default content on a required slot",
			source: "component A(children: html): html {\n<slot required>x</slot>\n}\n",
			want:   "cannot declare default content",
		},
		{
			name:   "slot inside a for body",
			source: "component A(items: string[], children: html): html {\n{for item, index in items}<slot required />{/for}\n}\n",
			want:   "cannot appear inside a for body",
		},
		{
			name:   "slot rendered twice on one path",
			source: "component A(children: html): html {\n<slot required /><slot required />\n}\n",
			want:   "rendered more than once on the same path",
		},
		{
			name:   "slot element used as a value",
			source: "component A(label: string): html {\n<slot name=\"label\" />\n}\n",
			want:   "must bind an html parameter",
		},
		{
			name:   "fill names an unknown slot",
			source: "component A(children: html): html {\n<slot required />\n}\nexport component B(): html {\n<A><template name=\"other\">x</template><p>y</p></A>\n}\n",
			want:   "has no slot named other",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := htmlbind.Generate("slots.pw.html", []byte("package pages\n\n"+testCase.source), htmlbind.GenerateOptions{})
			if err == nil {
				t.Fatal("expected a compile error")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), testCase.want)
			}
		})
	}
}

func TestSlotInBothIfBranchesIsAllowed(t *testing.T) {
	source := []byte(`package pages

component A(wide: bool, children: html): html {
{if wide}<div class="wide"><slot required /></div>{else}<span><slot required /></span>{/if}
}
`)
	if _, err := htmlbind.Generate("branches.pw.html", source, htmlbind.GenerateOptions{}); err != nil {
		t.Fatal(err)
	}
}

// TestChainComposition renders a document, a layout, and a page as one
// chain, which is the composition a handler assembles from several template

// TestChainComposition renders a document, a layout, and a page as one chain,
// which is the composition a handler assembles from several template files.
func TestChainComposition(t *testing.T) {
	source := []byte(`package pages

export component Document(title: string, children: html): html {
<!doctype html>
<html>
<head><title>{title}</title></head>
<body><slot required /></body>
</html>
}

export component Layout(children: html): html {
<main class="layout"><slot required /></main>
}

export component Page(body: string): html {
<p>{body}</p>
}
`)
	generated, err := htmlbind.Generate("chain.pw.html", source, htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	companion := []byte(`package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

func TestRenderChain(t *testing.T) {
	var out bytes.Buffer
	wrappers := []htmlbind.Wrapper{
		BindDocument(DocumentParams{Title: "Docs"}),
		BindLayout(LayoutParams{}),
	}
	if err := htmlbind.RenderChain(&out, wrappers, Page(PageParams{Body: "hello"})); err != nil {
		t.Fatal(err)
	}
	order := []string{"<!doctype html>", "<title>Docs</title>", "<body>", "<main class=\"layout\">", "<p>hello</p>", "</main>", "</body>"}
	rest := out.String()
	for _, fragment := range order {
		index := strings.Index(rest, fragment)
		if index < 0 {
			t.Fatalf("output %q is missing %q in order", out.String(), fragment)
		}
		rest = rest[index+len(fragment):]
	}
}

func TestRenderChainWithoutWrappers(t *testing.T) {
	var out bytes.Buffer
	if err := htmlbind.Render(&out, Page(PageParams{Body: "solo"})); err != nil {
		t.Fatal(err)
	}
	if out.String() != " <p>solo</p> " {
		t.Fatalf("unexpected leaf-only output %q", out.String())
	}
}

func TestRenderChainRejectsMissingLeaf(t *testing.T) {
	var out bytes.Buffer
	wrappers := []htmlbind.Wrapper{BindLayout(LayoutParams{})}
	if err := htmlbind.RenderChain(&out, wrappers, htmlbind.Fragment{}); err != htmlbind.ErrNoLeaf {
		t.Fatalf("want ErrNoLeaf, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("invalid chain wrote %q before failing", out.String())
	}
}
`)
	runGeneratedTests(t, generated, companion)
}

// TestScopedStyleAndHeadMerging covers the single-file-component behaviour:
// styles live next to the markup, class names are scoped per component, and
// every reachable component's head contributions land in the document shell.
func TestScopedStyleAndHeadMerging(t *testing.T) {
	source := []byte(`package pages

export component Document(children: html): html {
<!doctype html>
<html>
<head><meta charset="utf-8" /></head>
<body><slot required /></body>
</html>
}

export component Card(label: string): html {
<head>
<link rel="stylesheet" href="/shared.css" />
<style>
.box { color: red; animation: fade 1s }
.box .label { font-weight: bold }
@keyframes fade { from { opacity: 0 } }
</style>
</head>
<div class="box shadow"><span class="label">{label}</span></div>
}
`)
	generated, err := htmlbind.Generate("sfc.pw.html", source, htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	companion := []byte(`package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

func TestScopedStyleReachesTheDocumentHead(t *testing.T) {
	var out bytes.Buffer
	wrappers := []htmlbind.Wrapper{BindDocument(DocumentParams{})}
	if err := htmlbind.RenderChain(&out, wrappers, Card(CardParams{Label: "A&B"})); err != nil {
		t.Fatal(err)
	}
	body := out.String()

	head, rest, ok := strings.Cut(body, "</head>")
	if !ok {
		t.Fatalf("no head in %q", body)
	}
	for _, want := range []string{"<meta charset=\"utf-8\" />", "<link rel=\"stylesheet\" href=\"/shared.css\">", "@keyframes fade_"} {
		if !strings.Contains(head, want) {
			t.Fatalf("head %q does not contain %q", head, want)
		}
	}
	if strings.Contains(rest, "<style") || strings.Contains(rest, "<link") {
		t.Fatalf("head contribution leaked into the body: %q", rest)
	}
	if !strings.Contains(rest, "shadow") {
		t.Fatalf("undeclared class was rewritten: %q", rest)
	}
	if strings.Contains(rest, "\"box shadow\"") {
		t.Fatalf("declared class was not scoped: %q", rest)
	}
	if !strings.Contains(rest, "A&amp;B") {
		t.Fatalf("escaping regressed: %q", rest)
	}
	if strings.Contains(head, "animation: fade 1s") {
		t.Fatalf("keyframes reference was not rewritten: %q", head)
	}
}

func TestShellRendersWithoutAChain(t *testing.T) {
	var out bytes.Buffer
	if err := htmlbind.Render(&out, Document(DocumentParams{Children: Card(CardParams{Label: "x"})})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "<meta charset=\"utf-8\" />") {
		t.Fatalf("shell lost its own head content: %q", out.String())
	}
}
`)
	runGeneratedTests(t, generated, companion)
}

// TestAsyncAndCacheDiagnostics covers the rules that keep an await boundary and
// a cached component from producing output the runtime cannot stand behind.
func TestAsyncAndCacheDiagnostics(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{
			"async call outside await",
			`external async Load(): string
component Bad(): html {<p>{Load()}</p>}`,
			"can only be called in an await binding",
		},
		{
			"nested async call in a binding",
			`external async Load(value: string): string
component Bad(): html {{await v = Load(Load("x"))}<p>{v}</p>{fallback}p{/await}}`,
			"can only be called in an await binding",
		},
		{
			"awaiting a synchronous external",
			`external Load(): string
component Bad(): html {{await v = Load()}<p>{v}</p>{fallback}p{/await}}`,
			"is not async; declare it as external async",
		},
		{
			"missing fallback clause",
			`external async Load(): string
component Bad(): html {{await v = Load()}<p>{v}</p>{/await}}`,
			"expected {fallback} inside {await}",
		},
		{
			"slot inside an await clause",
			`external async Load(): string
component Bad(children: html): html {{await v = Load()}<slot required />{fallback}p{/await}}`,
			"cannot appear inside an await block",
		},
		{
			"binding shadows the generated scope field",
			`external async Load(): string
component Bad(): html {{await outer = Load()}<p>{outer}</p>{fallback}p{/await}}`,
			"cannot be named outer",
		},
		{
			"error field that does not exist",
			`external async Load(): string
component Bad(): html {{await v = Load()}<p>{v}</p>{fallback}p{recover err}{err.detail}{/await}}`,
			"unknown field detail on error",
		},
		{
			"unknown annotation",
			`@memo(ttl: "5m")
component Bad(): html {<p>x</p>}`,
			"unknown annotation @memo",
		},
		{
			"cache without a ttl",
			`@cache()
component Bad(): html {<p>x</p>}`,
			"@cache requires a ttl argument",
		},
		{
			"cache with an unparsable ttl",
			`@cache(ttl: "soon")
component Bad(): html {<p>x</p>}`,
			"@cache ttl is not a duration",
		},
		{
			"cache with a slot parameter",
			`@cache(ttl: "5m")
component Bad(children: html): html {<p><slot required /></p>}`,
			"cannot declare the html parameter children",
		},
		{
			"cached component owning an await boundary",
			`external async Load(): string
@cache(ttl: "5m")
component Bad(): html {{await v = Load()}<p>{v}</p>{fallback}p{/await}}`,
			"cannot reach an await boundary",
		},
		{
			"cached component reaching an await boundary through a call",
			`external async Load(): string
component Inner(): html {{await v = Load()}<p>{v}</p>{fallback}p{/await}}
@cache(ttl: "5m")
component Bad(): html {<Inner />}`,
			"cannot reach an await boundary; Inner declares one",
		},
		{
			"annotation on a type declaration",
			`@cache(ttl: "5m")
type Bad { name: string }`,
			"annotation cannot precede a type declaration",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := htmlbind.Generate("invalid.txt", []byte(test.source), htmlbind.GenerateOptions{Package: "invalid"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

// TestAsyncExternalContextArgument covers the two shapes an async external's Go
// implementation may take. The template declaration is the same either way; the
// caller reports which functions accept a leading context.
func TestAsyncExternalContextArgument(t *testing.T) {
	source := []byte(`external async LoadUser(id: string): User
external async LoadTags(id: string): string[]
type User { name: string }
component Page(id: string): html {{await user = LoadUser(id), tags = LoadTags(id)}<p>{user.name}</p>{fallback}p{/await}}`)

	plain, err := htmlbind.Generate("page.txt", source, htmlbind.GenerateOptions{Package: "pages"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(plain, []byte("LoadUser(p.Id)")) || !bytes.Contains(plain, []byte("LoadTags(p.Id)")) {
		t.Fatalf("externals were not called as plain functions:\n%s", plain)
	}

	mixed, err := htmlbind.Generate("page.txt", source, htmlbind.GenerateOptions{
		Package:          "pages",
		ContextExternals: map[string]bool{"LoadTags": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(mixed, []byte("LoadTags(ctx, p.Id)")) {
		t.Fatalf("context-taking external did not receive ctx:\n%s", mixed)
	}
	if !bytes.Contains(mixed, []byte("LoadUser(p.Id)")) {
		t.Fatalf("plain external gained a ctx argument:\n%s", mixed)
	}
}
