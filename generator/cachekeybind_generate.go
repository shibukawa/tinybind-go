package generator

import (
	"os"
	"path/filepath"
)

// defaultCacheKeyOut is the cache key output file.
const defaultCacheKeyOut = "cachekeybind_gen.go"

// GenerateCacheKeys analyzes dir and writes the cache key methods.
// It returns "" when the package uses no type as a cache key.
func (g *Generator) GenerateCacheKeys(dir, outDir, outName string) (string, error) {
	return g.generateCacheKeys(newPackageLoad(dir), outDir, outName)
}

// generateCacheKeys is GenerateCacheKeys over a package the run already loaded.
func (g *Generator) generateCacheKeys(load *packageLoad, outDir, outName string) (string, error) {
	if g.Options.featureDisabled(FeatureCacheKey) {
		return "", nil
	}
	plan, err := analyzeCacheKeys(load, g.Options)
	if err != nil {
		return "", err
	}
	if len(plan.Keys) == 0 {
		return "", nil
	}
	src, err := emitCacheKeysSelected(plan, nil)
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
		outName = defaultCacheKeyOut
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
