// Package routetree discovers a filesystem route tree and derives the stdlib
// ServeMux patterns, ancestor layout chains, and Go package names for it.
//
// The tree is an opt-in directory (conventionally pages) whose subdirectories are
// both URL segments and Go packages. A directory name carries its own segment
// kind: a trailing underscore marks one dynamic segment, and two mark a
// catch-all. Bracketed spellings are impossible because a directory holding Go
// source must be a legal import path element; see the package documentation of
// [ValidateDirName] for what the toolchain accepts.
package routetree

import (
	"errors"
	"fmt"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Default file names for the reserved roles of a route directory.
const (
	DefaultPageFile     = "page.tb.html"
	DefaultLayoutFile   = "layout.tb.html"
	DefaultDocumentFile = "document.tb.html"
	DefaultLogicFile    = "page.go"
	DefaultRootDir      = "pages"
)

// SegmentKind classifies how a directory contributes to the URL.
type SegmentKind uint8

const (
	// StaticSegment contributes the directory name literally.
	StaticSegment SegmentKind = iota
	// DynamicSegment binds one path element to a named parameter.
	DynamicSegment
	// CatchAllSegment binds the remainder of the path to a named parameter.
	CatchAllSegment
)

func (k SegmentKind) String() string {
	switch k {
	case DynamicSegment:
		return "dynamic"
	case CatchAllSegment:
		return "catch-all"
	default:
		return "static"
	}
}

// Segment is one directory of the route tree.
type Segment struct {
	// Dir is the directory name exactly as it appears on disk.
	Dir string
	// Name is the parameter name for a dynamic or catch-all segment, and the
	// literal URL segment for a static one.
	Name string
	Kind SegmentKind
}

// Layout is one ancestor wrapper contributing to a page.
type Layout struct {
	// RelDir is the layout directory relative to the route root, using slashes.
	// The root layout has an empty RelDir.
	RelDir string
	// File is the absolute path of the layout template.
	File string
	// Package is the Go package name of the layout directory.
	Package string
	// ImportPath is the Go import path of the layout directory. It is empty
	// unless Config.ImportBase was set.
	ImportPath string
	// Params are the dynamic segments in scope at this level, outermost first.
	// A layout may only depend on segments at or above its own directory.
	Params []Segment
}

// Route is one discovered page.
type Route struct {
	// RelDir is the page directory relative to the route root, using slashes.
	// The root page has an empty RelDir.
	RelDir string
	// Dir is the absolute page directory.
	Dir string
	// PageFile is the absolute path of the page template.
	PageFile string
	// LogicFile is the absolute path of page.go, or empty when absent.
	LogicFile string
	// Package is the Go package name for the page directory.
	Package string
	// ImportPath is the Go import path of the page directory. It is empty
	// unless Config.ImportBase was set.
	ImportPath string
	// Path is the URL path pattern, such as /users/{id}.
	Path string
	// Segments are every directory from the route root to the page directory.
	Segments []Segment
	// Params are the dynamic and catch-all segments only, in route order.
	Params []Segment
	// Layouts are the ancestor layouts, outermost first.
	Layouts []Layout
}

// Pattern returns the stdlib ServeMux pattern for the page, which is always a
// GET route; see decision:route-handler-shape.
//
// The root page registers as /{$} rather than /, because a bare / is a prefix
// pattern in the standard library and would answer every unmatched path
// instead of letting it be a 404.
func (r Route) Pattern() string {
	if r.Path == "/" {
		return "GET /{$}"
	}
	return "GET " + r.Path
}

// Tree is a discovered route tree.
type Tree struct {
	// Root is the absolute route root directory.
	Root string
	// ImportBase is the Go import path of Root, copied from the Config so a
	// caller reading the tree back needs no second source for it. It is empty
	// unless Config.ImportBase was set.
	ImportBase string
	// DocumentFile is the absolute path of the root document shell, or empty.
	DocumentFile string
	// Routes are the discovered pages, ordered by path.
	Routes []Route
}

// Package is one Go package a route tree contains.
type Package struct {
	// RelDir is the directory relative to the route root, using slashes. The
	// route root package itself has an empty RelDir.
	RelDir string
	// Dir is the absolute directory.
	Dir string
	// Name is the Go package name.
	Name string
	// ImportPath is the Go import path. It is empty unless Config.ImportBase was
	// set.
	ImportPath string
}

// Packages lists every Go package the tree contains: the route root, every route
// directory, and every layout directory. The root comes first and the rest are
// ordered by directory.
//
// It is what a caller runs the binder generator over, which is what makes
// httpbind.Bind work inside a page or a server action. A binder is generated per
// package from the Bind call sites inside it, so a route package nobody analyzes
// has nothing to dispatch through at runtime. Run it after the tree's own
// generated files are on disk, because analysis type-checks the package.
//
// Doing so puts no page route and no action endpoint into an OpenAPI document:
// the only registrations are in the generated registry, and discovery skips what
// tinybind generated.
func (t *Tree) Packages() []Package {
	seen := map[string]bool{}
	out := []Package{{
		Dir:        t.Root,
		Name:       PackageName(filepath.Base(t.Root)),
		ImportPath: t.ImportBase,
	}}
	seen[t.Root] = true
	add := func(pkg Package) {
		if seen[pkg.Dir] {
			return
		}
		seen[pkg.Dir] = true
		out = append(out, pkg)
	}
	for _, route := range t.Routes {
		for _, layout := range route.Layouts {
			add(Package{
				RelDir:     layout.RelDir,
				Dir:        filepath.Dir(layout.File),
				Name:       layout.Package,
				ImportPath: layout.ImportPath,
			})
		}
		add(Package{
			RelDir:     route.RelDir,
			Dir:        route.Dir,
			Name:       route.Package,
			ImportPath: route.ImportPath,
		})
	}
	rest := out[1:]
	sort.Slice(rest, func(i, j int) bool { return rest[i].Dir < rest[j].Dir })
	return out
}

// Config selects the route root and the reserved file names. A zero Config
// uses the defaults.
type Config struct {
	// Root is the route root directory. Empty uses DefaultRootDir relative to
	// the working directory.
	Root string
	// ImportBase is the Go import path of the route root directory. Setting it
	// lets discovery compute an ImportPath for every route and layout, which is
	// what a generated registry needs to import them. It cannot be derived,
	// because the route root's position inside its module is not visible from
	// the directory alone.
	ImportBase string
	// PageFile, LayoutFile, DocumentFile, and LogicFile override the reserved
	// names. Empty values use the defaults.
	PageFile     string
	LayoutFile   string
	DocumentFile string
	LogicFile    string
}

func (c Config) withDefaults() Config {
	if c.Root == "" {
		c.Root = DefaultRootDir
	}
	if c.PageFile == "" {
		c.PageFile = DefaultPageFile
	}
	if c.LayoutFile == "" {
		c.LayoutFile = DefaultLayoutFile
	}
	if c.DocumentFile == "" {
		c.DocumentFile = DefaultDocumentFile
	}
	if c.LogicFile == "" {
		c.LogicFile = DefaultLogicFile
	}
	return c
}

// Error is one discovery failure, anchored at the file or directory that
// caused it.
type Error struct {
	// Path is the offending file or directory.
	Path string
	// Message states what is wrong and, where possible, what to do instead.
	Message string
}

func (e *Error) Error() string { return e.Path + ": " + e.Message }

// Discover walks the configured route root and returns its routes. Every
// problem it finds is reported; the returned error joins them so one run
// surfaces more than the first mistake. A non-nil Tree is still returned
// alongside errors so a caller may report and continue.
func Discover(cfg Config) (*Tree, error) {
	cfg = cfg.withDefaults()
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &Error{Path: root, Message: "route root is not a directory"}
	}

	tree := &Tree{Root: root, ImportBase: cfg.ImportBase}
	if documentPath := filepath.Join(root, cfg.DocumentFile); fileExists(documentPath) {
		tree.DocumentFile = documentPath
	}

	walker := &walker{cfg: cfg, root: root, tree: tree}
	walker.walk(root, "", nil, nil)
	walker.checkDuplicates()

	sort.Slice(tree.Routes, func(i, j int) bool { return tree.Routes[i].Path < tree.Routes[j].Path })
	if len(walker.errs) > 0 {
		return tree, errors.Join(walker.errs...)
	}
	return tree, nil
}

