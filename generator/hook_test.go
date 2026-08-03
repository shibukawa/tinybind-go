package generator_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// parallelTemplate names several distinct images plus one repeated, so a test
// can tell concurrent conversion from concurrent re-conversion.
const parallelTemplate = `package fixture

export component Page(): html {
<img src="/public/a.png" alt="a">
<img src="/public/b.png" alt="b">
<img src="/public/c.png" alt="c">
<img src="/public/d.png" alt="d">
<img src="/public/a.png" alt="a again">
}
`

// writeParallelFixture lays out a module whose conversions are independent, so
// converting them together is the only thing that changes.
func writeParallelFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		"page.tb.html": parallelTemplate,
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
	assets := filepath.Join(dir, "public")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.png", "b.png", "c.png", "d.png"} {
		if err := os.WriteFile(filepath.Join(assets, name), []byte("png "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tidyTempModule(t, dir)
	return dir
}

// slowImageHook is safe for concurrent use and slow enough that running four of
// them in sequence is visibly different from running them together.
func slowImageHook(dir string, calls *int64, delay time.Duration, fail map[string]bool) htmlbind.ReferenceHook {
	return htmlbind.ReferenceHook{
		Name: "image-format", Element: "img", Attribute: "src",
		Match: func(value string) bool { return strings.HasSuffix(value, ".png") },
		CacheKey: func(request htmlbind.ReferenceRequest) (htmlbind.ConversionInputs, error) {
			name := strings.TrimPrefix(request.Value, "/public/")
			return htmlbind.ConversionInputs{
				Sources: []string{filepath.Join(dir, "public", name)},
				Params:  "webp q80",
			}, nil
		},
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			atomic.AddInt64(calls, 1)
			time.Sleep(delay)
			if fail[request.Value] {
				return htmlbind.ReferenceResult{}, errors.New("encoder refused " + request.Value)
			}
			name := strings.TrimPrefix(request.Value, "/public/") + ".webp"
			return htmlbind.ReferenceResult{
				Value: request.Value + ".webp",
				Files: []htmlbind.ProducedFile{{
					Name: name, MediaType: "image/webp", Content: []byte("converted " + name),
				}},
			}, nil
		},
	}
}

func parallelOptions(dir, derived string, calls *int64, workers int, delay time.Duration, fail map[string]bool) generator.Options {
	options := generator.DefaultOptions()
	options.ReferenceHooks = []htmlbind.ReferenceHook{slowImageHook(dir, calls, delay, fail)}
	options.DerivedAssetDir = derived
	options.ConversionWorkers = workers
	return options
}

// TestParallelConversionProducesIdenticalOutput is the property the whole
// option rests on: it buys wall clock and changes nothing else.
//
// Both builds run over one directory with one derived root, so the only thing
// that differs between them is the worker count. That also proves the count
// stays out of the hashed options: it appears in neither the stamp nor the
// bytes under it.
func TestParallelConversionProducesIdenticalOutput(t *testing.T) {
	dir := writeParallelFixture(t)
	derived := t.TempDir()
	build := func(workers int) ([]byte, []string) {
		var calls int64
		options := parallelOptions(dir, derived, &calls, workers, 0, nil)
		result, err := generator.New(options).GeneratePackage(context.Background(),
			generator.GenerateRequest{Dir: dir, Out: dir, Force: true})
		if err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(result.TemplatesPath)
		if err != nil {
			t.Fatal(err)
		}
		var rewrites []string
		for _, rewrite := range result.Rewrites {
			rewrites = append(rewrites, rewrite.From+" -> "+rewrite.To)
		}
		return source, rewrites
	}
	sequential, sequentialRewrites := build(0)
	concurrent, concurrentRewrites := build(4)
	if string(sequential) != string(concurrent) {
		t.Fatalf("concurrent conversion changed the generated bytes:\n--- one ---\n%s--- many ---\n%s", sequential, concurrent)
	}
	// The report is built by the sequential compile, so worker completion order
	// must be invisible in it.
	if strings.Join(sequentialRewrites, ",") != strings.Join(concurrentRewrites, ",") {
		t.Fatalf("the report order changed: %v against %v", sequentialRewrites, concurrentRewrites)
	}
}

// TestParallelConversionConvertsEachValueOnce covers the single-flight memo: the
// repeated reference must not become a second encode just because the discovery
// pass found it first.
func TestParallelConversionConvertsEachValueOnce(t *testing.T) {
	dir := writeParallelFixture(t)
	var calls int64
	options := parallelOptions(dir, t.TempDir(), &calls, 4, 10*time.Millisecond, nil)
	if _, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir}); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt64(&calls); got != 4 {
		t.Fatalf("converted %d times, want one per distinct value (4)", got)
	}
}

// TestParallelConversionDefersErrors covers the reporting rule: a conversion
// that fails on a worker says nothing there, and the compile raises it at the
// template position, so the reported failure is the first in template order
// rather than whichever goroutine lost the race.
func TestParallelConversionDefersErrors(t *testing.T) {
	dir := writeParallelFixture(t)
	derived := t.TempDir()
	fail := map[string]bool{"/public/b.png": true, "/public/c.png": true}
	message := func(workers int) string {
		var calls int64
		options := parallelOptions(dir, derived, &calls, workers, 0, fail)
		_, err := generator.New(options).GeneratePackage(context.Background(),
			generator.GenerateRequest{Dir: dir, Out: dir, Force: true})
		if err == nil {
			t.Fatal("a failing conversion did not fail the build")
		}
		return err.Error()
	}
	sequential := message(0)
	// Repeated, because a race that resolves one way most of the time is the
	// failure mode this rule exists to remove.
	for i := 0; i < 5; i++ {
		if concurrent := message(4); sequential != concurrent {
			t.Fatalf("the reported error depends on concurrency:\n--- one ---\n%s\n--- many ---\n%s", sequential, concurrent)
		}
	}
	// b.png is written before c.png, so it is the one a build reports.
	if !strings.Contains(sequential, "b.png") || strings.Contains(sequential, "c.png") {
		t.Fatalf("the first failure in template order was not the one reported: %s", sequential)
	}
}

