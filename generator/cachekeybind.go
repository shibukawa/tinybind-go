package generator

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/shibukawa/tinybind-go/internal/gensource"
)

const (
	cachekeybindImportPath = "github.com/shibukawa/tinybind-go/cachekeybind"
	// cacheKeyTag is the cache key tag. It is short and names its purpose, as
	// check, db, dynamo and firestore do.
	cacheKeyTag = "cache"
	// cacheKeyMark is the tag value marking a field as part of the key, and the
	// only value the tag takes.
	cacheKeyMark = "key"
	// cacheKeyMethod is the single method a key type gets. One struct yields one
	// key, so the name never varies and the type satisfies the runtime interface
	// directly.
	cacheKeyMethod = "CacheKey"
)

// CacheKeyKind is how one Go type is framed into a cache key.
type CacheKeyKind string

const (
	CacheKeyString   CacheKeyKind = "string"
	CacheKeyBool     CacheKeyKind = "bool"
	CacheKeyInt      CacheKeyKind = "int"
	CacheKeyUint     CacheKeyKind = "uint"
	CacheKeyFloat    CacheKeyKind = "float"
	CacheKeyBytes    CacheKeyKind = "bytes"
	CacheKeyTime     CacheKeyKind = "time"
	CacheKeyOptional CacheKeyKind = "optional"
	CacheKeyArray    CacheKeyKind = "array"
)

// CacheKeyType is how one Go type reaches a framing helper.
type CacheKeyType struct {
	Kind CacheKeyKind
	// Go is the type as it must be written inside the generated package. It is
	// needed for the closure parameter an optional or an array frames through.
	Go string
	// Elem is the element of a pointer or a slice.
	Elem *CacheKeyType
}

// CacheKeyFieldPlan is one field that participates in a key.
type CacheKeyFieldPlan struct {
	Name string
	Type CacheKeyType
}

// CacheKeyPlan is the key plan for one struct type.
type CacheKeyPlan struct {
	Name       string
	Fields     []CacheKeyFieldPlan
	SourcePath string
}

// Identity is the prefix that separates this type's entries from every other
// type's. It is wholly derived: the package path and name are unique by
// construction while one struct yields one key, so nothing here is declared and
// nothing can be forgotten.
//
// There is deliberately no author-declared version. Invalidating on a meaning
// change is a deployment's job, and api:cache-store already states that the
// runtime never invalidates entries.
func (p CacheKeyPlan) Identity(packagePath string) string {
	if packagePath == "" {
		return p.Name
	}
	return packagePath + "." + p.Name
}

// CacheKeyPackagePlan is every key plan in one package.
type CacheKeyPackagePlan struct {
	Package     string
	PackagePath string
	Keys        []CacheKeyPlan
}

// AnalyzeCacheKeys builds key plans for the types a package uses as cache keys.
func AnalyzeCacheKeys(dir string) (*CacheKeyPackagePlan, error) {
	opts := DefaultOptions()
	opts.GenerateAll = true
	return AnalyzeCacheKeysWithOptions(dir, opts)
}

// AnalyzeCacheKeysWithOptions is AnalyzeCacheKeys with custom discovery.
func AnalyzeCacheKeysWithOptions(dir string, opts Options) (*CacheKeyPackagePlan, error) {
	return analyzeCacheKeys(newPackageLoad(dir), opts)
}

func analyzeCacheKeys(load *packageLoad, opts Options) (*CacheKeyPackagePlan, error) {
	pkg, err := load.get()
	if err != nil {
		return nil, err
	}
	normalized, err := opts.normalized()
	if err != nil {
		return nil, err
	}
	plan := &CacheKeyPackagePlan{Package: pkg.Name, PackagePath: pkg.PkgPath}
	symbols := cacheKeySymbols(normalized.symbols)
	if len(symbols) == 0 && !opts.GenerateAll {
		return plan, nil
	}

	// reached names the types a call site passes as a key. A type reached with
	// no marked field is an error rather than an empty key, so this set has to
	// stay separate from the tagged set below.
	reached := map[string]bool{}
	sources := map[string]string{}
	generated := map[string]bool{}
	for _, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		if skipCacheKeyFile(f, pkg, normalized) {
			generated[cacheKeyFileName(pkg, f)] = true
			continue
		}
		discovered, err := discoverGenericTypeArgs(f, pkg.TypesInfo, symbols, pkg.PkgPath)
		if err != nil {
			return nil, err
		}
		for name, bits := range discovered {
			if bits&UsageCacheKey != 0 {
				reached[name] = true
			}
		}
		for name := range declaredStructNames(f) {
			if path := cacheKeyFileName(pkg, f); path != "" {
				sources[name] = path
			}
		}
	}
	// Generate-all is gated on the tag, so an unrelated request or config struct
	// does not acquire a key method it never asked for. The gate exists at all
	// only because marking is opt-in: under default-include a key type would
	// carry no tag and nothing would distinguish it.
	wanted := map[string]bool{}
	for name := range reached {
		wanted[name] = true
	}
	if opts.GenerateAll {
		for name := range sources {
			if hasCacheKeyTag(pkg, name) {
				wanted[name] = true
			}
		}
	}
	if len(wanted) == 0 {
		return plan, nil
	}

	collector := &cacheKeyCollector{pkg: pkg, generated: generated}
	names := make([]string, 0, len(wanted))
	for name := range wanted {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		item, err := collector.collect(name, reached[name])
		if err != nil {
			return nil, err
		}
		if item == nil {
			continue
		}
		item.SourcePath = sources[name]
		plan.Keys = append(plan.Keys, *item)
	}
	sort.SliceStable(plan.Keys, func(i, j int) bool { return plan.Keys[i].Name < plan.Keys[j].Name })
	return plan, nil
}

