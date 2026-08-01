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

const hookTemplate = `package fixture

export component Page(): html {
<head>
<script src="/public/app.ts" type="module"></script>
</head>
<img src="/public/hero.png" alt="hero">
<img src="/public/hero.png" alt="again">
}
`

func writeHookFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		"page.tb.html": hookTemplate,
		"doc.go":       "package fixture\n\nimport _ \"github.com/shibukawa/tinybind-go/htmlbind\"\n",
		"go.mod": "module fixture\n\ngo 1.26\n\n" +
			"require github.com/shibukawa/tinybind-go v0.0.0\n\n" +
			"replace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(root) + "\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tidyTempModule(t, dir)
	return dir
}

// imageAndScriptHooks are the two driving cases side by side, and they disagree
// about naming: the image appends and the script replaces. That disagreement is
// why neither package holds a naming rule of its own.
func imageAndScriptHooks(dir string, calls *int) []htmlbind.ReferenceHook {
	count := func() {
		if calls != nil {
			*calls++
		}
	}
	return []htmlbind.ReferenceHook{
		{
			Name: "image-format", Element: "img", Attribute: "src",
			Match: func(value string) bool { return strings.HasSuffix(value, ".png") },
			CacheKey: func(request htmlbind.ReferenceRequest) (htmlbind.ConversionInputs, error) {
				return htmlbind.ConversionInputs{
					Sources: []string{filepath.Join(dir, "public", "hero.png")},
					Params:  "webp q80 encoder-1.2",
				}, nil
			},
			Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				count()
				name := strings.TrimPrefix(request.Value, "/public/") + ".webp"
				return htmlbind.ReferenceResult{
					Value: request.Value + ".webp",
					Files: []htmlbind.ProducedFile{{
						Name: name, MediaType: "image/webp", Content: []byte("converted hero"),
					}},
				}, nil
			},
		},
		{
			Name: "script-compile", Element: "script", Attribute: "src",
			Match: func(value string) bool { return strings.HasSuffix(value, ".ts") },
			CacheKey: func(request htmlbind.ReferenceRequest) (htmlbind.ConversionInputs, error) {
				return htmlbind.ConversionInputs{
					Sources: []string{filepath.Join(dir, "public", "app.ts")},
					Params:  "esnext",
				}, nil
			},
			Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				count()
				return htmlbind.ReferenceResult{
					Value: strings.TrimSuffix(request.Value, ".ts") + ".js",
					Files: []htmlbind.ProducedFile{
						{Name: "app.js", MediaType: "text/javascript", Content: []byte("converted app")},
						// A source map is produced and no attribute names it, so
						// the produced set is not the rewrite set.
						{Name: "maps/app.js.map", Content: []byte("{}")},
					},
					// An entry point's imports are named by no template.
					Read: []string{filepath.Join(dir, "public", "lib", "util.ts")},
				}, nil
			},
		},
	}
}

// writeHookSources lays the authored files a cache key names on disk, because a
// key digests their contents.
func writeHookSources(t *testing.T, dir string) {
	t.Helper()
	assets := filepath.Join(dir, "public", "lib")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		filepath.Join(dir, "public", "hero.png"):       "png bytes",
		filepath.Join(dir, "public", "app.ts"):         "export {}",
		filepath.Join(dir, "public", "lib", "u.ts"):    "helper",
		filepath.Join(dir, "public", "lib", "util.ts"): "helper",
	} {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestConversionsAreCachedAcrossRuns is what makes inline conversion
// affordable: the second build reuses the whole outcome and converts nothing.
func TestConversionsAreCachedAcrossRuns(t *testing.T) {
	dir := writeHookFixture(t)
	writeHookSources(t, dir)
	derived := t.TempDir()
	cacheDir := t.TempDir()
	calls := 0

	options := generator.DefaultOptions()
	options.ReferenceHooks = imageAndScriptHooks(dir, &calls)
	options.DerivedAssetDir = derived
	options.ConversionCacheDir = cacheDir

	result, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("first build converted %d times, want the image and the script once each", calls)
	}
	for name, want := range map[string]string{
		"hero.png.webp":   "converted hero",
		"app.js":          "converted app",
		"maps/app.js.map": "{}",
	} {
		content, err := os.ReadFile(filepath.Join(derived, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("produced file %s was not written: %v", name, err)
		}
		if string(content) != want {
			t.Fatalf("produced file %s = %q, want %q", name, content, want)
		}
	}
	// Every produced file joins the run's declared outputs, which is what
	// --check compares and what a cleanup pass could ever act on.
	var declared int
	for _, path := range result.Paths() {
		if strings.HasPrefix(path, derived) {
			declared++
		}
	}
	if declared != 3 {
		t.Fatalf("declared %d produced files in the result paths, want 3: %v", declared, result.Paths())
	}

	generated, err := os.ReadFile(result.TemplatesPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`src=\"/public/hero.png.webp\" alt=\"hero\"`,
		`src=\"/public/hero.png.webp\" alt=\"again\"`,
		`<script src=\"/public/app.js\" type=\"module\"></script>`,
	} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated templates lack %s:\n%s", want, generated)
		}
	}

	// A fresh generator over the same sources, with the stamp forced aside, must
	// answer from the cache rather than convert again.
	calls = 0
	if _, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir, Force: true}); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("second build converted %d times, want everything answered from the cache", calls)
	}

	// Editing a source named by a cache key must convert again, and only that
	// one.
	if err := os.WriteFile(filepath.Join(dir, "public", "hero.png"), []byte("new png bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls = 0
	if _, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir, Force: true}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("after editing one source the build converted %d times, want 1", calls)
	}
}

