package generator

import (
	"fmt"
	"os"
	"path/filepath"
)

// Generate analyzes dir and writes <outName> (default: httpbinder_gen.go) into outDir
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
	plan, err := g.Analyze(dir)
	if err != nil {
		return "", err
	}
	if len(plan.Types) == 0 {
		return "", fmt.Errorf("no generatable structs in %s", dir)
	}
	src, err := Emit(plan)
	if err != nil {
		return "", err
	}
	if outDir == "" {
		outDir = dir
	}
	if outName == "" {
		outName = "httpbinder_gen.go"
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
