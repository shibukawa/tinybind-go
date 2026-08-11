package routetree

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// scriptPage declares a component script block, which is the case a page tree
// could not serve: the compiler extracted the file and nothing returned it.
const scriptPage = `export component Page(label: string): html {
<script component>
export function setup(el) { el.textContent = "hi"; return () => {}; }
</script>
<div class="counter">{label}</div>
}
`

func generateTree(t *testing.T, root string, apply func(*GenerateOptions)) Result {
	t.Helper()
	options := GenerateOptions{
		Config:      Config{Root: root, ImportBase: "example.com/m/app"},
		RootPackage: "app",
	}
	if apply != nil {
		apply(&options)
	}
	result, err := GenerateTree(options)
	if err != nil {
		t.Fatalf("GenerateTree: %v", err)
	}
	return result
}

func pageSource(t *testing.T, files []Generated) string {
	t.Helper()
	for _, file := range files {
		if strings.HasSuffix(file.Path, "page_gen.go") {
			return string(file.Source)
		}
	}
	t.Fatalf("no page_gen.go among %d files", len(files))
	return ""
}

func TestGenerateTreeReturnsTheAssetsItsTemplatesExtracted(t *testing.T) {
	result := generateTree(t, tree(t, map[string]string{"page.tb.html": scriptPage}), nil)

	if len(result.Assets) != 1 {
		t.Fatalf("assets = %d, want 1: %+v", len(result.Assets), result.Assets)
	}
	asset := result.Assets[0]
	if asset.Kind != htmlbind.AssetScript {
		t.Errorf("asset kind = %q, want %q", asset.Kind, htmlbind.AssetScript)
	}
	if !strings.Contains(string(asset.Content), `el.textContent = "hi"`) {
		t.Errorf("asset content lost the authored body:\n%s", asset.Content)
	}
	// The whole point: the URL the generated component references is a file the
	// caller can now actually write.
	if !strings.Contains(pageSource(t, result.Files), asset.URL) {
		t.Errorf("generated page does not reference the returned asset URL %q", asset.URL)
	}
}

// Generate is the lossy variant, and stays that way so its existing callers see
// no change. The test exists because that is exactly the trap this package fell
// into: the discard is silent, and the symptom is a 404 at runtime.
func TestGenerateDiscardsAssetsAndStillEmitsEveryGoFile(t *testing.T) {
	root := tree(t, map[string]string{"page.tb.html": scriptPage})
	full := generateTree(t, root, nil)

	files, err := Generate(GenerateOptions{
		Config:      Config{Root: root, ImportBase: "example.com/m/app"},
		RootPackage: "app",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(files) != len(full.Files) {
		t.Fatalf("Generate returned %d files, GenerateTree %d", len(files), len(full.Files))
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Path, ".go") {
			t.Errorf("Generate returned a non-Go file: %s", file.Path)
		}
	}
}

func TestGenerateTreeDeduplicatesAnAssetTwoTemplatesExtract(t *testing.T) {
	result := generateTree(t, tree(t, map[string]string{
		"page.tb.html":       scriptPage,
		"about/page.tb.html": scriptPage,
	}), nil)

	// One name is one content, because the name carries the hash of the bytes.
	if len(result.Assets) != 1 {
		names := make([]string, len(result.Assets))
		for i, asset := range result.Assets {
			names[i] = asset.FileName()
		}
		t.Fatalf("assets = %v, want one deduplicated entry", names)
	}
}

func TestGenerateTreeCompilesWithTheCallersBoundaryPrefix(t *testing.T) {
	root := tree(t, map[string]string{"page.tb.html": scriptPage})

	result := generateTree(t, root, func(o *GenerateOptions) { o.DataAttributePrefix = "wave" })

	source := pageSource(t, result.Files)
	if !strings.Contains(source, `"data-wave-id"`) {
		t.Errorf("configured prefix did not reach the generated boundary attribute:\n%s", source)
	}
	// Without it a page tree took the default while a registered template took
	// the configured one, and one project disagreed with itself.
	if strings.Contains(source, `"data-tb-id"`) {
		t.Errorf("default prefix survived beside the configured one:\n%s", source)
	}
}

func TestGenerateTreeCompilesAssetURLsAgainstTheCallersPublicBase(t *testing.T) {
	root := tree(t, map[string]string{"page.tb.html": scriptPage})

	result := generateTree(t, root, func(o *GenerateOptions) { o.PublicURLBase = "/static/tb" })

	if len(result.Assets) != 1 {
		t.Fatalf("assets = %d, want 1", len(result.Assets))
	}
	if !strings.HasPrefix(result.Assets[0].URL, "/static/tb/") {
		t.Errorf("asset URL = %q, want it under /static/tb/", result.Assets[0].URL)
	}
	// The URL is written into the generated component, so the two must agree or
	// the reference points somewhere the caller never writes.
	if !strings.Contains(pageSource(t, result.Files), result.Assets[0].URL) {
		t.Errorf("generated page does not reference the rebased asset URL")
	}
}
