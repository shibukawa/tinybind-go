package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// The block is written where a single-file component author expects it: beside
// the head block, before the markup. Its body is authored JavaScript, so the
// braces and the comparison below must survive verbatim.
const scriptBlockSource = `package pages

export component Counter(label: string): html {
<head>
<style>.counter { color: red }</style>
</head>
<script component>
export function setup(el) {
  const state = { count: 0 };
  for (let i = 0; i < 3; i++) { state.count += i }
  el.textContent = ` + "`${state.count}`" + `;
  return () => { state.count = 0 };
}
</script>
<div class="counter">{label}</div>
}
`

func TestScriptBlockExtractsAndNamesItsOwner(t *testing.T) {
	result, err := htmlbind.GenerateModule("counter.tb.html", []byte(scriptBlockSource), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var script *htmlbind.Asset
	for index, asset := range result.Assets {
		if asset.Kind == htmlbind.AssetScript {
			script = &result.Assets[index]
		}
	}
	if script == nil {
		t.Fatalf("no script asset produced: %+v", result.Assets)
	}
	// The package-qualified identity, not the declared name: two components named
	// Counter in two directories are one name and two declarations, and a caller
	// keying a lifecycle on the short form would run one against the other.
	if script.Owner != "pages.counter.Counter" {
		t.Fatalf("script owner = %q, want pages.counter.Counter", script.Owner)
	}
	// The whole point of the block: a brace, a less-than, and a template
	// literal are the authored language rather than template punctuation.
	for _, want := range []string{
		"const state = { count: 0 };",
		"for (let i = 0; i < 3; i++)",
		"${state.count}",
		"return () => { state.count = 0 };",
	} {
		if !strings.Contains(string(script.Content), want) {
			t.Fatalf("script body lost %q:\n%s", want, script.Content)
		}
	}

	generated := string(result.GoSource)
	// A lifecycle method is an export, so the reference is a module.
	if want := `<script src=\"` + script.URL + `\" type=\"module\"></script>`; !strings.Contains(generated, want) {
		t.Fatalf("generated head lacks %s:\n%s", want, generated)
	}
	// The block is a declaration, not markup: nothing of it is rendered.
	if strings.Contains(generated, "state.count") {
		t.Fatalf("script block leaked into rendered output:\n%s", generated)
	}
	// The owner reaches the runtime value a caller reads.
	if want := `Scope: "pages.counter.Counter"`; !strings.Contains(generated, want) {
		t.Fatalf("generated assets lack %s:\n%s", want, generated)
	}
	// And the same identity marks the elements, so a caller holding the asset can
	// find the instances without a mapping and without a manifest.
	if want := `data-tb-component=\"pages.counter.Counter\"`; !strings.Contains(generated, want) {
		t.Fatalf("generated markup lacks the declaration marker:\n%s", generated)
	}
}

// A head contribution keeps meaning document lifetime, which is what an empty
// Scope says. Without this the feature would silently reclassify every script
// that already ships.
func TestHeadScriptCarriesNoOwner(t *testing.T) {
	const source = `package pages

export component Widget(label: string): html {
<head>
<script type="module">console.log("widget")</script>
</head>
<div>{label}</div>
}
`
	result, err := htmlbind.GenerateModule("widget.tb.html", []byte(source), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assets) != 1 {
		t.Fatalf("want one script asset, got %+v", result.Assets)
	}
	if result.Assets[0].Owner != "" {
		t.Fatalf("head script owner = %q, want empty", result.Assets[0].Owner)
	}
	if strings.Contains(string(result.GoSource), "Scope:") {
		t.Fatalf("head script emitted a Scope:\n%s", result.GoSource)
	}
}

// A script in markup carrying a template insertion is a shipped feature, and it
// is the reason the block needs a marker at all: position alone cannot tell the
// two apart, because both sit at the top of a component body.
func TestMarkupScriptWithInsertionStillCompiles(t *testing.T) {
	const source = `package pages

export component Document(javascript: string): html {
<script>{RawJavaScript(javascript)}</script>
}
`
	result, err := htmlbind.GenerateModule("doc.tb.html", []byte(source), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assets) != 0 {
		t.Fatalf("markup script must not be extracted, got %+v", result.Assets)
	}
	// The element is emitted in place, and its insertion stays an insertion.
	// The static run stops at the tag name because the boundary attribute is
	// written between it and the closing bracket.
	for _, want := range []string{"<script", "p.Javascript"} {
		if !strings.Contains(string(result.GoSource), want) {
			t.Fatalf("markup script lost %q:\n%s", want, result.GoSource)
		}
	}
}

// An absolute specifier is how two blocks share code: the browser's module map
// evaluates one URL once, which is what makes bundling unnecessary rather than
// merely optional.
func TestScriptBlockAcceptsAbsoluteImport(t *testing.T) {
	const source = `package pages

export component Counter(): html {
<script component>
import { format } from "/public/util.js";
// A comment mentioning "./util.js" is not an import.
export function setup(el) { el.textContent = format(1) }
</script>
<div></div>
}
`
	result, err := htmlbind.GenerateModule("counter.tb.html", []byte(source), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Assets[0].Content), `from "/public/util.js"`) {
		t.Fatalf("import was rewritten:\n%s", result.Assets[0].Content)
	}
}

