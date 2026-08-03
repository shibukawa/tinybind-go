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
			// The filename reaches generated output through HeadSources, so it is
			// passed as a stable slash-joined label rather than as the on-disk path,
			// which would bake this checkout's layout into the golden.
			got, err := htmlbind.Generate(name+"/input.txt", input, htmlbind.GenerateOptions{})
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
		// An object shorthand and a single-statement block match an insertion
		// shape, so they reach analysis; the hint has to survive that far.
		{"shorthand reaching analysis", `component Bad(): html {<script>const o = {name};</script>}`, "inside <script> content"},
		{"tight call block reaching analysis", `component Bad(): html {<script>if(x){render()}</script>}`, "inside <script> content"},
		{"typed insertion keeps its hint", `component Bad(value: string): html {<script>{value}</script>}`, "insert a value with RawJavaScript or JsonForScript"},
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

// TestUpdateManifest covers the server half of partial updates: rendering one
// chain twice with different search parameters must identify the same
// instances and mark only the boundary whose markup actually changed.
func TestUpdateManifest(t *testing.T) {
	source := []byte(`package pages

export component Document(children: html): html {
<!doctype html>
<html>
<head><meta charset="utf-8" /></head>
<body><slot required /></body>
</html>
}

export component Layout(section: string, children: html): html {
<main class="layout"><h1>{section}</h1><slot required /></main>
}

export component Page(query: string, page: int): html {
<p>results for {query} on page {page}</p>
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

var key = []byte("test validator key")

func collect(t *testing.T, section, query string, page int) (htmlbind.Manifest, string) {
	t.Helper()
	var out bytes.Buffer
	wrappers := []htmlbind.Wrapper{
		BindDocument(DocumentParams{}),
		BindLayout(LayoutParams{Section: section}),
	}
	manifest, err := htmlbind.CollectChain(&out, key, wrappers, Page(PageParams{Query: query, Page: page}))
	if err != nil {
		t.Fatal(err)
	}
	return manifest, out.String()
}

// The document shell owns the head and is retained across partial navigation,
// so it is not an instance. The layout and the page are.
func TestManifestCoversChainMembersExceptShell(t *testing.T) {
	manifest, html := collect(t, "Docs", "go", 1)
	if len(manifest.Instances) != 2 {
		t.Fatalf("want 2 instances, got %d: %+v", len(manifest.Instances), manifest.Instances)
	}
	if _, ok := manifest.Find("c1"); !ok {
		t.Fatalf("layout instance missing: %+v", manifest.Instances)
	}
	if _, ok := manifest.Find("c2"); !ok {
		t.Fatalf("page instance missing: %+v", manifest.Instances)
	}
	for _, want := range []string{` + "`" + `<main data-tb-id="c1"` + "`" + `, ` + "`" + `<p data-tb-id="c2"` + "`" + `} {
		if !strings.Contains(html, want) {
			t.Fatalf("output %q is missing %q", html, want)
		}
	}
	if strings.Contains(html, ` + "`" + `<html data-tb-id` + "`" + `) {
		t.Fatalf("document shell must not be a boundary: %q", html)
	}
}

// The page's frame changes with its own parameters while the layout frame,
// which excludes its child's output, stays comparable.
func TestSearchParameterChangeMovesOnlyThePage(t *testing.T) {
	before, _ := collect(t, "Docs", "go", 1)
	after, _ := collect(t, "Docs", "go", 2)
	changed := after.Changed(before)
	if len(changed) != 1 || changed[0].ID != "c2" {
		t.Fatalf("want only the page changed, got %+v", changed)
	}
	layoutBefore, _ := before.Find("c1")
	layoutAfter, _ := after.Find("c1")
	if layoutBefore.FrameValidator != layoutAfter.FrameValidator {
		t.Fatal("layout frame must not change when only a page parameter changed")
	}
	if layoutBefore.InputValidator != layoutAfter.InputValidator {
		t.Fatal("layout input must not change when only a page parameter changed")
	}
	pageBefore, _ := before.Find("c2")
	pageAfter, _ := after.Find("c2")
	if pageBefore.InputValidator == pageAfter.InputValidator {
		t.Fatal("page input validator must change with its parameters")
	}
	if pageBefore.ComponentID != pageAfter.ComponentID {
		t.Fatal("component identity must survive a parameter change")
	}
}

// A layout parameter changes the layout frame without disturbing the page.
func TestLayoutParameterChangeMovesOnlyTheLayout(t *testing.T) {
	before, _ := collect(t, "Docs", "go", 1)
	after, _ := collect(t, "Guides", "go", 1)
	changed := after.Changed(before)
	if len(changed) != 1 || changed[0].ID != "c1" {
		t.Fatalf("want only the layout changed, got %+v", changed)
	}
}

// An unchanged render reports nothing, which is what lets a delta omit every
// boundary.
func TestIdenticalRenderReportsNoChange(t *testing.T) {
	before, _ := collect(t, "Docs", "go", 1)
	after, _ := collect(t, "Docs", "go", 1)
	if changed := after.Changed(before); len(changed) != 0 {
		t.Fatalf("want no change, got %+v", changed)
	}
}

// Parent tracking is what later lets a delta replace an ancestor, and document
// order is what lets a structural operation precede the operations it anchors.
func TestNestingIsRecorded(t *testing.T) {
	manifest, _ := collect(t, "Docs", "go", 1)
	if manifest.Instances[0].ID != "c1" || manifest.Instances[1].ID != "c2" {
		t.Fatalf("want document order, got %+v", manifest.Instances)
	}
	layout, _ := manifest.Find("c1")
	page, _ := manifest.Find("c2")
	if layout.ParentID != "" {
		t.Fatalf("outermost boundary must have no parent, got %q", layout.ParentID)
	}
	if page.ParentID != "c1" {
		t.Fatalf("page parent must be the layout, got %q", page.ParentID)
	}
}

// An ordinary render must be unaffected by update support, including the
// instance attributes, so existing templates keep their exact bytes.
func TestOrdinaryRenderEmitsNoUpdateMarkup(t *testing.T) {
	var out bytes.Buffer
	wrappers := []htmlbind.Wrapper{
		BindDocument(DocumentParams{}),
		BindLayout(LayoutParams{Section: "Docs"}),
	}
	if err := htmlbind.RenderChain(&out, wrappers, Page(PageParams{Query: "go", Page: 1})); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "data-tb-") {
		t.Fatalf("ordinary render leaked update markup: %q", out.String())
	}
}
`)
	runGeneratedTests(t, generated, companion)
}

