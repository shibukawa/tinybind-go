package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

const assetSource = `package pages

export component Widget(label: string): html {
<head>
<script defer type="module">console.log("widget")</script>
<script src="https://cdn.example.com/lib.js" async></script>
<style>.widget { color: blue }</style>
</head>
<div class="widget">{label}</div>
}
`

// TestExtractionProducesOneFilePerKind covers the acceptance case of a
// component shipping both a script and a style, and the passthrough case of a
// script already naming an external URL.
func TestExtractionProducesOneFilePerKind(t *testing.T) {
	result, err := htmlbind.GenerateModule("widget.tb.html", []byte(assetSource), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assets) != 2 {
		t.Fatalf("want a stylesheet and a script, got %d assets: %+v", len(result.Assets), result.Assets)
	}
	style, script := result.Assets[0], result.Assets[1]
	if style.Kind != htmlbind.AssetStyle || style.Extension != "css" {
		t.Fatalf("first asset = %+v, want a stylesheet", style)
	}
	if script.Kind != htmlbind.AssetScript || script.Extension != "js" {
		t.Fatalf("second asset = %+v, want a script", script)
	}
	if !strings.Contains(string(style.Content), "color: blue") {
		t.Fatalf("stylesheet lost its rules:\n%s", style.Content)
	}
	if !strings.Contains(string(script.Content), `console.log("widget")`) {
		t.Fatalf("script lost its body:\n%s", script.Content)
	}

	generated := string(result.GoSource)
	// The extracted blocks leave through reference tags; the inline content is
	// gone from the generated head, so a policy may forbid inline script.
	for _, want := range []string{
		`<script src=\"` + script.URL + `\" defer type=\"module\"></script>`,
		`<script src=\"https://cdn.example.com/lib.js\" async></script>`,
		`<link rel=\"stylesheet\" href=\"` + style.URL + `\">`,
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated head lacks %s:\n%s", want, generated)
		}
	}
	if strings.Contains(generated, "<style") || strings.Contains(generated, "color: blue") {
		t.Fatalf("style block was not extracted:\n%s", generated)
	}
}

// TestExtractionDefaultsToThePublicGeneratedBase covers a project configuring
// neither public option.
func TestExtractionDefaultsToThePublicGeneratedBase(t *testing.T) {
	result, err := htmlbind.GenerateModule("widget.tb.html", []byte(assetSource), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range result.Assets {
		if !strings.HasPrefix(asset.URL, "/public/generated/") {
			t.Fatalf("asset URL %q does not use the default base", asset.URL)
		}
		if asset.URL != "/public/generated/"+asset.FileName() {
			t.Fatalf("asset URL %q does not name the written file %q", asset.URL, asset.FileName())
		}
		if !strings.HasPrefix(asset.Base, "widget."+string(asset.Kind)+".") {
			t.Fatalf("asset base %q is not the unit name plus its kind", asset.Base)
		}
	}
}

// TestExtractionIsDeterministic covers cache validity: an unchanged project
// regenerates identical names and bytes.
func TestExtractionIsDeterministic(t *testing.T) {
	first, err := htmlbind.GenerateModule("widget.tb.html", []byte(assetSource), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := htmlbind.GenerateModule("widget.tb.html", []byte(assetSource), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Assets) != len(second.Assets) {
		t.Fatalf("asset count changed: %d then %d", len(first.Assets), len(second.Assets))
	}
	for i := range first.Assets {
		if first.Assets[i].FileName() != second.Assets[i].FileName() {
			t.Fatalf("asset name changed: %q then %q", first.Assets[i].FileName(), second.Assets[i].FileName())
		}
		if string(first.Assets[i].Content) != string(second.Assets[i].Content) {
			t.Fatalf("asset %s content changed across runs", first.Assets[i].FileName())
		}
	}
	// A changed style changes the hash, so a client never serves stale CSS.
	changed, err := htmlbind.GenerateModule("widget.tb.html",
		[]byte(strings.Replace(assetSource, "color: blue", "color: red", 1)), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Assets[0].FileName() == first.Assets[0].FileName() {
		t.Fatalf("edited stylesheet reused the name %q", changed.Assets[0].FileName())
	}
}

// TestExtractionUsesAFullURLBaseVerbatim covers the CDN case: the reference
// changes, the file name does not.
func TestExtractionUsesAFullURLBaseVerbatim(t *testing.T) {
	local, err := htmlbind.GenerateModule("widget.tb.html", []byte(assetSource), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cdn, err := htmlbind.GenerateModule("widget.tb.html", []byte(assetSource), htmlbind.GenerateOptions{
		PublicURLBase: "https://cdn.example.com/assets/",
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, asset := range cdn.Assets {
		want := "https://cdn.example.com/assets/" + asset.FileName()
		if asset.URL != want {
			t.Fatalf("asset URL = %q, want %q", asset.URL, want)
		}
		if asset.FileName() != local.Assets[i].FileName() {
			t.Fatalf("URL base changed the file name: %q vs %q", asset.FileName(), local.Assets[i].FileName())
		}
	}
}

// TestStylesBundlePerGenerationUnit checks that two components of one file share
// one stylesheet and one link, while each script stays its own file.
func TestStylesBundlePerGenerationUnit(t *testing.T) {
	source := []byte(`package pages

export component Left(): html {
<head><style>.left { color: red }</style><script>left()</script></head>
<p class="left">left</p>
}

export component Right(): html {
<head><style>.right { color: green }</style><script>right()</script></head>
<p class="right">right</p>
}
`)
	result, err := htmlbind.GenerateModule("pair.tb.html", source, htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var styles, scripts int
	for _, asset := range result.Assets {
		if asset.Kind == htmlbind.AssetStyle {
			styles++
			for _, want := range []string{"color: red", "color: green"} {
				if !strings.Contains(string(asset.Content), want) {
					t.Fatalf("bundle lacks %q:\n%s", want, asset.Content)
				}
			}
		} else {
			scripts++
		}
	}
	if styles != 1 {
		t.Fatalf("want one stylesheet bundle per generation unit, got %d", styles)
	}
	if scripts != 2 {
		t.Fatalf("want one script file per component, got %d", scripts)
	}
}

// TestSharedStylesheetLinkIsMergedOnce covers the head-merging acceptance case:
// two components declaring the same stylesheet emit one link.
func TestSharedStylesheetLinkIsMergedOnce(t *testing.T) {
	source := []byte(`package pages

component Inner(): html {
<head><style>.inner { color: red }</style></head>
<p class="inner">inner</p>
}

export component Outer(): html {
<head><style>.outer { color: green }</style></head>
<div class="outer"><Inner /></div>
}
`)
	result, err := htmlbind.GenerateModule("shared.tb.html", source, htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(result.GoSource), `<link rel=\"stylesheet\"`); count != 2 {
		// One link literal per plan: the leaf's own, and the caller's merged
		// head holding exactly one shared reference.
		t.Fatalf("want one link per plan head, got %d:\n%s", count, result.GoSource)
	}
}

// TestEmptyStyleBlockReferencesNothing keeps a component that declares no rules
// from linking a stylesheet it does not contribute to.
func TestEmptyStyleBlockReferencesNothing(t *testing.T) {
	source := []byte(`package pages

export component Blank(): html {
<head><style>
</style></head>
<p>blank</p>
}
`)
	result, err := htmlbind.GenerateModule("blank.tb.html", source, htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assets) != 0 {
		t.Fatalf("empty style block produced %+v", result.Assets)
	}
	if strings.Contains(string(result.GoSource), "<link") {
		t.Fatalf("empty style block still emitted a link:\n%s", result.GoSource)
	}
}
