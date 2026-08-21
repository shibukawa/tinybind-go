package generator

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/tools/go/packages"

	"github.com/shibukawa/tinybind-go/parser"
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

	parseOnce sync.Once
	parsed    *parser.Result
	parseErr  error
}

func newPackageLoad(dir string) *packageLoad {
	return &packageLoad{dir: dir}
}

// get type-checks the directory on first use and replays that outcome, errors
// included, to every later phase. A load seeded from a PackageSet has its
// package already and type-checks nothing.
func (load *packageLoad) get() (*packages.Package, error) {
	load.once.Do(func() {
		if load.pkg != nil {
			return
		}
		load.pkg, load.err = loadPackage(load.dir)
	})
	return load.pkg, load.err
}

// routes parses the loaded package on first use and replays the result, errors
// included, to every later phase. The OpenAPI and transform phases and the
// route export all read routes from one run's options, whose normalization
// yields one parser config, which is what makes a single cached result correct
// for all of them.
func (load *packageLoad) routes(config parser.Config) (*parser.Result, error) {
	load.parseOnce.Do(func() {
		pkg, err := load.get()
		if err != nil {
			load.parseErr = err
			return
		}
		routeParseCount.Add(1)
		load.parsed, load.parseErr = parser.ParseLoadedPackage(pkg, config)
	})
	return load.parsed, load.parseErr
}

// packageLoadCount counts type checks so tests can prove that one run performs
// exactly one, which is the property the whole packageLoad seam exists for.
var packageLoadCount atomic.Int64

// routeParseCount counts route parses for the same proof: a run that reads
// routes from several phases still parses once.
var routeParseCount atomic.Int64

// loadMode is the union of what the analysis phases need, since they share one
// loaded package. NeedDeps is what makes a load expensive and what makes
// loading several directories together worth doing: the closure it checks is
// mostly the same closure for every directory of one project.
const loadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedSyntax |
	packages.NeedTypes |
	packages.NeedTypesInfo |
	packages.NeedImports |
	packages.NeedModule |
	packages.NeedDeps

// loadPackage type-checks the one Go package in dir.
func loadPackage(dir string) (*packages.Package, error) {
	packageLoadCount.Add(1)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	pkgs, err := packages.Load(&packages.Config{Mode: loadMode, Dir: abs}, ".")
	if err != nil {
		return nil, fmt.Errorf("packages.Load %s: %w", abs, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no package in %s", abs)
	}
	pkg := pkgs[0]
	for _, candidate := range pkgs {
		if analyzable(candidate) {
			pkg = candidate
			break
		}
	}
	if err := checkLoadedPackage(pkg, abs); err != nil {
		return nil, err
	}
	return pkg, nil
}

// analyzable reports whether a loaded root is the package to generate for
// rather than one of the test variants go list reports beside it.
func analyzable(pkg *packages.Package) bool {
	return pkg.Name != "" && !strings.HasSuffix(pkg.ID, ".test") && !strings.HasSuffix(pkg.Name, "_test")
}

// checkLoadedPackage reports why a loaded package cannot be analyzed, naming
// the directory it was asked for.
func checkLoadedPackage(pkg *packages.Package, abs string) error {
	if pkg.Types == nil || pkg.TypesInfo == nil {
		return fmt.Errorf("type-check failed for %s: %v", abs, pkg.Errors)
	}
	if missing := unresolvedImports(pkg); len(missing) > 0 {
		return fmt.Errorf("cannot analyze %s: no package was found for %s, so no call site can be discovered; run go mod tidy",
			abs, strings.Join(missing, ", "))
	}
	return nil
}

// unresolvedImports names the imports that a hand-written file asks for and
// that resolved to no package.
//
// A type error is not one of these and must not be treated as one: a package
// analyzed before its codec exists does not satisfy EntityEncoder yet, by
// design, and refusing that would refuse every first run. go/packages reports
// both as errors on the package, so the two are told apart here by whether the
// import produced a package at all rather than by what the error says.
//
// It is checked because the failure is otherwise silent and misreads as a
// missing feature. Discovery matches a call site through the resolved package
// of its callee, so when a runtime import resolves to nothing every pattern
// misses, and a phase that reads declarations from disk instead of from types
// keeps working. The result is a codec with the half that needed no types and
// nothing else, from a run that reported success.
//
// Generated files are not counted, for the same reason discovery does not read
// them. A codec pass writes the first import of a runtime package into a
// package that had none, so between that pass and the next the module can name
// a dependency its go.mod does not yet require. No call site of the caller's
// is behind that import, so there is nothing for the next pass to miss.
func unresolvedImports(pkg *packages.Package) []string {
	// An import that resolved carries its package name; one that did not is the
	// empty name go/types reports as `invalid package name: ""`.
	missing := map[string]bool{}
	for path, imported := range pkg.Imports {
		if imported == nil || imported.Name == "" {
			missing[path] = true
		}
	}
	if len(missing) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range pkg.Syntax {
		if f == nil || ast.IsGenerated(f) {
			continue
		}
		for _, spec := range f.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || !missing[path] || seen[path] {
				continue
			}
			seen[path] = true
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}
