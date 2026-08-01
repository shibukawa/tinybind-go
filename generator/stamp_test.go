package generator_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/generator"
)

// stampRequest names every artifact of the custom framework fixture so the
// cache has to account for all four generated files.
func stampRequest(dir string) generator.GenerateRequest {
	return generator.GenerateRequest{
		Dir:            dir,
		Name:           "handler_pw_gen.go",
		TemplatesName:  "templates_pw_gen.go",
		OpenAPIName:    "openapi_pw_gen.go",
		ConfigBindName: "config_pw_gen.go",
		OpenAPI:        true,
	}
}

func generatePackage(t *testing.T, runner *generator.Generator, request generator.GenerateRequest) generator.GenerateResult {
	t.Helper()
	result, err := runner.GeneratePackage(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// writeTimes records when each generated file was last written, which is how
// these tests observe whether a run rewrote anything.
func writeTimes(t *testing.T, paths []string) map[string]time.Time {
	t.Helper()
	times := make(map[string]time.Time, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		times[path] = info.ModTime()
	}
	return times
}

func assertUntouched(t *testing.T, before map[string]time.Time, result generator.GenerateResult) {
	t.Helper()
	paths := result.Paths()
	if len(paths) != len(before) {
		t.Fatalf("paths=%v, want the %d files of the previous run", paths, len(before))
	}
	for _, path := range paths {
		written, ok := before[path]
		if !ok {
			t.Fatalf("%s was not written by the previous run", path)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(written) {
			t.Fatalf("%s was rewritten by a run that reported a cache hit", path)
		}
	}
}

func assertRewritten(t *testing.T, before map[string]time.Time, result generator.GenerateResult) {
	t.Helper()
	if result.Cached {
		t.Fatal("run reported a cache hit")
	}
	for _, path := range result.Paths() {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if written, ok := before[path]; ok && info.ModTime().Equal(written) {
			t.Fatalf("%s was not rewritten", path)
		}
	}
}

func appendToFile(t *testing.T, path, text string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(content, []byte(text)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGenerationStampSkipsUnchangedRuns walks one fixture through the whole
// cache lifecycle. The steps share a fixture on purpose: every regeneration
// type-checks the package, which is exactly the cost the stamp exists to avoid.
func TestGenerationStampSkipsUnchangedRuns(t *testing.T) {
	_, fixture := copyCustomFrameworkFixture(t)
	runner := generator.New(customFrameworkOptions(t))
	request := stampRequest(fixture)

	first := generatePackage(t, runner, request)
	if first.Cached {
		t.Fatal("the first run reported a cache hit")
	}
	if len(first.Paths()) != 4 {
		t.Fatalf("paths=%v, want templates, binder, configbind and OpenAPI", first.Paths())
	}
	for _, path := range first.Paths() {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "// tinybind:generated v1 inputs=sha256:") {
			t.Fatalf("%s carries no input hash:\n%s", filepath.Base(path), content)
		}
	}
	written := writeTimes(t, first.Paths())

	t.Run("unchanged inputs are skipped", func(t *testing.T) {
		result := generatePackage(t, runner, request)
		if !result.Cached {
			t.Fatal("the second run regenerated unchanged inputs")
		}
		assertUntouched(t, written, result)
	})

	t.Run("test files are not inputs", func(t *testing.T) {
		appendToFile(t, filepath.Join(fixture, "fixture_test.go"), "\n// touched by the generator cache test\n")
		result := generatePackage(t, runner, request)
		if !result.Cached {
			t.Fatal("a test file edit invalidated the cache")
		}
		assertUntouched(t, written, result)
	})

	t.Run("force regenerates", func(t *testing.T) {
		forced := request
		forced.Force = true
		result := generatePackage(t, runner, forced)
		assertRewritten(t, written, result)
		written = writeTimes(t, result.Paths())
	})

	t.Run("a source edit regenerates", func(t *testing.T) {
		appendToFile(t, filepath.Join(fixture, "handler.go"), "\n// touched by the generator cache test\n")
		assertRewritten(t, written, generatePackage(t, runner, request))
		written = writeTimes(t, first.Paths())
	})

	t.Run("a template edit regenerates", func(t *testing.T) {
		page := filepath.Join(fixture, "page.pw.html")
		content, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		edited := strings.Replace(string(content), "<p>{user.name}</p>", "<p>Hello {user.name}</p>", 1)
		if edited == string(content) {
			t.Fatalf("template fixture changed shape:\n%s", content)
		}
		if err := os.WriteFile(page, []byte(edited), 0o644); err != nil {
			t.Fatal(err)
		}
		assertRewritten(t, written, generatePackage(t, runner, request))
		written = writeTimes(t, first.Paths())
	})

	t.Run("a deleted output regenerates", func(t *testing.T) {
		removed := filepath.Join(fixture, "openapi_pw_gen.go")
		if err := os.Remove(removed); err != nil {
			t.Fatal(err)
		}
		result := generatePackage(t, runner, request)
		if result.Cached {
			t.Fatal("a deleted output was reported as up to date")
		}
		if _, err := os.Stat(removed); err != nil {
			t.Fatalf("deleted output not restored: %v", err)
		}
		written = writeTimes(t, first.Paths())
	})

	t.Run("an edited output regenerates", func(t *testing.T) {
		edited := filepath.Join(fixture, "handler_pw_gen.go")
		appendToFile(t, edited, "\n// hand edited\n")
		result := generatePackage(t, runner, request)
		if result.Cached {
			t.Fatal("an edited output was reported as up to date")
		}
		content, err := os.ReadFile(edited)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "// hand edited") {
			t.Fatalf("the hand edit survived regeneration:\n%s", content)
		}
	})
}

// TestGenerationStampIsPerPackage keeps one package's cache from answering for
// another, which is what makes `go generate ./...` skip only what is unchanged.
func TestGenerationStampIsPerPackage(t *testing.T) {
	root, fixture := copyCustomFrameworkFixture(t)
	runner := generator.New(customFrameworkOptions(t))

	first := generatePackage(t, runner, stampRequest(fixture))
	written := writeTimes(t, first.Paths())

	// The sibling package is generated from the same module, and generating it
	// may not disturb the fixture's cache. Whether it has anything to generate
	// is beside the point here, so its outcome is ignored.
	_, _ = runner.GeneratePackage(context.Background(), stampRequest(filepath.Join(root, "pw")))
	result := generatePackage(t, runner, stampRequest(fixture))
	if !result.Cached {
		t.Fatal("generating a sibling package invalidated the cache")
	}
	assertUntouched(t, written, result)
}