// TestBoundaryEligibility pins which components become update boundaries.
// Boundaries are automatic here, so a component that cannot carry an instance
// attribute is silently excluded rather than failing generation; the error form
// belongs with the explicit update flag.
func TestBoundaryEligibility(t *testing.T) {
	tests := []struct {
		name, source string
		want         bool
	}{
		{"single root", `export component A(): html {<p>x</p>}`, true},
		{"root with leading doctype", `export component A(): html {<!doctype html><p>x</p>}`, true},
		{"root with surrounding whitespace", "export component A(): html {\n  <p>x</p>\n}", true},
		{"sibling roots", `export component A(): html {<p>x</p><p>y</p>}`, false},
		{"text beside root", `export component A(): html {lead<p>x</p>}`, false},
		{"conditional root", `export component A(flag: bool): html {{if flag}<p>x</p>{/if}}`, false},
		{"loop root", `export component A(items: string[]): html {{for item in items}<li>{item}</li>{/for}}`, false},
		{"component root", `component Inner(): html {<p>x</p>} export component A(): html {<Inner />}`, false},
		{"unexported", `component A(): html {<p>x</p>}`, false},
		{"document shell", `export component A(children: html): html {<html><head></head><body><slot required /></body></html>}`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			generated, err := htmlbind.Generate("boundary.pw.html", []byte("package pages\n"+test.source), htmlbind.GenerateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			got := bytes.Contains(generated, []byte("htmlbind.Boundary["))
			if got != test.want {
				t.Fatalf("boundary emitted = %v, want %v\n%s", got, test.want, generated)
			}
			if attr := bytes.Contains(generated, []byte("BoundaryAttr()")); attr != test.want {
				t.Fatalf("instance attribute emitted = %v, want %v\n%s", attr, test.want, generated)
			}
		})
	}
}