// TestDeclinedConversionIsCachedToo is the size-regression case: an encode that
// loses to its source is declined, and the decision is stored, so the losing
// encode runs once and never again.
func TestDeclinedConversionIsCachedToo(t *testing.T) {
	dir := writeHookFixture(t)
	writeHookSources(t, dir)
	cacheDir := t.TempDir()
	source := filepath.Join(dir, "public", "hero.png")
	calls := 0

	options := generator.DefaultOptions()
	options.DerivedAssetDir = t.TempDir()
	options.ConversionCacheDir = cacheDir
	options.ReferenceHooks = []htmlbind.ReferenceHook{{
		Name: "image-format", Element: "img", Attribute: "src",
		Match: func(value string) bool { return strings.HasSuffix(value, ".png") },
		CacheKey: func(htmlbind.ReferenceRequest) (htmlbind.ConversionInputs, error) {
			return htmlbind.ConversionInputs{Sources: []string{source}, Params: "webp q80"}, nil
		},
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			calls++
			original, err := os.ReadFile(source)
			if err != nil {
				return htmlbind.ReferenceResult{}, err
			}
			// The encode came out larger, which only the converted bytes can
			// say. Shipping a bigger file to gain a format is a loss.
			encoded := append(original, []byte(" and then some")...)
			if len(encoded) >= len(original) {
				return htmlbind.ReferenceResult{Skip: true, Reason: "webp was larger than the source"}, nil
			}
			return htmlbind.ReferenceResult{Value: request.Value + ".webp"}, nil
		},
	}}

	result, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("first build converted %d times, want 1", calls)
	}
	if len(result.Rewrites) != 1 || !result.Rewrites[0].Skipped {
		t.Fatalf("the losing encode was not reported as a skip: %+v", result.Rewrites)
	}
	generated, err := os.ReadFile(result.TemplatesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `src=\"/public/hero.png\" alt=\"hero\"`) {
		t.Fatalf("a declined conversion must leave the reference alone:\n%s", generated)
	}

	calls = 0
	again, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("the losing encode ran again; a declined conversion must be cached like any other outcome")
	}
	if len(again.Rewrites) != 1 || !again.Rewrites[0].Skipped || again.Rewrites[0].Reason != "webp was larger than the source" {
		t.Fatalf("the cached decision lost its reason: %+v", again.Rewrites)
	}
}

// TestNoCacheDirectoryConvertsEveryTime keeps the unconfigured case correct,
// which matters more than keeping it fast.
func TestNoCacheDirectoryConvertsEveryTime(t *testing.T) {
	dir := writeHookFixture(t)
	writeHookSources(t, dir)
	calls := 0
	options := generator.DefaultOptions()
	options.ReferenceHooks = imageAndScriptHooks(dir, &calls)
	options.DerivedAssetDir = t.TempDir()

	for range 2 {
		if _, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir, Force: true}); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 4 {
		t.Fatalf("converted %d times over two builds with no cache, want 4", calls)
	}
}