// TestParallelConversionWarmCacheConvertsNothing covers the interaction that
// matters most in practice: an incremental build starts no worker because there
// is nothing to convert.
func TestParallelConversionWarmCacheConvertsNothing(t *testing.T) {
	dir := writeParallelFixture(t)
	cacheDir := t.TempDir()
	derived := t.TempDir()
	build := func(calls *int64) {
		options := generator.DefaultOptions()
		options.ReferenceHooks = []htmlbind.ReferenceHook{slowImageHook(dir, calls, 0, nil)}
		options.DerivedAssetDir = derived
		options.ConversionCacheDir = cacheDir
		options.ConversionWorkers = 4
		if _, err := generator.New(options).GeneratePackage(context.Background(),
			generator.GenerateRequest{Dir: dir, Out: dir, Force: true}); err != nil {
			t.Fatal(err)
		}
	}
	var cold int64
	build(&cold)
	if cold != 4 {
		t.Fatalf("the cold build converted %d times, want 4", cold)
	}
	var warm int64
	build(&warm)
	if warm != 0 {
		t.Fatalf("the warm build converted %d times, want none", warm)
	}
}

// TestCachedConversionReplaysHeadContribution covers the entry surviving a
// cache round trip: a build answered entirely from the store must still link
// the stylesheet, or the second build serves a page the first one did not.
func TestCachedConversionReplaysHeadContribution(t *testing.T) {
	dir := writeHookFixture(t)
	writeHookSources(t, dir)
	cacheDir := t.TempDir()
	derived := t.TempDir()
	var calls int64
	stylesheet := htmlbind.ReferenceHook{
		Name: "script-compile", Element: "script", Attribute: "src",
		Match: func(value string) bool { return strings.HasSuffix(value, ".ts") },
		CacheKey: func(request htmlbind.ReferenceRequest) (htmlbind.ConversionInputs, error) {
			return htmlbind.ConversionInputs{
				Sources: []string{filepath.Join(dir, "public", "app.ts")},
				Params:  "esnext",
			}, nil
		},
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			atomic.AddInt64(&calls, 1)
			return htmlbind.ReferenceResult{
				Value: strings.TrimSuffix(request.Value, ".ts") + ".js",
				Files: []htmlbind.ProducedFile{
					{Name: "app.js", Content: []byte("built")},
					{Name: "app.css", Content: []byte(".a{}")},
				},
				Head: []htmlbind.HeadEntry{{
					Element:    "link",
					Attributes: map[string]string{"rel": "stylesheet", "href": "/public/app.css"},
				}},
			}, nil
		},
	}
	build := func() string {
		options := generator.DefaultOptions()
		options.ReferenceHooks = []htmlbind.ReferenceHook{stylesheet}
		options.DerivedAssetDir = derived
		options.ConversionCacheDir = cacheDir
		result, err := generator.New(options).GeneratePackage(context.Background(),
			generator.GenerateRequest{Dir: dir, Out: dir, Force: true})
		if err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(result.TemplatesPath)
		if err != nil {
			t.Fatal(err)
		}
		return string(source)
	}
	first := build()
	if !strings.Contains(first, `app.css`) {
		t.Fatalf("the produced stylesheet was not linked:\n%s", first)
	}
	again := build()
	if atomic.LoadInt64(&calls) != 1 {
		t.Fatalf("the second build converted again: %d calls", calls)
	}
	if first != again {
		t.Fatalf("a cached conversion lost its head contribution:\n--- first ---\n%s--- again ---\n%s", first, again)
	}
}

// TestParallelConversionOverlaps proves the conversions actually run together
// rather than merely producing the right answer.
//
// It asserts overlap directly instead of timing the build: each transform waits
// until every other one has started, so a serialized implementation blocks its
// first conversion and fails with a message saying so, while a concurrent one
// releases immediately. Nothing here depends on how fast the machine is.
func TestParallelConversionOverlaps(t *testing.T) {
	dir := writeParallelFixture(t)
	const want = 4
	var inflight int64
	overlapped := make(chan struct{})
	var once sync.Once
	hook := htmlbind.ReferenceHook{
		Name: "image-format", Element: "img", Attribute: "src",
		Match: func(value string) bool { return strings.HasSuffix(value, ".png") },
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			if atomic.AddInt64(&inflight, 1) == want {
				once.Do(func() { close(overlapped) })
			}
			select {
			case <-overlapped:
			case <-time.After(10 * time.Second):
				return htmlbind.ReferenceResult{}, errors.New("conversions did not overlap")
			}
			return htmlbind.ReferenceResult{Value: request.Value + ".webp"}, nil
		},
	}
	options := generator.DefaultOptions()
	options.ReferenceHooks = []htmlbind.ReferenceHook{hook}
	options.ConversionWorkers = want
	if _, err := generator.New(options).GeneratePackage(context.Background(),
		generator.GenerateRequest{Dir: dir, Out: dir}); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt64(&inflight); got != want {
		t.Fatalf("%d conversions ran, want %d", got, want)
	}
}