type walker struct {
	cfg  Config
	root string
	tree *Tree
	errs []error
}

func (w *walker) fail(path, format string, args ...any) {
	w.errs = append(w.errs, &Error{Path: path, Message: fmt.Sprintf(format, args...)})
}

// walk visits one directory. segments and layouts carry the chain accumulated
// from the route root, and both are copied before being extended so sibling
// branches never share backing arrays.
func (w *walker) walk(dir, relDir string, segments []Segment, layouts []Layout) {
	if layoutPath := filepath.Join(dir, w.cfg.LayoutFile); fileExists(layoutPath) {
		layouts = appendCopy(layouts, Layout{
			RelDir:     relDir,
			File:       layoutPath,
			Package:    PackageName(dirBase(dir, w.root)),
			ImportPath: joinImport(w.cfg.ImportBase, relDir),
			Params:     params(segments),
		})
	}

	if pagePath := filepath.Join(dir, w.cfg.PageFile); fileExists(pagePath) {
		route := Route{
			RelDir:     relDir,
			Dir:        dir,
			PageFile:   pagePath,
			Package:    PackageName(dirBase(dir, w.root)),
			ImportPath: joinImport(w.cfg.ImportBase, relDir),
			Path:       urlPath(segments),
			Segments:   segments,
			Params:     params(segments),
			Layouts:    layouts,
		}
		if logicPath := filepath.Join(dir, w.cfg.LogicFile); fileExists(logicPath) {
			route.LogicFile = logicPath
		}
		w.tree.Routes = append(w.tree.Routes, route)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		w.errs = append(w.errs, err)
		return
	}

	// A catch-all consumes the rest of the path, so it can have no children and
	// no siblings deeper in the tree.
	if last, ok := lastSegment(segments); ok && last.Kind == CatchAllSegment {
		for _, entry := range entries {
			if entry.IsDir() && !ignoredDir(entry.Name()) {
				w.fail(filepath.Join(dir, entry.Name()),
					"a catch-all segment consumes the rest of the path, so %q can hold no route subdirectories", last.Dir)
			}
		}
		return
	}

	dynamicSeen := ""
	for _, entry := range entries {
		if !entry.IsDir() || ignoredDir(entry.Name()) {
			continue
		}
		name := entry.Name()
		childDir := filepath.Join(dir, name)
		if err := ValidateDirName(name); err != nil {
			w.fail(childDir, "%s", err.Error())
			continue
		}
		segment := ParseSegment(name)
		if segment.Kind != StaticSegment {
			if !token.IsIdentifier(segment.Name) {
				w.fail(childDir, "parameter name %q is not a Go identifier", segment.Name)
				continue
			}
			if dynamicSeen != "" {
				w.fail(childDir,
					"directory level already has the dynamic sibling %q; two dynamic siblings cannot be told apart", dynamicSeen)
				continue
			}
			dynamicSeen = name
			if dup, ok := duplicateParam(segments, segment.Name); ok {
				w.fail(childDir, "parameter name %q is already bound by %q in this route", segment.Name, dup)
				continue
			}
		}
		w.walk(childDir, joinRel(relDir, name), appendCopy(segments, segment), layouts)
	}
}

