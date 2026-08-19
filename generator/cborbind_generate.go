package generator

import (
	"os"
	"path/filepath"
)

// defaultCborOut is the CBOR codec output file.
const defaultCborOut = "cborbind_gen.go"

// GenerateCborCodecs analyzes dir and writes the CBOR codecs.
// It returns "" when the package declares no CBOR codec.
func (g *Generator) GenerateCborCodecs(dir, outDir, outName string) (string, error) {
	return g.generateCborCodecs(newPackageLoad(dir), outDir, outName)
}

// generateCborCodecs is GenerateCborCodecs over a package the run already
// loaded.
func (g *Generator) generateCborCodecs(load *packageLoad, outDir, outName string) (string, error) {
	if g.Options.featureDisabled(FeatureCBORWireCodec) && g.Options.featureDisabled(FeatureCBORWorldCodec) {
		return "", nil
	}
	plan, err := analyzeCborCodecs(load, g.Options)
	if err != nil {
		return "", err
	}
	if len(plan.Types) == 0 {
		return "", nil
	}
	src, err := EmitCborCodecs(plan)
	if err != nil {
		return "", err
	}
	if len(src) == 0 {
		return "", nil
	}
	if outDir == "" {
		outDir = load.dir
	}
	if outName == "" {
		outName = defaultCborOut
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