// The case the marker exists for. An ordinary component call opens no update
// boundary, so it carries no instance attribute and enters no manifest — and a
// component rendered many times inside a page is exactly that. Before the
// marker there was nothing on those elements at all, so a caller holding an
// asset scoped to Row could not find a single Row in the document.
//
// The marker is not a boundary and does not make one: it says which declaration
// an element came from, not which instance it is. Telling two Rows apart is a
// separate question.
func TestScriptBlockMarksAnOrdinaryComponentCall(t *testing.T) {
	const source = `package pages

export component Row(text: string): html {
<script component>export function setup(el) { return () => {} }</script>
<li>{text}</li>
}

export component List(rows: string[]): html {
<ul>{for row in rows}<Row text={row}/>{/for}</ul>
}
`
	result, err := htmlbind.GenerateModule("rows.tb.html", []byte(source), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	generated := string(result.GoSource)

	if want := `data-tb-component=\"pages.rows.Row\"`; !strings.Contains(generated, want) {
		t.Fatalf("Row carries no declaration marker:\n%s", generated)
	}
	// It rides the static markup, so it costs no instruction and no render-time
	// work no matter how many rows there are.
	if strings.Contains(generated, `Attr("data-tb-component"`) {
		t.Errorf("the marker was emitted as an instruction rather than static markup:\n%s", generated)
	}
	// List declares no block, so it is marked by nothing.
	if strings.Count(generated, "data-tb-component") != 1 {
		t.Errorf("marker count = %d, want exactly one:\n%s", strings.Count(generated, "data-tb-component"), generated)
	}
}

func TestScriptBlockDiagnostics(t *testing.T) {
	for _, testcase := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "two blocks",
			source: `package pages

export component Counter(): html {
<script component>export function setup() {}</script>
<script component>export function other() {}</script>
<div></div>
}
`,
			want: "more than one script block",
		},
		{
			name: "block inside markup",
			source: `package pages

export component Counter(): html {
<div><script component>export function setup() {}</script></div>
}
`,
			want: "declares a script block inside markup",
		},
		{
			name: "global block",
			source: `package pages

export component Counter(): html {
<script component global>console.log("hi")</script>
<div></div>
}
`,
			want: "a component script block is a module",
		},
		{
			name: "relative import",
			source: `package pages

export component Counter(): html {
<script component>
import { format } from "./util.js";
export function setup(el) { el.textContent = format(1) }
</script>
<div></div>
}
`,
			want: "resolves against the generated file's URL",
		},
		{
			name: "block inside a control block",
			source: `package pages

export component Counter(on: bool): html {
<div>{if on}<script component>export function setup() {}</script>{/if}</div>
}
`,
			want: "declares a script block inside markup",
		},
		{
			// The marker naming the declaration has to live somewhere, and two
			// roots give it no single element to live on. It is the rule a
			// reloadable component already follows, for the same reason.
			name: "two root elements",
			source: `package pages

export component Counter(): html {
<script component>export function setup() {}</script>
<div></div>
<span></span>
}
`,
			want: "must render exactly one root element",
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			_, err := htmlbind.GenerateModule("counter.tb.html", []byte(testcase.source), htmlbind.GenerateOptions{})
			if err == nil {
				t.Fatalf("want an error naming %q, got none", testcase.want)
			}
			if !strings.Contains(err.Error(), testcase.want) {
				t.Fatalf("error = %v, want it to name %q", err, testcase.want)
			}
		})
	}
}
