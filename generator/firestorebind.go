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
	firestorebindImportPath = "github.com/shibukawa/tinybind-go/firestorebind"
	datastoreImportPath     = "github.com/shibukawa/tinygodriver/nosql/datastore"
	// firestoreTag is the entity tag. It is named for the package rather than
	// for the API, so a reader who found firestorebind by name finds the tag by
	// the same word.
	firestoreTag = "firestore"
	// driverEntityTag is the spelling the driver's own MarshalEntity reads,
	// borrowed from cloud.google.com/go/datastore. It is recognized only to
	// reject it: the two mappings disagree on every renamed property, and the
	// driver's own documentation asks a generator to treat the overlap as an
	// error rather than as agreement.
	driverEntityTag = "datastore"
)

// FirestoreUsage selects which generated entity methods a type needs.
type FirestoreUsage uint8

const (
	// FirestoreEncode emits EncodeEntity.
	FirestoreEncode FirestoreUsage = 1 << iota
	// FirestoreDecode emits DecodeEntity.
	FirestoreDecode
	// FirestoreKey emits EntityKey.
	FirestoreKey
)

// FirestoreKind is how one Go type maps onto a Datastore property value.
type FirestoreKind string

const (
	FirestoreString  FirestoreKind = "string"
	FirestoreInt     FirestoreKind = "integer.int"
	FirestoreUint    FirestoreKind = "integer.uint"
	FirestoreDouble  FirestoreKind = "double"
	FirestoreBool    FirestoreKind = "boolean"
	FirestoreBlob    FirestoreKind = "blob"
	FirestoreTime    FirestoreKind = "timestamp"
	FirestoreKeyRef  FirestoreKind = "key"
	FirestoreGeo     FirestoreKind = "geoPoint"
	FirestoreArray   FirestoreKind = "array"
	FirestoreStruct  FirestoreKind = "entity"
	FirestorePointer FirestoreKind = "ptr"
	// FirestoreRaw is a datastore.Value field, stored as it stands. It is the
	// escape hatch for what the table above cannot express, including the
	// dynamic property names a map would have needed.
	FirestoreRaw FirestoreKind = "value"
)

// FirestoreType describes one Go type in property terms.
type FirestoreType struct {
	Kind FirestoreKind
	// Go is the type as it must be written inside the generated package.
	Go string
	// Elem is the element of a slice or pointer.
	Elem *FirestoreType
	// Struct is the named struct type of a nested entity, always declared in the
	// same package as its parent.
	Struct string
	// Bits is the width a number is parsed at: 8, 16, 32 or 64, and 0 for int
	// and uint, which strconv reads as the platform width.
	Bits int
}

// FirestoreFieldPlan is one struct field and the property it maps to.
type FirestoreFieldPlan struct {
	Name     string
	Property string
	// Role is "", "name", "id", "parent" or "version". A field with a role
	// other than "" and "version" carries identity rather than a property.
	Role      string
	OmitEmpty bool
	NoIndex   bool
	// Stored reports whether the field also becomes a property. It is false for
	// an identity field unless the tag gave it a real property name.
	Stored bool
	Type   FirestoreType
}

// FirestoreEntityPlan is the entity codec plan for one struct type.
type FirestoreEntityPlan struct {
	Name       string
	Kind       string
	SourcePath string
	Fields     []FirestoreFieldPlan
	Usage      FirestoreUsage
}

// Identity returns the field that supplies the key's name or id.
func (p FirestoreEntityPlan) Identity() (FirestoreFieldPlan, bool) {
	if f, ok := p.roleField("name"); ok {
		return f, true
	}
	return p.roleField("id")
}

// Parent returns the field that supplies the ancestor path.
func (p FirestoreEntityPlan) Parent() (FirestoreFieldPlan, bool) { return p.roleField("parent") }

// Version returns the field that receives Entity.Version.
func (p FirestoreEntityPlan) Version() (FirestoreFieldPlan, bool) { return p.roleField("version") }

func (p FirestoreEntityPlan) roleField(role string) (FirestoreFieldPlan, bool) {
	for _, f := range p.Fields {
		if f.Role == role {
			return f, true
		}
	}
	return FirestoreFieldPlan{}, false
}

// Properties returns the fields that become entity properties, which excludes
// the identity fields the key carries instead.
func (p FirestoreEntityPlan) Properties() []FirestoreFieldPlan {
	out := make([]FirestoreFieldPlan, 0, len(p.Fields))
	for _, f := range p.Fields {
		if f.Stored {
			out = append(out, f)
		}
	}
	return out
}