// TestProducedFileWithNoDirectoryIsAConfigurationError: discarding the file
// silently would leave the rewritten reference dangling, which is the property
// this seam guarantees.
func TestProducedFileWithNoDirectoryIsAConfigurationError(t *testing.T) {
	dir := writeHookFixture(t)
	writeHookSources(t, dir)
	options := generator.DefaultOptions()
	options.ReferenceHooks = imageAndScriptHooks(dir, nil)

	_, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
	if err == nil {
		t.Fatal("a produced file with nowhere to go was accepted")
	}
	if !strings.Contains(err.Error(), "DerivedAssetDir") {
		t.Fatalf("error does not name the option: %v", err)
	}
}

// TestMalformedHookIsReportedAgainstTheRegistration keeps a fault in the
// generate command off a template position it has nothing to do with.
func TestMalformedHookIsReportedAgainstTheRegistration(t *testing.T) {
	dir := writeHookFixture(t)
	options := generator.DefaultOptions()
	options.ReferenceHooks = []htmlbind.ReferenceHook{{
		Name: "broken", Element: "my-image", Attribute: "src",
		Transform: func(htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			return htmlbind.ReferenceResult{}, nil
		},
	}}

	_, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
	if err == nil {
		t.Fatal("a hyphenated hook element was accepted")
	}
	if !strings.Contains(err.Error(), "hyphenated") || strings.Contains(err.Error(), "page.tb.html") {
		t.Fatalf("want a registration error with no template position, got: %v", err)
	}
}

// TestRegisteringNoHookLeavesGenerationUnchanged is the constraint every
// accepted seam in this catalog carries.
func TestRegisteringNoHookLeavesGenerationUnchanged(t *testing.T) {
	// One fixture directory for both runs: generated source records the real
	// path of the template it came from, so two directories could never match
	// whatever the hooks did.
	dir := writeHookFixture(t)
	read := func(t *testing.T, options generator.Options) string {
		t.Helper()
		result, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir, Force: true})
		if err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(result.TemplatesPath)
		if err != nil {
			t.Fatal(err)
		}
		// The stamp records the run's own directory, which differs per fixture,
		// so it is not part of what this test compares.
		var kept []string
		for _, line := range strings.Split(string(source), "\n") {
			if !strings.HasPrefix(line, "// tinybind:generated") {
				kept = append(kept, line)
			}
		}
		return strings.Join(kept, "\n")
	}
	plain := read(t, generator.DefaultOptions())
	unused := generator.DefaultOptions()
	unused.ReferenceHooks = []htmlbind.ReferenceHook{{
		Name: "unused", Element: "video", Attribute: "poster",
		Transform: func(htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			t.Fatal("a transform ran for an element the template never writes")
			return htmlbind.ReferenceResult{}, nil
		},
	}}
	if withHook := read(t, unused); withHook != plain {
		t.Fatal("registering an unused hook changed generated output")
	}
}

// TestOneAssetConvertsOnceAcrossTemplates: the in-run memo spans the run, so two
// templates naming one image convert it once even on a cold cache.
func TestOneAssetConvertsOnceAcrossTemplates(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	page := func(name string) string {
		return "package fixture\n\nexport component " + name + "(): html {\n" +
			"<img src=\"/public/hero.png\" alt=\"hero\">\n}\n"
	}
	files := map[string]string{
		"a.tb.html": page("A"),
		"b.tb.html": page("B"),
		"doc.go":    "package fixture\n\nimport _ \"github.com/shibukawa/tinybind-go/htmlbind\"\n",
		"go.mod": "module fixture\n\ngo 1.26\n\n" +
			"require github.com/shibukawa/tinybind-go v0.0.0\n\n" +
			"replace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(root) + "\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tidyTempModule(t, dir)
	writeHookSources(t, dir)

	converted := 0
	options := generator.DefaultOptions()
	options.DerivedAssetDir = t.TempDir()
	options.ReferenceHooks = []htmlbind.ReferenceHook{{
		Name: "convert", Element: "img", Attribute: "src",
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			converted++
			return htmlbind.ReferenceResult{
				Value: request.Value + ".webp",
				Files: []htmlbind.ProducedFile{{Name: "hero.png.webp", Content: []byte("webp bytes")}},
			}, nil
		},
	}}
	if _, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir}); err != nil {
		t.Fatal(err)
	}
	if converted != 1 {
		t.Fatalf("converted %d times for one asset across two templates, want 1", converted)
	}
	generated, err := os.ReadFile(filepath.Join(dir, generator.DefaultTemplatesName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(generated), `/public/hero.png.webp`) != 2 {
		t.Fatalf("both templates should carry the rewrite:\n%s", generated)
	}
}
