package generator_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// A caller generating a tree of packages spends nearly all of it type-checking,
// one directory at a time, and the obvious way to spend less is to run the
// directories at once. Whether that is allowed is this module's answer to give:
// a Generator carries the run's options and nothing else, so the promise is
// about what the phases behind it reach, not about the struct.
//
// These tests are that promise held to the race detector. They run the same
// directories sequentially and concurrently against one shared Generator and
// require the same bytes from both, which is the property a caller needs:
// generation is a pure function of a directory, so scheduling cannot change
// what is generated.

// concurrentFixturePackages is wide enough that the detector sees real overlap
// and small enough that a run of it stays affordable: every package here is
// type-checked with its dependencies.
const concurrentFixturePackages = 6

const concurrentHandlerSource = `package pkg%[1]d

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
	_ "github.com/shibukawa/tinybind-go/htmlbind"
)

// CreateRequest is the bound request model.
type CreateRequest struct {
	Name string ` + "`payload:\"name\" check:\"required\"`" + `
	Age  int    ` + "`payload:\"age\"`" + `
}

// Created is the written response model.
type Created struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func Handler(w http.ResponseWriter, r *http.Request) {
	req, err := httpbind.Bind[CreateRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[Created](w, r, Created{ID: "1", Name: req.Name})
}

func Register() {
	http.HandleFunc("POST /pkg%[1]d/items", Handler)
}
`

// A config declaration whose help text comes from godoc, so the phase that
// rewrites hand-written sources runs too. Every package declares its own, so
// what a run rewrites belongs to the directory that rewrote it.
const concurrentConfigSource = `package pkg%[1]d

import "github.com/shibukawa/tinybind-go/configbind"

// Settings configures this package.
type Settings struct {
	// Port is the listen port.
	Port int ` + "`default:\"8080\"`" + `
	// Host is the listen address.
	Host string
}

// BindSettings registers the configuration.
func BindSettings() *Settings { return configbind.Bind[Settings]("pkg%[1]d") }
`

// A SQL declaration, so the other template compiler is on the path as well.
const concurrentQuerySource = `package pkg%[1]d
type Row { id: int, name: string }
export statement FindRow(id: int): sql.optional<Row> {SELECT id, name FROM rows WHERE id = {id}}
`

// Every package names one shared image, so the conversion memo and the cache
// are exercised by directories converting the same value at the same time.
const concurrentTemplateSource = `package pkg%[1]d

export component Card(label: string): html {
<img src="/public/hero.png" alt="hero">
<p class="card">{label}</p>
}
`

// writeConcurrentFixture lays out one module of independent packages and
// returns the module root and every package directory in it.
func writeConcurrentFixture(t *testing.T, packages int) (string, []string) {
	t.Helper()
	root := t.TempDir()
	writeTempModule(t, root)
	assets := filepath.Join(root, "public")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "hero.png"), []byte("png bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs := make([]string, 0, packages)
	for i := range packages {
		dir := filepath.Join(root, fmt.Sprintf("pkg%d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, source := range map[string]string{
			"handler.go":   fmt.Sprintf(concurrentHandlerSource, i),
			"settings.go":  fmt.Sprintf(concurrentConfigSource, i),
			"card.tb.html": fmt.Sprintf(concurrentTemplateSource, i),
			"rows.tb.sql":  fmt.Sprintf(concurrentQuerySource, i),
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		dirs = append(dirs, dir)
	}
	tidyTempModule(t, root)
	return root, dirs
}

// artifactFingerprint is everything one directory's generation produced,
// reduced to a comparable string: the artifacts, in a stable order, with their
// content digests, and the route analysis beside them.
//
// It returns an error rather than failing, because it is called from goroutines
// the test does not own.
func artifactFingerprint(runner *generator.Generator, dir string, set *generator.PackageSet) (string, error) {
	artifacts, routes, err := runner.GenerateArtifactsWithRoutes(context.Background(),
		generator.GenerateRequest{Dir: dir, OpenAPI: true, Packages: set})
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		digest := sha256.Sum256(artifact.Content)
		lines = append(lines, fmt.Sprintf("%v\t%v\t%s\t%s\t%s\t%s\t%s",
			artifact.Kind, artifact.Destination, artifact.OutputBase, artifact.Extension,
			artifact.PackageName, artifact.PublicPath, hex.EncodeToString(digest[:])))
	}
	sort.Strings(lines)
	encoded, err := json.Marshal(routes)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n") + "\n---\n" + string(encoded), nil
}

// fingerprintAll runs every directory on its own goroutine when concurrent is
// set, and one after another when it is not.
func fingerprintAll(t *testing.T, runner *generator.Generator, dirs []string, concurrent bool) []string {
	t.Helper()
	return fingerprintAllFrom(t, runner, dirs, nil, concurrent)
}

// fingerprintAllFrom is fingerprintAll served from one loaded set.
func fingerprintAllFrom(t *testing.T, runner *generator.Generator, dirs []string, set *generator.PackageSet, concurrent bool) []string {
	t.Helper()
	out := make([]string, len(dirs))
	errs := make([]error, len(dirs))
	if !concurrent {
		for i, dir := range dirs {
			out[i], errs[i] = artifactFingerprint(runner, dir, set)
		}
	} else {
		var wait sync.WaitGroup
		for i, dir := range dirs {
			wait.Add(1)
			go func() {
				defer wait.Done()
				out[i], errs[i] = artifactFingerprint(runner, dir, set)
			}()
		}
		wait.Wait()
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("%s: %v", dirs[i], err)
		}
	}
	return out
}

