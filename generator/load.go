package generator

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/tools/go/packages"
)

// Type checking is what generation actually spends its time on, and the binder,
// configbind, route and OpenAPI phases all need the same type-checked package.
// A run therefore threads one packageLoad through its phases so the package is
// loaded at most once, however many phases are enabled.
type packageLoad struct {
	dir  string
	once sync.Once
	pkg  *packages.Package
	err  error
}

func newPackageLoad(dir string) *packageLoad {
	return &packageLoad{dir: dir}
}

// get type-checks the directory on first use and replays that outcome, errors
// included, to every later phase.
func (load *packageLoad) get() (*packages.Package, error) {
	load.once.Do(func() {
		load.pkg, load.err = loadPackage(load.dir)
	})
	return load.pkg, load.err
}

// packageLoadCount counts type checks so tests can prove that one run performs
// exactly one, which is the property the whole packageLoad seam exists for.
var packageLoadCount atomic.Int64

// loadPackage type-checks the one Go package in dir. The mode is the union of
// what the analysis phases need, since they share the result.
func loadPackage(dir string) (*packages.Package, error) {
	packageLoadCount.Add(1)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedModule |
			packages.NeedDeps,
		Dir: abs,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("packages.Load %s: %w", abs, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no package in %s", abs)
	}
	pkg := pkgs[0]
	for _, candidate := range pkgs {
		if candidate.Name != "" && !strings.HasSuffix(candidate.ID, ".test") && !strings.HasSuffix(candidate.Name, "_test") {
			pkg = candidate
			break
		}
	}
	if pkg.Types == nil || pkg.TypesInfo == nil {
		return nil, fmt.Errorf("type-check failed for %s: %v", abs, pkg.Errors)
	}
	return pkg, nil
}