func (w *walker) checkDuplicates() {
	seen := make(map[string]string, len(w.tree.Routes))
	for _, route := range w.tree.Routes {
		// Two routes collide when their patterns match the same requests, which
		// depends on segment kinds rather than on parameter names.
		key := normalizedPath(route.Segments)
		if first, ok := seen[key]; ok {
			w.fail(route.PageFile, "route %s duplicates the one declared by %s", route.Path, first)
			continue
		}
		seen[key] = route.PageFile
	}
}

// ParseSegment reads the segment kind and parameter name out of a directory
// name. Two trailing underscores mark a catch-all, one marks a dynamic
// segment, and anything else is static.
func ParseSegment(dir string) Segment {
	name := strings.TrimRight(dir, "_")
	if name == "" {
		// A name made only of underscores leaves no parameter to bind, so it is
		// an ordinary static segment rather than a malformed dynamic one.
		return Segment{Dir: dir, Name: dir, Kind: StaticSegment}
	}
	switch len(dir) - len(name) {
	case 0:
		return Segment{Dir: dir, Name: dir, Kind: StaticSegment}
	case 1:
		return Segment{Dir: dir, Name: name, Kind: DynamicSegment}
	default:
		return Segment{Dir: dir, Name: name, Kind: CatchAllSegment}
	}
}

