package generator_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

const assetTemplate = `package fixture

export component Card(label: string): html {
<head>
<style>.card { color: red }</style>
<script defer>card()</script>
<script src="https://cdn.example.com/lib.js"></script>
</head>
<p class="card">{label}</p>
}
`

func writeAssetFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		"card.tb.html": assetTemplate,
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

// TestGenerateTemplatesWritesPublicAssets covers the emission requirement: the
// generator writes extracted files exactly as it writes Go artifacts.
func TestGenerateTemplatesWritesPublicAssets(t *testing.T) {
	dir := writeAssetFixture(t)
	publicDir := t.TempDir()
	options := generator.DefaultOptions()
	options.PublicDir = publicDir
	options.PublicURLBase = "/static/gen"

	result, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AssetPaths) != 2 {
		t.Fatalf("want a stylesheet and a script, got %v", result.AssetPaths)
	}
	kinds := map[string]string{}
	for _, path := range result.AssetPaths {
		if filepath.Dir(path) != publicDir {
			t.Fatalf("asset %q escaped the public directory %q", path, publicDir)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		kinds[filepath.Ext(path)] = string(content)
	}
	if !strings.Contains(kinds[".css"], "color: red") {
		t.Fatalf("stylesheet lost its rules: %q", kinds[".css"])
	}
	if !strings.Contains(kinds[".js"], "card()") {
		t.Fatalf("script lost its body: %q", kinds[".js"])
	}

	generated, err := os.ReadFile(result.TemplatesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `/static/gen/card.style.`) {
		t.Fatalf("generated head does not reference the stylesheet:\n%s", generated)
	}
	if !strings.Contains(string(generated), `/static/gen/card.script.`) {
		t.Fatalf("generated head does not reference the script:\n%s", generated)
	}
	if strings.Contains(string(generated), "color: red") {
		t.Fatalf("style block stayed inline:\n%s", generated)
	}
}

// TestPublicAssetDefaultsNeedNoConfiguration covers the zero-configuration
// acceptance case: files land in public/generated and URLs start with
// /public/generated.
func TestPublicAssetDefaultsNeedNoConfiguration(t *testing.T) {
	dir := writeAssetFixture(t)
	// The default directory is relative, so the run happens inside a scratch
	// working directory rather than the repository.
	restore, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(restore) })

	options := generator.DefaultOptions()
	options.PublicDir, options.PublicURLBase = "", ""
	result, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(work, generator.DefaultPublicDir))
	if err != nil {
		t.Fatalf("default public directory was not written: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want two generated assets in %s, got %d", generator.DefaultPublicDir, len(entries))
	}
	generated, err := os.ReadFile(result.TemplatesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), generator.DefaultPublicURLBase+"/card.style.") {
		t.Fatalf("generated head does not use the default URL base:\n%s", generated)
	}
}

// TestPublicAssetOptionsArePaired covers the actionable failure when only one
// of the two independent options is configured.
func TestPublicAssetOptionsArePaired(t *testing.T) {
	dir := writeAssetFixture(t)
	cases := []struct {
		name    string
		mutate  func(*generator.Options)
		request generator.GenerateRequest
	}{
		{
			name:   "only a directory",
			mutate: func(o *generator.Options) { o.PublicDir, o.PublicURLBase = t.TempDir(), "" },
		},
		{
			name:   "only a URL base",
			mutate: func(o *generator.Options) { o.PublicDir, o.PublicURLBase = "", "/static" },
		},
		{
			name:    "only a requested URL base",
			mutate:  func(o *generator.Options) {},
			request: generator.GenerateRequest{PublicURLBase: "/static"},
		},
		{
			name:    "only a requested directory",
			mutate:  func(o *generator.Options) {},
			request: generator.GenerateRequest{PublicDir: "assets"},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			options := generator.DefaultOptions()
			test.mutate(&options)
			request := test.request
			request.Dir = dir
			request.Out = dir
			_, err := generator.New(options).GenerateArtifacts(context.Background(), request)
			if !errors.Is(err, generator.ErrPublicAssetPairing) {
				t.Fatalf("error = %v, want ErrPublicAssetPairing", err)
			}
			for _, want := range []string{"PublicDir", "PublicURLBase", generator.DefaultPublicDir} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("message %q does not mention %q", err, want)
				}
			}
		})
	}
}

// TestPublicAssetArtifactsCarryTheirDestination covers data:generation-artifact:
// an extracted file is a public asset, not Go source.
func TestPublicAssetArtifactsCarryTheirDestination(t *testing.T) {
	dir := writeAssetFixture(t)
	options := generator.DefaultOptions()
	options.PublicDir, options.PublicURLBase = t.TempDir(), "https://cdn.example.com/assets"

	artifacts, err := generator.New(options).GenerateArtifacts(context.Background(), generator.GenerateRequest{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	var assets []generator.Artifact
	for _, artifact := range artifacts {
		if artifact.Destination == generator.DestinationPublicAsset {
			assets = append(assets, artifact)
		}
	}
	if len(assets) != 2 {
		t.Fatalf("want two public assets, got %d of %d artifacts", len(assets), len(artifacts))
	}
	wantKinds := map[generator.ArtifactKind]string{
		generator.ArtifactStylesheet: generator.ExtensionCSS,
		generator.ArtifactScript:     generator.ExtensionJS,
	}
	for _, asset := range assets {
		extension, known := wantKinds[asset.Kind]
		if !known {
			t.Fatalf("unexpected asset kind %q", asset.Kind)
		}
		if asset.Extension != extension {
			t.Fatalf("%s extension = %q, want %q", asset.Kind, asset.Extension, extension)
		}
		if filepath.Base(asset.SourcePath) != "card.tb.html" {
			t.Fatalf("%s owner = %q", asset.Kind, asset.SourcePath)
		}
		// A full URL base emits that host and changes nothing else.
		want := "https://cdn.example.com/assets/" + asset.OutputBase + "." + asset.Extension
		if asset.PublicPath != want {
			t.Fatalf("%s public path = %q, want %q", asset.Kind, asset.PublicPath, want)
		}
		if asset.PackageName != "" {
			t.Fatalf("%s carries a Go package name %q", asset.Kind, asset.PackageName)
		}
	}
}

// TestPublicAssetWritesAreStableAcrossRuns covers cache validity end to end.
func TestPublicAssetWritesAreStableAcrossRuns(t *testing.T) {
	dir := writeAssetFixture(t)
	options := generator.DefaultOptions()
	options.PublicDir, options.PublicURLBase = t.TempDir(), "/static"
	runner := generator.New(options)
	request := generator.GenerateRequest{Dir: dir, Out: dir}

	first, err := runner.GeneratePackage(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runner.GeneratePackage(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(first.AssetPaths, ",") != strings.Join(second.AssetPaths, ",") {
		t.Fatalf("asset names changed: %v then %v", first.AssetPaths, second.AssetPaths)
	}
	entries, err := os.ReadDir(options.PublicDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("regeneration left %d files, want 2", len(entries))
	}
}
