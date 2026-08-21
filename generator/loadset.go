package generator

import (
	"context"
	"fmt"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

// Loading is nearly all of what generating one directory costs, and a caller
// generating a tree of them pays it per directory. Most of what it pays for is
// the same work each time: loadMode checks the whole dependency closure, and
// the closure the packages of one project share is the framework they all
// import. Twelve directories therefore type-check that framework twelve times.
//
// Running the directories at once does not remove the duplication - it only
// overlaps it, and go list serializes behind the build cache besides. Loading
// them together does remove it: the shared closure is checked once and every
// directory reads its own package out of the result.

// PackageSet holds several directories type-checked in one go/packages call.
//
// It is read-only once returned, so the directories it covers may generate
// concurrently, which is the pairing it exists for: one load, then whatever
// concurrency the caller wants over the generation that follows.
//
// A set describes the tree as it was when the set was built, and nothing here
// is incremental. A caller that writes generated files and then generates
// directories which type-check them must build a new set in between - which is
// why a run staged around what it writes builds one set per stage rather than
// one per run.
type PackageSet struct {
	// byDir is keyed by the absolute directory the caller asked for, so a
	// lookup by the same directory finds it however that path was spelled.
	byDir map[string]*packages.Package
}

// LoadPackages type-checks dirs, and the dependency closure they share, in one
// go/packages call.
//
// The result covers every directory that resolved to one analyzable package. A
// directory that did not is left out rather than reported, because leaving it
// out is what preserves the diagnostic: its generation loads it alone and fails
// the way it fails today, naming that directory instead of the batch. The same
// goes for a directory outside the module the others are in, which one load
// cannot reach.
//
// An error is returned only when the load itself could not run.
func LoadPackages(ctx context.Context, dirs []string) (*PackageSet, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	set := &PackageSet{byDir: map[string]*packages.Package{}}
	// A directory reached through a symlink is spelled one way by the caller
	// and another by go list, so the two are matched on the resolved path and
	// the result is stored under what the caller will look it up by.
	requested := map[string][]string{}
	patterns := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		resolved := resolvePath(abs)
		if _, seen := requested[resolved]; !seen {
			patterns = append(patterns, abs)
		}
		requested[resolved] = append(requested[resolved], abs)
	}
	if len(patterns) == 0 {
		return set, nil
	}
	// go list runs in the module root rather than in one of the directories,
	// so every pattern is inside the module it resolves against whichever
	// directory happens to be first.
	base, _ := modulePosition(patterns[0])
	if base == "" {
		base = patterns[0]
	}
	packageLoadCount.Add(1)
	pkgs, err := packages.Load(&packages.Config{Mode: loadMode, Dir: base, Context: ctx}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("packages.Load %d directories under %s: %w", len(patterns), base, err)
	}
	for _, pkg := range pkgs {
		if !analyzable(pkg) {
			continue
		}
		dir, ok := packageDir(pkg)
		if !ok {
			continue
		}
		asked, ok := requested[resolvePath(dir)]
		if !ok {
			continue
		}
		if err := checkLoadedPackage(pkg, dir); err != nil {
			continue
		}
		for _, name := range asked {
			if _, taken := set.byDir[name]; !taken {
				set.byDir[name] = pkg
			}
		}
	}
	return set, nil
}

// Len reports how many directories the set covers, so a caller can log what a
// load reached without reaching into it.
func (set *PackageSet) Len() int {
	if set == nil {
		return 0
	}
	return len(set.byDir)
}

// loadFor is the package load one directory's generation reads.
//
// A directory the set covers is served from it and type-checks nothing. A
// directory it does not - a nil set, a tree that changed, a package the load
// could not use - loads on its own, which is both the fallback and the whole of
// the behavior before there was a set.
func (set *PackageSet) loadFor(dir string) *packageLoad {
	if set != nil {
		if abs, err := filepath.Abs(dir); err == nil {
			if pkg, ok := set.byDir[abs]; ok {
				return &packageLoad{dir: dir, pkg: pkg}
			}
		}
	}
	return newPackageLoad(dir)
}

// packageDir is the directory a loaded package was read from. go/packages
// reports the files rather than the directory, and a package with no file at
// all - which is what an empty directory loads as - has no directory to report.
func packageDir(pkg *packages.Package) (string, bool) {
	for _, group := range [][]string{pkg.GoFiles, pkg.CompiledGoFiles, pkg.OtherFiles, pkg.IgnoredFiles} {
		for _, file := range group {
			if dir := filepath.Dir(file); filepath.IsAbs(dir) {
				return dir, true
			}
		}
	}
	return "", false
}

// resolvePath is path with its symlinks resolved, or path when they cannot be,
// which is only used to decide whether two spellings name one directory.
func resolvePath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
