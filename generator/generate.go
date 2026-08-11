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
func (g *Generator) generateTransport(load *packageLoad, outDir, outName string) (string, []string, error) {
	if g.Options.Transform == nil {
		return "", nil, nil
	}
	artifacts, warnings, err := g.transportArtifacts(load)
	if err != nil || len(artifacts) == 0 {
		return "", warnings, err
	}
	if outDir == "" {
		outDir = load.dir
	}
	if outName == "" {
		outName = defaultTransportOut
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", warnings, err
	}
	path := filepath.Join(outDir, outName)
	if err := os.WriteFile(path, artifacts[0].Content, 0o644); err != nil {
		return "", warnings, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, warnings, nil
	}
	return abs, warnings, nil
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
	// When a backend is selected the two binder sets register the same types
	// against two runtimes, so they must never compile together.
	target := netHTTPTarget()
	if g.Options.Transform != nil {
		target.buildTag = "!fasthttp"
	}
	src, err := emitSelectedFor(plan, nil, target)
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

// defaultFastBindersOut is the derived backend's binder and writer file.
const defaultFastBindersOut = "tinybind_fasthttp_gen.go"

// generateFastBinders writes the selected backend's binders and writers. The
// transport-free half of the file is emitted into both copies rather than
// split out: only one ever compiles, so duplicating it on disk costs nothing
// and keeps each file self-contained.
func (g *Generator) generateFastBinders(load *packageLoad, outDir, outName string) (string, error) {
	if g.Options.Transform == nil {
		return "", nil
	}
	plan, err := analyzeLoadedPackage(load, g.Options)
	if err != nil {
		return "", err
	}
	if len(plan.Types) == 0 {
		return "", nil
	}
	src, err := emitSelectedFor(plan, nil, fasthttpTarget())
	if err != nil {
		return "", err
	}
	if outDir == "" {
		outDir = load.dir
	}
	if outName == "" {
		outName = defaultFastBindersOut
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

// defaultRoutesOut is the derived backend's route registration.
const defaultRoutesOut = "tinybind_routes_gen.go"

// generateRoutes writes the function installing the discovered routes on the
// caller's router. fasthttp has no router of its own, so the application picks
// one and the generated code names whichever TransformOptions.Router says.
func (g *Generator) generateRoutes(load *packageLoad, outDir, outName string) (string, error) {
	if g.Options.Transform == nil {
		return "", nil
	}
	transform, err := g.Options.Transform.normalized()
	if err != nil {
		return "", err
	}
	pkg, err := load.get()
	if err != nil {
		return "", err
	}
	normalized, err := g.Options.normalized()
	if err != nil {
		return "", err
	}
	result, err := parser.ParseLoadedPackage(pkg, normalized.parserConfig)
	if err != nil {
		return "", err
	}
	src, err := emitRouteRegistration(pkg, result.Routes, transform.Router, fasthttpTarget())
	if err != nil || src == nil {
		return "", err
	}
	if outDir == "" {
		outDir = load.dir
	}
	if outName == "" {
		outName = defaultRoutesOut
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
	plan, err := AnalyzeTransform(pkg, g.transformOptions())
	if err != nil {
		return nil, err
	}
	return plan.Refusals.Diagnostics(), nil
}
