package generator_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// readUnstamped reads a generated file without its provenance line, leaving
// the generated code alone to compare.
func readUnstamped(t *testing.T, path string) string {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(string(source), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "// tinybind:generated ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// A second run reads whatever the first one wrote. The generated binder reads
// the body lazily, so it captures the request in a closure -- the one shape the
// eligibility rule refuses. Analyzing it as authored source made the transform
// refuse its own output, and the refusal stopped every later phase.
func TestTransformIgnoresItsOwnGeneratedBinder(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("models.go", `package fixture

type Ask struct {
	Name string `+"`json:\"name\" check:\"required\"`"+`
}

type Reply struct {
	Message string `+"`json:\"message\"`"+`
}
`)
	write("handlers.go", `//go:build !fasthttp

package fixture

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

func Greet(w http.ResponseWriter, r *http.Request) {
	ask, err := httpbind.Bind[Ask](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	httpbind.Write(w, r, Reply{Message: ask.Name})
}
`)
	tidyTempModule(t, dir)

	options := generator.DefaultOptions()
	transform := generator.DefaultTransformOptions()
	options.Transform = &transform

	run := func(out string) generator.GenerateResult {
		t.Helper()
		result, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{
			Dir: dir, Out: out, Force: true,
		})
		if err != nil {
			t.Fatalf("generate into %s: %v", out, err)
		}
		return result
	}

	first := run(t.TempDir())
	if first.FastBindersPath == "" {
		t.Fatal("first run wrote no fasthttp binders")
	}
	// Put the first run's binder beside the sources under the name it would
	// have taken there, which is the state an Out-less second run starts from.
	binder, err := os.ReadFile(first.BinderPath)
	if err != nil {
		t.Fatalf("read binder: %v", err)
	}
	write(filepath.Base(first.BinderPath), string(binder))

	second := run(t.TempDir())
	if second.FastBindersPath == "" {
		t.Error("second run wrote no fasthttp binders")
	}
	if second.TransportPath == "" {
		t.Error("second run wrote no transport file")
	}
	// Skipping the file is only half of it: the second run has to derive the
	// same handlers from the same authored source as the first. The stamp is
	// dropped before comparing, because its input hash covers the package
	// directory and the second run's directory really does hold another file.
	for _, pair := range []struct {
		name        string
		first, next string
	}{
		{"transport", first.TransportPath, second.TransportPath},
		{"fasthttp binders", first.FastBindersPath, second.FastBindersPath},
		{"routes", first.RoutesPath, second.RoutesPath},
	} {
		if pair.first == "" || pair.next == "" {
			continue
		}
		before, after := readUnstamped(t, pair.first), readUnstamped(t, pair.next)
		if before != after {
			t.Errorf("%s differs between runs", pair.name)
		}
	}
	// The refused binder is the one the transform used to choke on, so the
	// registry it fills is what proves the fasthttp build would answer. An
	// unregistered type compiles and fails on the first request instead, which
	// is how this surfaced in the first place.
	binders, err := os.ReadFile(second.FastBindersPath)
	if err != nil {
		t.Fatalf("read fasthttp binders: %v", err)
	}
	if !strings.Contains(string(binders), "RegisterBind") {
		t.Error("fasthttp binders registered no binder")
	}

	// Place the second run's whole output and build both backends from it.
	for _, path := range []string{
		second.BinderPath, second.TransportPath, second.FastBindersPath, second.RoutesPath,
	} {
		if path == "" {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		write(filepath.Base(path), string(source))
	}
	tidyTempModule(t, dir)
	for _, tags := range []string{"fasthttp", ""} {
		args := []string{"build", "-o", os.DevNull}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		cmd := exec.Command("go", append(args, ".")...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("build with tags %q failed: %v\n%s", tags, err, output)
		}
	}
}

// A framework generating through the artifact API gets no file-writing entry
// point to borrow, so the artifacts have to describe the whole derived backend.
func TestArtifactsCarryTheDerivedBackend(t *testing.T) {
	options := generator.DefaultOptions()
	transform := generator.DefaultTransformOptions()
	options.Transform = &transform
	artifacts, err := generator.New(options).GenerateArtifacts(context.Background(),
		generator.GenerateRequest{Dir: filepath.Join("..", "testdata", "transform_rewrite")})
	if err != nil {
		t.Fatalf("generate artifacts: %v", err)
	}
	byKind := map[generator.ArtifactKind][]generator.Artifact{}
	for _, artifact := range artifacts {
		byKind[artifact.Kind] = append(byKind[artifact.Kind], artifact)
	}
	for _, kind := range []generator.ArtifactKind{
		generator.ArtifactBinding,
		generator.ArtifactTransport,
		generator.ArtifactTransportBinding,
		generator.ArtifactTransportRoutes,
	} {
		if len(byKind[kind]) == 0 {
			t.Errorf("no %s artifact", kind)
		}
	}
	for _, artifact := range byKind[generator.ArtifactTransportBinding] {
		if !strings.Contains(string(artifact.Content), "//go:build fasthttp") {
			t.Error("transport binding is not behind the backend tag")
		}
		if !strings.Contains(string(artifact.Content), "RegisterBind") {
			t.Error("transport binding registered no binder")
		}
	}
}