// FirestorePackagePlan is every entity plan in one package.
type FirestorePackagePlan struct {
	Package     string
	PackagePath string
	Entities    []FirestoreEntityPlan
}

// AnalyzeFirestoreEntities builds entity plans for the types a package binds to
// Firestore, discovered from firestorebind call sites.
func AnalyzeFirestoreEntities(dir string) (*FirestorePackagePlan, error) {
	opts := DefaultOptions()
	opts.GenerateAll = true
	return AnalyzeFirestoreEntitiesWithOptions(dir, opts)
}

// AnalyzeFirestoreEntitiesWithOptions is AnalyzeFirestoreEntities with custom
// discovery.
func AnalyzeFirestoreEntitiesWithOptions(dir string, opts Options) (*FirestorePackagePlan, error) {
	return analyzeFirestoreEntities(newPackageLoad(dir), opts)
}

func analyzeFirestoreEntities(load *packageLoad, opts Options) (*FirestorePackagePlan, error) {
	pkg, err := load.get()
	if err != nil {
		return nil, err
	}
	normalized, err := opts.normalized()
	if err != nil {
		return nil, err
	}
	plan := &FirestorePackagePlan{Package: pkg.Name, PackagePath: pkg.PkgPath}
	symbols := firestoreSymbols(normalized.symbols)
	if len(symbols) == 0 && !opts.GenerateAll {
		return plan, nil
	}

	usage := map[string]FirestoreUsage{}
	sources := map[string]string{}
	// Generated files are not inputs, and their declarations are not evidence of
	// a collision: a codec regenerated over its own output would otherwise find
	// the methods it wrote last time and refuse to write them again.
	generated := map[string]bool{}
	for _, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		if skipFirestoreFile(f, pkg, normalized) {
			generated[dynamoFileName(pkg, f)] = true
			continue
		}
		discovered, err := discoverGenericTypeArgs(f, pkg.TypesInfo, symbols)
		if err != nil {
			return nil, err
		}
		for name, bits := range discovered {
			usage[name] |= firestoreUsageOf(bits)
		}
		for name := range declaredStructNames(f) {
			if path := dynamoFileName(pkg, f); path != "" {
				sources[name] = path
			}
		}
	}
	if opts.GenerateAll {
		for name := range sources {
			if hasFirestoreTag(pkg, name) {
				usage[name] |= FirestoreEncode | FirestoreDecode | FirestoreKey
			}
		}
	}
	// A bound type that declares an identity always gets its key builder,
	// whether or not a call was found that needs one. The documented way to read
	// an entity is Load(ctx, v.EntityKey()), which uses the method rather than
	// any discoverable function, so a rule that waited for a discoverable use
	// would never emit it at all.
	for name, bits := range usage {
		if bits != 0 {
			usage[name] = bits | FirestoreKey
		}
	}
	if len(usage) == 0 {
		return plan, nil
	}

	collector := &firestoreCollector{pkg: pkg, plans: map[string]*FirestoreEntityPlan{}, generated: generated}
	names := make([]string, 0, len(usage))
	for name := range usage {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := collector.collect(name, usage[name]); err != nil {
			return nil, err
		}
	}
	for _, name := range collector.order {
		entity := collector.plans[name]
		entity.SourcePath = sources[name]
		plan.Entities = append(plan.Entities, *entity)
	}
	sort.SliceStable(plan.Entities, func(i, j int) bool { return plan.Entities[i].Name < plan.Entities[j].Name })
	return plan, nil
}

// firestoreSymbols keeps the discovery symbols whose usage is an entity
// operation.
func firestoreSymbols(symbols []DiscoverySymbol) []DiscoverySymbol {
	var out []DiscoverySymbol
	for _, symbol := range symbols {
		if symbol.Usage&UsageEntity != 0 {
			out = append(out, symbol)
		}
	}
	return out
}

func firestoreUsageOf(usage Usage) FirestoreUsage {
	var out FirestoreUsage
	if usage&UsageEncodeEntity != 0 {
		out |= FirestoreEncode
	}
	if usage&UsageDecodeEntity != 0 {
		out |= FirestoreDecode
	}
	if usage&UsageEntityKey != 0 {
		out |= FirestoreKey
	}
	return out
}

