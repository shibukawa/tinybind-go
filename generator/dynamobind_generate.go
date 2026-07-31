package generator

import (
	"os"
	"path/filepath"
)

// defaultDynamoOut is the DynamoDB item codec output file.
const defaultDynamoOut = "dynamobind_gen.go"

// GenerateDynamoItems analyzes dir and writes the DynamoDB item codec.
// It returns "" when the package binds no type to DynamoDB.
func (g *Generator) GenerateDynamoItems(dir, outDir, outName string) (string, error) {
	return g.generateDynamoItems(newPackageLoad(dir), outDir, outName)
}

// generateDynamoItems is GenerateDynamoItems over a package the run already
// loaded.
func (g *Generator) generateDynamoItems(load *packageLoad, outDir, outName string) (string, error) {
	if g.Options.featureDisabled(FeatureItemCodec) {
		return "", nil
	}
	plan, err := analyzeDynamoItems(load, g.Options)
	if err != nil {
		return "", err
	}
	if len(plan.Items) == 0 {
		return "", nil
	}
	src, err := emitDynamoSelected(plan, nil, !g.Options.featureDisabled(FeatureItemTable))
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
		outName = defaultDynamoOut
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