// cacheKeySymbols keeps the discovery symbols whose usage is a cache key.
func cacheKeySymbols(symbols []DiscoverySymbol) []DiscoverySymbol {
	var out []DiscoverySymbol
	for _, symbol := range symbols {
		if symbol.Usage&UsageCacheKey != 0 {
			out = append(out, symbol)
		}
	}
	return out
}

func skipCacheKeyFile(f *ast.File, pkg *packages.Package, normalized normalizedOptions) bool {
	base := filepath.Base(cacheKeyFileName(pkg, f))
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_httpbind_gen.go") ||
		strings.HasSuffix(base, "_openapi_gen.go") ||
		base == defaultCacheKeyOut ||
		base == "tinybind_gen.go" ||
		base == "tinybind_openapi_gen.go" ||
		gensource.IsGenerated(f, normalized.parserConfig.GeneratedHeaders...)
}

// cacheKeyFileName is the on-disk path of one syntax file.
func cacheKeyFileName(pkg *packages.Package, f *ast.File) string {
	if pkg == nil || pkg.Fset == nil || f == nil {
		return ""
	}
	handle := pkg.Fset.File(f.Pos())
	if handle == nil {
		return ""
	}
	return handle.Name()
}

// hasCacheKeyTag reports whether a named struct carries any cache tag.
func hasCacheKeyTag(pkg *packages.Package, name string) bool {
	st, _, ok := lookupStruct(pkg, name)
	if !ok {
		return false
	}
	for i := range st.NumFields() {
		if _, found := reflect.StructTag(st.Tag(i)).Lookup(cacheKeyTag); found {
			return true
		}
	}
	return false
}

type cacheKeyCollector struct {
	pkg *packages.Package
	// generated names the files this run skipped. A declaration in one is not
	// evidence of a collision: a key regenerated over its own output would
	// otherwise find the method it wrote last time and refuse to write it again.
	generated map[string]bool
}

// collect plans one type. reached reports whether a call site passed it as a
// key, which is what makes an unmarked struct an error rather than a skip.
func (c *cacheKeyCollector) collect(name string, reached bool) (*CacheKeyPlan, error) {
	st, named, ok := lookupStruct(c.pkg, name)
	if !ok {
		if reached {
			return nil, fmt.Errorf("cachekeybind: %s is used as a cache key but is not a struct type declared in this package", name)
		}
		return nil, nil
	}
	plan := &CacheKeyPlan{Name: name}
	for i := range st.NumFields() {
		field := st.Field(i)
		tag, tagged := reflect.StructTag(st.Tag(i)).Lookup(cacheKeyTag)
		if !tagged {
			continue
		}
		// A blank field carries no value, so nothing about it can reach a key.
		// It is checked before the exported filter below, because a blank
		// identifier is unexported and would otherwise be skipped in silence.
		if field.Name() == "_" {
			return nil, fmt.Errorf("cachekeybind: %s: a blank field carries no value, so a %s tag on one keys nothing; the identity is derived from the package path and type name, and invalidating a meaning change is the deployment's job", name, cacheKeyTag)
		}
		if field.Embedded() || !field.Exported() {
			return nil, fmt.Errorf("cachekeybind: %s.%s: an unexported or embedded field cannot be marked; a key is built from exported fields only", name, field.Name())
		}
		if err := checkCacheKeyMark(name, field.Name(), tag); err != nil {
			return nil, err
		}
		fieldType, err := c.fieldType(field.Type())
		if err != nil {
			return nil, fmt.Errorf("cachekeybind: %s.%s: %w", name, field.Name(), err)
		}
		plan.Fields = append(plan.Fields, CacheKeyFieldPlan{Name: field.Name(), Type: fieldType})
	}
	if len(plan.Fields) == 0 {
		if !reached {
			return nil, nil
		}
		return nil, fmt.Errorf("cachekeybind: %s is used as a cache key but marks no field; every instance would share one entry, so mark the fields the result depends on with %s:%q", name, cacheKeyTag, cacheKeyMark)
	}
	if err := c.checkMethodCollision(named, name); err != nil {
		return nil, err
	}
	return plan, nil
}