func skipFirestoreFile(f *ast.File, pkg *packages.Package, normalized normalizedOptions) bool {
	base := filepath.Base(dynamoFileName(pkg, f))
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_httpbind_gen.go") ||
		strings.HasSuffix(base, "_openapi_gen.go") ||
		base == defaultDynamoOut ||
		base == defaultDynamoQueryOut ||
		base == defaultFirestoreOut ||
		base == "tinybind_gen.go" ||
		base == "tinybind_openapi_gen.go" ||
		gensource.IsGenerated(f, normalized.parserConfig.GeneratedHeaders...)
}

// hasFirestoreTag reports whether a package-level struct declares at least one
// firestore tag. Generate-all uses it so an unrelated request or config struct
// does not acquire an entity codec it never asked for.
func hasFirestoreTag(pkg *packages.Package, name string) bool {
	st, _, ok := lookupStruct(pkg, name)
	if !ok {
		return false
	}
	for i := range st.NumFields() {
		if _, found := reflect.StructTag(st.Tag(i)).Lookup(firestoreTag); found {
			return true
		}
	}
	return false
}

// firestoreCollector builds one plan per type, following nested structs so a
// parent's operations reach the types it contains.
type firestoreCollector struct {
	pkg   *packages.Package
	plans map[string]*FirestoreEntityPlan
	order []string
	// generated names the files a previous run wrote, whose declarations are
	// invisible to this one.
	generated map[string]bool
	// nested names the types reached only as a field of another, which may
	// declare no identity of their own.
	nested map[string]bool
}

func (c *firestoreCollector) collect(name string, usage FirestoreUsage) error {
	if usage == 0 {
		return nil
	}
	if existing, ok := c.plans[name]; ok {
		before := existing.Usage
		existing.Usage |= usage
		if existing.Usage == before {
			return nil
		}
		return c.recurse(*existing)
	}
	st, named, ok := lookupStruct(c.pkg, name)
	if !ok {
		return fmt.Errorf("firestorebind: %s is not a package-level struct type", name)
	}
	entity, err := c.plan(name, st, named)
	if err != nil {
		return err
	}
	entity.Usage = usage
	c.plans[name] = &entity
	c.order = append(c.order, name)
	return c.recurse(entity)
}

