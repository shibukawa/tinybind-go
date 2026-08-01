package generator_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

const depsTemplate = `package fixture

export component Page(): html {
<img src="/public/hero.src" alt="hero">
}
`

// writeDepsFixture lays out a package whose template points at an authored file
// on disk, which is what a real transform reads.
func writeDepsFixture(t *testing.T) (dir, source string) {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir = t.TempDir()
	assets := filepath.Join(dir, "public")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	source = filepath.Join(assets, "hero.src")
	files := map[string]string{
		filepath.Join(dir, "page.tb.html"): depsTemplate,
		source:                             "original",
		filepath.Join(dir, "doc.go"):       "package fixture\n\nimport _ \"github.com/shibukawa/tinybind-go/htmlbind\"\n",
		filepath.Join(dir, "go.mod"): "module fixture\n\ngo 1.26\n\n" +
			"require github.com/shibukawa/tinybind-go v0.0.0\n\n" +
			"replace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(root) + "\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tidyTempModule(t, dir)
	return dir, source
}

// convertingHook converts inline and names its inputs. The declared sources are
// hashed as build inputs whether the conversion ran or was answered from the
// cache, so an edit to one regenerates either way.
//
// With reportInputs off it names nothing, and its real input becomes invisible
// to the build, which is the documented limit.
func convertingHook(dir string, reportInputs bool) htmlbind.ReferenceHook {
	sourceOf := func(value string) string {
		return filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(value, "/")))
	}
	hook := htmlbind.ReferenceHook{
		Name: "convert", Element: "img", Attribute: "src",
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			content, err := os.ReadFile(sourceOf(request.Value))
			if err != nil {
				return htmlbind.ReferenceResult{}, err
			}
			return htmlbind.ReferenceResult{
				Value: request.Value + ".out",
				Files: []htmlbind.ProducedFile{{
					Name: "hero.src.out", Content: []byte("converted:" + string(content)),
				}},
			}, nil
		},
	}
	if reportInputs {
		hook.CacheKey = func(request htmlbind.ReferenceRequest) (htmlbind.ConversionInputs, error) {
			return htmlbind.ConversionInputs{Sources: []string{sourceOf(request.Value)}, Params: "v1"}, nil
		}
	}
	return hook
}

// TestEditedSourceRegenerates is the correctness property the read set exists
// for. A file a transform reads is not otherwise a hashed input, so without the
// recorded read set the second run would skip and ship the stale conversion.
func TestEditedSourceRegenerates(t *testing.T) {
	dir, source := writeDepsFixture(t)
	derived := t.TempDir()
	options := generator.DefaultOptions()
	options.ReferenceHooks = []htmlbind.ReferenceHook{convertingHook(dir, true)}
	options.DerivedAssetDir = derived
	run := func() generator.GenerateResult {
		t.Helper()
		result, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	produced := func() string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(derived, "hero.src.out"))
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}

	first := run()
	if first.Cached {
		t.Fatal("the first run reported a cache hit")
	}
	if first.DepsPath == "" {
		t.Fatal("a reported read set wrote no record")
	}
	if got := produced(); got != "converted:original" {
		t.Fatalf("produced %q, want the original conversion", got)
	}

	// Nothing changed: the recorded read set must not defeat the skip it
	// guards.
	if second := run(); !second.Cached {
		t.Fatal("an unchanged project regenerated")
	}

	if err := os.WriteFile(source, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	third := run()
	if third.Cached {
		t.Fatal("editing a file the transform read did not regenerate")
	}
	if got := produced(); got != "converted:edited" {
		t.Fatalf("produced %q after the edit, want the new conversion", got)
	}
}

// TestUnreportedReadIsNotHashed states the one correctness property this
// package cannot verify for the caller, so the limit is visible rather than
// discovered in production: a real input that no Conversion source and no
// transform names is invisible to the build.
func TestUnreportedReadIsNotHashed(t *testing.T) {
	dir, source := writeDepsFixture(t)
	derived := t.TempDir()
	options := generator.DefaultOptions()
	options.ReferenceHooks = []htmlbind.ReferenceHook{convertingHook(dir, false)}
	options.DerivedAssetDir = derived

	result, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.DepsPath != "" {
		t.Fatal("a hook naming no input still recorded one")
	}
	if err := os.WriteFile(source, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !again.Cached {
		t.Fatal("an unnamed input was somehow detected; update the documented limit")
	}
}

// TestNoHookWritesNoRecord keeps the file out of a project that uses none.
func TestNoHookWritesNoRecord(t *testing.T) {
	dir, _ := writeDepsFixture(t)
	result, err := generator.New(generator.DefaultOptions()).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.DepsPath != "" {
		t.Fatal("a project registering no hook wrote a dependency record")
	}
	if _, err := os.Stat(filepath.Join(dir, "tinybind_deps_gen.json")); !os.IsNotExist(err) {
		t.Fatalf("dependency record exists with no hook registered: %v", err)
	}
}