// checkCacheKeyMark rejects anything but the mark. The tag takes one value, so
// every other spelling is a mistake rather than a variant.
func checkCacheKeyMark(typeName, fieldName, tag string) error {
	for _, option := range strings.Split(tag, ",") {
		switch option {
		case cacheKeyMark:
		case "":
			return fmt.Errorf("cachekeybind: %s.%s: empty %s tag option; write %s:%q", typeName, fieldName, cacheKeyTag, cacheKeyTag, cacheKeyMark)
		default:
			return fmt.Errorf("cachekeybind: %s.%s: unknown %s tag option %q; the only value is %q", typeName, fieldName, cacheKeyTag, option, cacheKeyMark)
		}
	}
	return nil
}

// checkMethodCollision refuses a type that already declares the method, so a
// hand-written key is never silently replaced by a generated one.
func (c *cacheKeyCollector) checkMethodCollision(named *types.Named, name string) error {
	if named == nil {
		return nil
	}
	for i := range named.NumMethods() {
		method := named.Method(i)
		if method.Name() != cacheKeyMethod || c.wasGenerated(method.Pos()) {
			continue
		}
		return fmt.Errorf("cachekeybind: %s already declares %s; the generated key would collide with it", name, cacheKeyMethod)
	}
	return nil
}

// wasGenerated reports whether a declaration lives in a file this run skipped.
func (c *cacheKeyCollector) wasGenerated(pos token.Pos) bool {
	if c.pkg == nil || c.pkg.Fset == nil || !pos.IsValid() {
		return false
	}
	handle := c.pkg.Fset.File(pos)
	return handle != nil && c.generated[handle.Name()]
}

// fieldType maps a Go type onto the framing helper that encodes it. A type with
// no framing is an error rather than a skipped field: a skipped field is the
// silent wrong-answer failure this package exists to remove.
func (c *cacheKeyCollector) fieldType(t types.Type) (CacheKeyType, error) {
	goType := c.goString(t)
	fail := func(format string, args ...any) (CacheKeyType, error) {
		return CacheKeyType{}, fmt.Errorf(format, args...)
	}
	if isCacheKeyTime(t) {
		return CacheKeyType{Kind: CacheKeyTime, Go: goType}, nil
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		kind, ok := basicCacheKeyKind(u)
		if !ok {
			return fail("%s has no cache key framing", goType)
		}
		return CacheKeyType{Kind: kind, Go: goType}, nil

	case *types.Pointer:
		elem, err := c.fieldType(u.Elem())
		if err != nil {
			return CacheKeyType{}, err
		}
		return CacheKeyType{Kind: CacheKeyOptional, Go: goType, Elem: &elem}, nil

	case *types.Slice:
		if basic, ok := u.Elem().Underlying().(*types.Basic); ok && basic.Kind() == types.Byte {
			return CacheKeyType{Kind: CacheKeyBytes, Go: goType}, nil
		}
		elem, err := c.fieldType(u.Elem())
		if err != nil {
			return CacheKeyType{}, err
		}
		return CacheKeyType{Kind: CacheKeyArray, Go: goType, Elem: &elem}, nil

	default:
		return fail("%s has no cache key framing; a struct or map field cannot be framed, so key on the fields it is derived from instead", goType)
	}
}

// goString writes a type as the generated file must spell it: unqualified for
// this package, package-qualified for anything else.
func (c *cacheKeyCollector) goString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if c.pkg != nil && c.pkg.Types != nil && p == c.pkg.Types {
			return ""
		}
		return p.Name()
	})
}

// isCacheKeyTime reports whether a type is time.Time, which frames through a
// fixed layout rather than through its struct fields.
func isCacheKeyTime(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "time" && obj.Name() == "Time"
}

func basicCacheKeyKind(b *types.Basic) (CacheKeyKind, bool) {
	switch b.Kind() {
	case types.String:
		return CacheKeyString, true
	case types.Bool:
		return CacheKeyBool, true
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
		return CacheKeyInt, true
	case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr:
		return CacheKeyUint, true
	case types.Float32, types.Float64:
		return CacheKeyFloat, true
	default:
		return "", false
	}
}