// recurse gives every nested struct the operations its parent needs. A key is
// never inherited: an entityValue carries none, so only a type a call names has
// one.
func (c *firestoreCollector) recurse(entity FirestoreEntityPlan) error {
	inherited := entity.Usage &^ FirestoreKey
	if inherited == 0 {
		return nil
	}
	for _, f := range entity.Fields {
		for t := &f.Type; t != nil; t = t.Elem {
			if t.Kind == FirestoreStruct {
				if c.nested == nil {
					c.nested = map[string]bool{}
				}
				c.nested[t.Struct] = true
				if err := c.collect(t.Struct, inherited); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *firestoreCollector) plan(name string, st *types.Struct, named *types.Named) (FirestoreEntityPlan, error) {
	entity := FirestoreEntityPlan{Name: name, Kind: name}
	seen := map[string]string{}
	roles := map[string]string{}
	for i := range st.NumFields() {
		field := st.Field(i)
		tag := reflect.StructTag(st.Tag(i))
		if field.Embedded() || !field.Exported() {
			continue
		}
		raw, tagged := tag.Lookup(firestoreTag)
		if !tagged {
			if _, driver := tag.Lookup(driverEntityTag); driver {
				return entity, fmt.Errorf(
					"firestorebind: %s.%s: found tag %q but no %q tag; the driver's own MarshalEntity reads %q and this codec reads %q, and the two disagree on every renamed property",
					name, field.Name(), driverEntityTag, firestoreTag, driverEntityTag, firestoreTag)
			}
			continue
		}
		plan, err := c.field(name, field, raw)
		if err != nil {
			return entity, err
		}
		if plan.Name == "" {
			continue
		}
		switch plan.Role {
		case "name", "id", "parent", "version":
			if other, dup := roles[plan.Role]; dup {
				return entity, fmt.Errorf("firestorebind: %s: fields %s and %s are both %s", name, other, plan.Name, plan.Role)
			}
			roles[plan.Role] = plan.Name
		}
		if plan.Stored {
			if other, dup := seen[plan.Property]; dup {
				return entity, fmt.Errorf("firestorebind: %s: fields %s and %s both map to property %q", name, other, plan.Name, plan.Property)
			}
			seen[plan.Property] = plan.Name
		}
		entity.Fields = append(entity.Fields, plan)
	}
	if _, both := roles["name"]; both {
		if _, also := roles["id"]; also {
			return entity, fmt.Errorf("firestorebind: %s: fields %s and %s declare both a name and an id key; a key path element carries one or the other",
				name, roles["name"], roles["id"])
		}
	}
	if len(entity.Fields) == 0 {
		return entity, fmt.Errorf("firestorebind: %s has no mappable exported field", name)
	}
	if err := c.checkMethodCollision(named, entity); err != nil {
		return entity, err
	}
	return entity, nil
}

func (c *firestoreCollector) field(typeName string, field *types.Var, raw string) (FirestoreFieldPlan, error) {
	parts := strings.Split(raw, ",")
	plan := FirestoreFieldPlan{Name: field.Name(), Property: field.Name(), Stored: true}
	named := false
	if len(parts) > 0 && parts[0] != "" {
		if parts[0] == "-" {
			plan.Stored = false
		} else {
			plan.Property = parts[0]
			named = true
		}
	}
	for _, option := range parts[1:] {
		switch option {
		case "":
		case "omitempty":
			plan.OmitEmpty = true
		case "noindex":
			plan.NoIndex = true
		case "name", "id", "parent", "version":
			plan.Role = option
		default:
			return plan, fmt.Errorf("firestorebind: %s.%s: unknown %s tag option %q", typeName, field.Name(), firestoreTag, option)
		}
	}
	// A field named "-" with no role is skipped outright, as in every other
	// mapping mode.
	if !plan.Stored && plan.Role == "" {
		return FirestoreFieldPlan{}, nil
	}
	// An identity field is not a property unless the tag gave it a real name,
	// which is the deliberate opt-in to storing identity twice.
	if plan.Role == "name" || plan.Role == "id" || plan.Role == "parent" {
		plan.Stored = named
	}
	// A version is never written: the server assigns it.
	if plan.Role == "version" {
		plan.Stored = false
		if !named {
			plan.Property = field.Name()
		}
	}
	if plan.NoIndex && !plan.Stored {
		return plan, fmt.Errorf("firestorebind: %s.%s: noindex on a field that is not stored as a property", typeName, field.Name())
	}
	resolved, err := c.resolve(field.Type(), typeName, field.Name())
	if err != nil {
		return plan, err
	}
	plan.Type = resolved
	if err := checkFirestoreRole(typeName, field.Name(), plan.Role, resolved); err != nil {
		return plan, err
	}
	return plan, nil
}

// checkFirestoreRole rejects an identity option on a field that cannot carry it.
// A key path element holds a string name or an int64 id and nothing else, and a
// version is the int64 the server reports.
func checkFirestoreRole(typeName, fieldName, role string, t FirestoreType) error {
	fail := func(want string) error {
		return fmt.Errorf("firestorebind: %s.%s: %s needs %s, and %s is not one", typeName, fieldName, role, want, t.Go)
	}
	switch role {
	case "name":
		if t.Kind != FirestoreString {
			return fail("a string field")
		}
	case "id":
		if t.Kind != FirestoreInt || (t.Bits != 64 && t.Bits != 0) {
			return fail("an int64 field")
		}
	case "version":
		if t.Kind != FirestoreInt || (t.Bits != 64 && t.Bits != 0) {
			return fail("an int64 field")
		}
	case "parent":
		if t.Kind != FirestoreKeyRef && t.Kind != FirestoreStruct {
			return fail("a datastore.Key field or another bound type")
		}
	}
	return nil
}

// checkMethodCollision rejects a type that already declares one of the methods
// the codec emits, by hand. A method the last generation run wrote does not
// count: it is about to be replaced.
func (c *firestoreCollector) checkMethodCollision(named *types.Named, entity FirestoreEntityPlan) error {
	emitted := map[string]bool{"EncodeEntity": true, "DecodeEntity": true, "EntityKey": true, "Kind": true, "EntityVersion": true}
	for i := range named.NumMethods() {
		method := named.Method(i)
		if !emitted[method.Name()] || c.wasGenerated(method.Pos()) {
			continue
		}
		return fmt.Errorf("firestorebind: %s already declares %s; the generated codec would collide with it", entity.Name, method.Name())
	}
	return nil
}

func (c *firestoreCollector) wasGenerated(pos token.Pos) bool {
	if c.pkg == nil || c.pkg.Fset == nil || !pos.IsValid() {
		return false
	}
	return c.generated[c.pkg.Fset.Position(pos).Filename]
}
