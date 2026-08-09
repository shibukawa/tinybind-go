package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shibukawa/tinybind-go/parser"
)

// Generate analyzes dir and writes <outName> (default: tinybind_gen.go) into outDir
// (default: dir). Returns the absolute path of the written file.
func Generate(dir, outDir, outName string) (string, error) {
	return New(DefaultOptions()).Generate(dir, outDir, outName)
}

// Generator is a reusable, configurable code generator.
type Generator struct{ Options Options }

// New constructs a usage-directed generator. Set GenerateAll for legacy output.
func New(opts Options) *Generator { return &Generator{Options: opts} }

// Analyze analyzes a package using this generator's discovery symbols.
func (g *Generator) Analyze(dir string) (*PackagePlan, error) {
	return AnalyzePackageWithOptions(dir, g.Options)
}

// Generate analyzes dir and writes generated source.
func (g *Generator) Generate(dir, outDir, outName string) (string, error) {
	return g.generate(newPackageLoad(dir), outDir, outName)
}

// defaultTransportOut is the generated transport file. One file per package,
// not one per source: the transform closes over the call graph, so a handler
// and the helper it hands the request to may be authored apart.
const defaultTransportOut = "tinybind_transport_gen.go"

// generateTransport writes the other transport's copy of the package handlers.
// It writes nothing, and reports no error, when no backend is selected.
func (g *Generator) generateTransport(load *packageLoad, outDir, outName string) (string, error) {
	if g.Options.Transform == nil {
		return "", nil
	}
	artifacts, err := g.transportArtifacts(load)
	if err != nil || len(artifacts) == 0 {
		return "", err
	}
	if outDir == "" {
		outDir = load.dir
	}
	if outName == "" {
		outName = defaultTransportOut
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, outName)
	if err := os.WriteFile(path, artifacts[0].Content, 0o644); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

// generate is Generate over a package the run already loaded.
func (g *Generator) generate(load *packageLoad, outDir, outName string) (string, error) {
	dir := load.dir
	plan, err := analyzeLoadedPackage(load, g.Options)
	if err != nil {
		return "", err
	}
	if len(plan.Types) == 0 {
		// Wrapped so a caller generating over a whole set of packages can skip the
		// ones with nothing to bind, which is what the route tree of
		// routetree.Tree.Packages leaves it holding.
		return "", fmt.Errorf("%w: no generatable structs in %s", ErrNothingToGenerate, dir)
	}
	src, err := Emit(plan)
	if err != nil {
		return "", err
	}
	if outDir == "" {
		outDir = dir
	}
	if outName == "" {
		outName = "tinybind_gen.go"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, outName)
	if err := os.WriteFile(path, src, 0o644); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

// reportTransform lists what a transformed build would refuse, writing nothing
// and failing on nothing but a load error. The refusals are the report.
func (g *Generator) reportTransform(load *packageLoad) ([]parser.Diagnostic, error) {
	if g.Options.Transform == nil {
		return nil, nil
	}
	pkg, err := load.get()
	if err != nil {
		return nil, err
	}
	plan, err := AnalyzeTransform(pkg, *g.Options.Transform)
	if err != nil {
		return nil, err
	}
	return plan.Refusals.Diagnostics(), nil
}