// TestDataAttributePrefix covers the configurable namespace. The prefix is
// baked into generated code rather than negotiated, because the browser runtime
// hardcodes it.
func TestDataAttributePrefix(t *testing.T) {
	source := []byte("package pages\nexport component A(): html {<p>x</p>}")
	generated, err := htmlbind.Generate("prefix.pw.html", source, htmlbind.GenerateOptions{DataAttributePrefix: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte(`Attr:        "data-app-id"`)) {
		t.Fatalf("configured prefix missing:\n%s", generated)
	}
	// An empty option selects the default, matching how the asset options
	// behave; only a value that cannot form an attribute name is an error.
	if _, err := htmlbind.Generate("prefix.pw.html", source, htmlbind.GenerateOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"App", "tb_", "-tb", "tb-"} {
		if _, err := htmlbind.Generate("prefix.pw.html", source, htmlbind.GenerateOptions{DataAttributePrefix: invalid}); err == nil {
			t.Fatalf("prefix %q was accepted", invalid)
		}
	}
}

// TestComponentKindNamesRatherThanVersions pins the split between identity and
// version. The kind names a component so its endpoint URL survives an unrelated
// edit; detecting that the page is stale is the build identity's job, and it
// covers changes a per-component digest cannot see anyway.
func TestComponentKindNamesRatherThanVersions(t *testing.T) {
	kind := func(file, source string) []byte {
		generated, err := htmlbind.Generate(file, []byte(source), htmlbind.GenerateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		start := bytes.Index(generated, []byte("const AKind = "))
		if start < 0 {
			t.Fatalf("no kind in:\n%s", generated)
		}
		rest := generated[start:]
		return rest[:bytes.IndexByte(rest, '\n')]
	}
	base := kind("pages/card.tb.html", "package pages\n@reloadable\nexport component A(id: string, n: int): html {<p>{n}</p>}")
	if !bytes.Contains(base, []byte(`"pages.card.A"`)) {
		t.Fatalf("want a readable package.file.name kind, got %s", base)
	}
	// A template edit must not move the endpoint.
	if edited := kind("pages/card.tb.html", "package pages\n@reloadable\nexport component A(id: string, n: int): html {<div>{n}</div>}"); !bytes.Equal(base, edited) {
		t.Fatalf("an edit changed the kind: %s vs %s", base, edited)
	}
	// The file and the package are what make it unique.
	if other := kind("pages/badge.tb.html", "package pages\n@reloadable\nexport component A(id: string, n: int): html {<p>{n}</p>}"); bytes.Equal(base, other) {
		t.Fatal("two files must not share a kind")
	}
	if other := kind("admin/card.tb.html", "package admin\n@reloadable\nexport component A(id: string, n: int): html {<p>{n}</p>}"); bytes.Equal(base, other) {
		t.Fatal("two packages must not share a kind")
	}
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
	result, err := htmlbind.GenerateModule("sfc.pw.html", source, htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	generated := result.GoSource
	if len(result.Assets) != 1 || result.Assets[0].Kind != htmlbind.AssetStyle {
		t.Fatalf("want one extracted stylesheet, got %v", result.Assets)
	}
	if !strings.Contains(string(result.Assets[0].Content), "@keyframes fade_") {
		t.Fatalf("scoped keyframes did not reach the stylesheet:\n%s", result.Assets[0].Content)
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
	for _, want := range []string{"<meta charset=\"utf-8\" />", "<link rel=\"stylesheet\" href=\"/shared.css\">", "<link rel=\"stylesheet\" href=\"/public/generated/sfc.style."} {
		if !strings.Contains(head, want) {
			t.Fatalf("head %q does not contain %q", head, want)
		}
	}
	if strings.Contains(head, "<style") {
		t.Fatalf("style block was not extracted: %q", head)
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

// TestReloadableComponent covers the whole redraw path from template syntax to
// a served endpoint: the modifier, the generated typed decoder, the id and kind
// on the root element, and registration.
func TestReloadableComponent(t *testing.T) {
	source := []byte(`package pages

@reloadable
export component Counter(id: string, page: int, label: string?): html {
<span class="counter">page {page}</span>
}
`)
	generated, err := htmlbind.Generate("counter.pw.html", source, htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	companion := []byte(`package pages

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlupdate"
)

func serve(t *testing.T, query url.Values) *httptest.ResponseRecorder {
	t.Helper()
	options := htmlupdate.Options{Key: []byte("k")}
	registry := &htmlupdate.Registry{}
	registry.Register(CounterReloadable)
	recorder := httptest.NewRecorder()
	// A redraw is addressed at whatever URL the caller serves it from, with the
	// component in headers. Here that is the page the region sits on.
	path := "/dashboard"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("X-Tinybind-Render", "redraw")
	request.Header.Set("X-Tinybind-Kind", CounterKind)
	request.Header.Set("X-Tinybind-Instance", "counter-1")
	// A real page carries the build it was rendered by, from its script tag.
	request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
	options.Redraw(recorder, request, registry)
	return recorder
}

// The rendered root carries both the author's id and the kind, so the region
// stays addressable and redrawable after the first redraw replaced it.
func TestRedrawRendersTheRegisteredComponent(t *testing.T) {
	recorder := serve(t, url.Values{"page": {"7"}, "label": {"item"}})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}
	body := recorder.Body.String()
	for _, want := range []string{` + "`" + `id="counter-1"` + "`" + `, ` + "`" + `data-tb-kind="` + "`" + `, "page 7"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %q is missing %q", body, want)
		}
	}
}

// An optional parameter may be absent; a present but undecodable one is still
// an error, because these arguments come from the caller.
func TestRedrawDecodesTypedParameters(t *testing.T) {
	if code := serve(t, url.Values{"page": {"7"}}).Code; code != http.StatusOK {
		t.Fatalf("an absent optional should be fine, got %d", code)
	}
	for name, query := range map[string]url.Values{
		"not an integer": {"page": {"seven"}},
		"missing":        {"label": {"x"}},
		"repeated":       {"page": {"1", "2"}},
	} {
		if code := serve(t, query).Code; code != http.StatusBadRequest {
			t.Fatalf("%s gave %d, want 400", name, code)
		}
	}
}

// The kind names the component rather than versioning it: package, file, and
// declaration, readable in a URL and in a log line.
func TestKindIsTheComponentIdentity(t *testing.T) {
	if CounterKind != "pages.counter.Counter" {
		t.Fatalf("kind = %q", CounterKind)
	}
	if CounterReloadable.KindID != CounterKind {
		t.Fatal("the registration and the markup must agree on the kind")
	}
}
`)
	runGeneratedTests(t, generated, companion)
}

// TestReloadableDiagnostics covers the rules an explicit opt-in must satisfy.
// Unlike an automatic boundary these are errors, because the author asked.
func TestReloadableDiagnostics(t *testing.T) {
	tests := []struct{ name, source, want string }{
		{"not exported", `@reloadable
component A(id: string): html {<p>x</p>}`, "must be exported"},
		{"no id", `@reloadable
export component A(): html {<p>x</p>}`, "must declare an id parameter"},
		{"optional id", `@reloadable
export component A(id: string?): html {<p>x</p>}`, "required string"},
		{"two roots", `@reloadable
export component A(id: string): html {<p>x</p><p>y</p>}`, "exactly one root element"},
		{"undecodable parameter", `type R { n: int } @reloadable
export component A(id: string, r: R): html {<p>x</p>}`, "query string"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := htmlbind.Generate("bad.pw.html", []byte("package pages\n"+test.source), htmlbind.GenerateOptions{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
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
		{
			"reading an async parameter outside an await clause",
			`component Bad(name: async string): html {<p>{name}</p>}`,
			"must be bound by an await clause before it is read",
		},
		{
			"reading a field of an async record",
			`type User { name: string }
component Bad(user: async User): html {{await v = user.name}<p>{v}</p>{fallback}p{/await}}`,
			"must be bound by an await clause before it is read",
		},
		{
			"comparing an async value",
			`component Bad(count: async int): html {{if count == 1}<p>x</p>{/if}}`,
			"must be bound by an await clause before it is read",
		},
		{
			"awaiting a value that is not async",
			`component Bad(name: string): html {{await v = name}<p>{v}</p>{fallback}p{/await}}`,
			"only an async value or an async external call can be awaited",
		},
		{
			"async modifying another async",
			`component Bad(name: async async string): html {{await v = name}<p>{v}</p>{fallback}p{/await}}`,
			"async cannot modify another async",
		},
		{
			"array of async values",
			`component Bad(names: [async string]): html {<p>x</p>}`,
			"async applies to the whole type",
		},
		{
			"async external parameter",
			`external Load(value: async string): string
component Bad(): html {<p>{Load("x")}</p>}`,
			"cannot be async; declare the function external async instead",
		},
		{
			"async external result",
			`external Load(): async string
component Bad(): html {<p>x</p>}`,
			"cannot return an async type",
		},
		{
			"async slot parameter",
			`component Bad(children: async html): html {<p><slot required /></p>}`,
			"html parameter children cannot be async",
		},
		{
			"cached component with an async parameter",
			`@cache(ttl: "5m")
component Bad(name: async string): html {{await v = name}<p>{v}</p>{fallback}p{/await}}`,
			"cannot declare the async parameter name",
		},
		{
			"cached component with a record reaching an async field",
			`type User { pending: async string }
@cache(ttl: "5m")
component Bad(user: User): html {<p>x</p>}`,
			"cannot declare the async parameter user",
		},
		{
			"serializing an async value into a script",
			`type User { pending: async string }
component Bad(user: User): html {<script type="application/json">{JsonForScript(user)}</script>}`,
			"not statically serializable",
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

// TestDocumentedRawTextExamples compiles the examples in the "Braces inside
// <script> and <style>" section of docs/htmlbind.md, so the documented rules and
// rule:raw-text-insertion-gate cannot drift apart.
func TestDocumentedRawTextExamples(t *testing.T) {
	t.Run("authored content survives byte for byte", func(t *testing.T) {
		source := "export component Widget(): html {\n<script>\n" +
			"class X {}\n" +
			"function f() {\n  return 1\n}\n" +
			"function g(){return 1}\n" +
			"const o = { a: 1 };\n" +
			"const n = {0: 'a'};\n" +
			"const p = {a, b};\n" +
			"class C { m() { this.v = 1; } }\n" +
			"if (x) { render() }\n" +
			"const s = `hi ${name}`;\n" +
			"</script>\n<style>\n" +
			".a { color: red; }\n" +
			".b{color:red}\n" +
			"@media print {\n  .c { color: #000; }\n}\n" +
			"</style>\n" +
			"<script type=\"speculationrules\">\n{\"prerender\": [{\"where\": {\"href_matches\": \"/*\"}}]}\n</script>\n}\n"
		generated, err := htmlbind.Generate("doc.txt", []byte(source), htmlbind.GenerateOptions{Package: "doc"})
		if err != nil {
			t.Fatalf("documented content example failed: %v", err)
		}
		for _, want := range []string{
			`class X {}`, `function g(){return 1}`, `const o = { a: 1 };`, `const n = {0: 'a'};`,
			`const p = {a, b};`, `class C { m() { this.v = 1; } }`, `if (x) { render() }`,
			"`hi ${name}`", `.a { color: red; }`, `.b{color:red}`, `.c { color: #000; }`,
			`{\"prerender\": [{\"where\": {\"href_matches\": \"/*\"}}]}`,
		} {
			if !bytes.Contains(generated, []byte(want)) {
				t.Errorf("generated output missing authored line %q", want)
			}
		}
	})

	t.Run("every documented insertion shape compiles", func(t *testing.T) {
		source := `type Payload { id: int }
type Config { js: trusted_javascript }

export component Widget(
  js: trusted_javascript,
  cfg: Config,
  css: string,
  payload: Payload,
  ready: bool,
  on: trusted_javascript,
  off: trusted_javascript
): html {
<script>{js}</script>
<script>{cfg.js}</script>
<script>{JsonForScript(payload)}</script>
<script>{(ready ? on : off)}</script>
<style>{RawCSS(css)}</style>
<script>{if ready}console.log(1){/if}</script>
}
`
		if _, err := htmlbind.Generate("doc.txt", []byte(source), htmlbind.GenerateOptions{Package: "doc"}); err != nil {
			t.Fatalf("documented insertion example failed: %v", err)
		}
	})

	t.Run("the escape resolves a collision", func(t *testing.T) {
		source := "export component W(): html {\n<script>const o = {{name}};</script>\n}\n"
		generated, err := htmlbind.Generate("doc.txt", []byte(source), htmlbind.GenerateOptions{Package: "doc"})
		if err != nil {
			t.Fatalf("documented escape example failed: %v", err)
		}
		if !bytes.Contains(generated, []byte(`const o = {name};`)) {
			t.Fatal("escape did not emit a literal brace pair")
		}
	})

	t.Run("a bare script_json value is already encoded", func(t *testing.T) {
		// The documented residual: a tight shorthand naming an insertable
		// parameter compiles. It must compile rather than crash, because the
		// emitter only unwraps a direct JsonForScript call.
		source := "export component W(payload: script_json): html {\n<script>const o = {payload};</script>\n}\n"
		if _, err := htmlbind.Generate("doc.txt", []byte(source), htmlbind.GenerateOptions{Package: "doc"}); err != nil {
			t.Fatalf("bare script_json insertion failed: %v", err)
		}
		spaced := "export component W(payload: script_json): html {\n<script>const o = { payload };</script>\n}\n"
		generated, err := htmlbind.Generate("doc.txt", []byte(spaced), htmlbind.GenerateOptions{Package: "doc"})
		if err != nil {
			t.Fatalf("spaced form failed: %v", err)
		}
		if !bytes.Contains(generated, []byte(`const o = { payload };`)) {
			t.Fatal("spaced form was not kept as authored content")
		}
	})
}