// ValidateDirName reports whether a directory name can hold Go source as part
// of this module.
//
// A route directory is also a Go package, so its name must be a legal import
// path element. The Go toolchain rejects an illegal element while matching
// package patterns, before build constraints are considered, so a single
// offending directory breaks go build ./... for the whole module rather than
// for that package alone. That is why bracketed Next.js spellings such as
// [id] cannot be used here.
func ValidateDirName(name string) error {
	if name == "" {
		return errors.New("directory name is empty")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.' || r == '~':
		default:
			return fmt.Errorf("directory name %q contains %q, which is not allowed in a Go import path; "+
				"use a trailing underscore for a dynamic segment, as in id_", name, string(r))
		}
	}
	switch name[0] {
	case '-', '~':
		return fmt.Errorf("directory name %q may not start with %q in a Go import path", name, string(name[0]))
	}
	if strings.Trim(name, ".") == "" {
		return fmt.Errorf("directory name %q is only dots", name)
	}
	return nil
}

// PackageName derives the Go package clause for a route directory. A name that
// is already an identifier is used unchanged, which is what makes the common
// id_ form read naturally; anything else is sanitized, because a URL segment
// such as sign-in is legal on disk but not as a package clause.
func PackageName(dir string) string {
	if dir == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range dir {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			// Dropped rather than replaced, so sign-in and sign_in do not both
			// become the same package name by accident.
		}
	}
	name := b.String()
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		name = "p" + name
	}
	if token.IsKeyword(name) {
		name += "_"
	}
	return name
}

// ignoredDir reports directory names the Go toolchain excludes from ./...
// matching. They are excluded from routing too, which gives a private folder
// convention for free.
func ignoredDir(name string) bool {
	return strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") || name == "testdata"
}

func urlPath(segments []Segment) string {
	if len(segments) == 0 {
		return "/"
	}
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		switch segment.Kind {
		case DynamicSegment:
			parts = append(parts, "{"+segment.Name+"}")
		case CatchAllSegment:
			parts = append(parts, "{"+segment.Name+"...}")
		default:
			parts = append(parts, segment.Dir)
		}
	}
	return "/" + path.Join(parts...)
}

// normalizedPath erases parameter names so two routes differing only in what
// they call a segment are recognized as the same pattern.
func normalizedPath(segments []Segment) string {
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		switch segment.Kind {
		case DynamicSegment:
			parts = append(parts, "{}")
		case CatchAllSegment:
			parts = append(parts, "{...}")
		default:
			parts = append(parts, segment.Dir)
		}
	}
	return "/" + path.Join(parts...)
}

func params(segments []Segment) []Segment {
	var out []Segment
	for _, segment := range segments {
		if segment.Kind != StaticSegment {
			out = append(out, segment)
		}
	}
	return out
}

func duplicateParam(segments []Segment, name string) (string, bool) {
	for _, segment := range segments {
		if segment.Kind != StaticSegment && segment.Name == name {
			return segment.Dir, true
		}
	}
	return "", false
}

func lastSegment(segments []Segment) (Segment, bool) {
	if len(segments) == 0 {
		return Segment{}, false
	}
	return segments[len(segments)-1], true
}

// appendCopy extends a slice without ever sharing its backing array, so two
// sibling branches of the walk cannot overwrite each other's chain.
func appendCopy[T any](src []T, item T) []T {
	out := make([]T, len(src)+1)
	copy(out, src)
	out[len(src)] = item
	return out
}

// joinImport appends a route-relative directory to the configured import base.
// An empty base yields an empty path, so a caller that never set one sees no
// half-formed import.
func joinImport(base, relDir string) string {
	if base == "" {
		return ""
	}
	if relDir == "" {
		return base
	}
	return base + "/" + relDir
}

func joinRel(relDir, name string) string {
	if relDir == "" {
		return name
	}
	return relDir + "/" + name
}

func dirBase(dir, root string) string {
	if dir == root {
		return filepath.Base(root)
	}
	return filepath.Base(dir)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// joinErrors combines discovery or analysis failures into one error, so a
// caller reports every problem in a run rather than the first.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
