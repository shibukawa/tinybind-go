package generator

import (
	"os"
	"path/filepath"
)

// defaultFirestoreOut is the Firestore entity codec output file.
const defaultFirestoreOut = "firestorebind_gen.go"

// GenerateFirestoreEntities analyzes dir and writes the Firestore entity codec.
// It returns "" when the package binds no type to Firestore.
func (g *Generator) GenerateFirestoreEntities(dir, outDir, outName string) (string, error) {
	return g.generateFirestoreEntities(newPackageLoad(dir), outDir, outName)
}

// generateFirestoreEntities is GenerateFirestoreEntities over a package the run
// already loaded.
func (g *Generator) generateFirestoreEntities(load *packageLoad, outDir, outName string) (string, error) {
	if g.Options.featureDisabled(FeatureEntityCodec) {
		return "", nil
	}
	plan, err := analyzeFirestoreEntities(load, g.Options)
	if err != nil {
		return "", err
	}
	if len(plan.Entities) == 0 {
		return "", nil
	}
	src, err := emitFirestoreSelected(plan, nil)
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
		outName = defaultFirestoreOut
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