// TestConcurrentGenerationMatchesSequential is the safety claim itself: one
// Generator, several directories at once, the same bytes as one at a time.
// Run under -race it is also the claim that no phase behind it shares mutable
// state across directories.
func TestConcurrentGenerationMatchesSequential(t *testing.T) {
	_, dirs := writeConcurrentFixture(t, concurrentFixturePackages)
	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"
	runner := generator.New(options)

	sequential := fingerprintAll(t, runner, dirs, false)
	concurrent := fingerprintAll(t, runner, dirs, true)

	for i, dir := range dirs {
		if concurrent[i] != sequential[i] {
			t.Errorf("%s: concurrent generation produced different output:\nsequential:\n%s\nconcurrent:\n%s",
				dir, sequential[i], concurrent[i])
		}
	}
}

// TestConcurrentGenerationSharesConversionCache runs the same directories with
// a reference hook and one cache directory between them, which is what a build
// converting assets does. The cache is cold for the concurrent run, so its
// entries are written by directories racing to convert one image.
func TestConcurrentGenerationSharesConversionCache(t *testing.T) {
	root, dirs := writeConcurrentFixture(t, concurrentFixturePackages)
	source := filepath.Join(root, "public", "hero.png")
	derived := t.TempDir()

	var converted atomic.Int64
	hook := htmlbind.ReferenceHook{
		Name: "image-format", Element: "img", Attribute: "src",
		Match: func(value string) bool { return strings.HasSuffix(value, ".png") },
		CacheKey: func(htmlbind.ReferenceRequest) (htmlbind.ConversionInputs, error) {
			return htmlbind.ConversionInputs{Sources: []string{source}, Params: "webp q80"}, nil
		},
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			converted.Add(1)
			original, err := os.ReadFile(source)
			if err != nil {
				return htmlbind.ReferenceResult{}, err
			}
			return htmlbind.ReferenceResult{
				Value: request.Value + ".webp",
				Files: []htmlbind.ProducedFile{{
					Name: "hero.png.webp", MediaType: "image/webp",
					Content: append([]byte("converted "), original...),
				}},
			}, nil
		},
	}
	options := func(cacheDir string) generator.Options {
		out := generator.DefaultOptions()
		out.SQLDialect = "postgresql"
		out.ReferenceHooks = []htmlbind.ReferenceHook{hook}
		out.DerivedAssetDir = derived
		out.ConversionCacheDir = cacheDir
		out.ConversionWorkers = 4
		return out
	}

	concurrent := fingerprintAll(t, generator.New(options(t.TempDir())), dirs, true)
	if converted.Load() == 0 {
		t.Fatal("the hook never ran, so this test proves nothing about converting concurrently")
	}
	sequential := fingerprintAll(t, generator.New(options(t.TempDir())), dirs, false)

	for i, dir := range dirs {
		if concurrent[i] != sequential[i] {
			t.Errorf("%s: converting concurrently produced different output:\nsequential:\n%s\nconcurrent:\n%s",
				dir, sequential[i], concurrent[i])
		}
	}
}
