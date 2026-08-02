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
	dynamobindImportPath = "github.com/shibukawa/tinybind-go/dynamobind"
	dynamodbImportPath   = "github.com/shibukawa/tinygodriver/nosql/dynamodb"
	// dynamoTag is the item tag. It is short and names its purpose, as check,
	// db, opt and enum do; the SDK's dynamodbav names a library instead.
	dynamoTag = "dynamo"
	// sdkItemTag is the aws-sdk-go-v2 and driver-reflection spelling. It is
	// recognized only to reject it, since a struct ported from the SDK would
	// otherwise be generated against Go field names without a word of warning.
	sdkItemTag = "dynamodbav"
)

// DynamoUsage selects which generated item methods a type needs.
type DynamoUsage uint8

const (
	// DynamoEncode emits EncodeItem.
	DynamoEncode DynamoUsage = 1 << iota
	// DynamoDecode emits DecodeItem.
	DynamoDecode
	// DynamoKey emits ItemKey and the table definition.
	DynamoKey
)

// DynamoKind is how one Go type maps onto a DynamoDB attribute.
type DynamoKind string

const (
	DynamoString    DynamoKind = "S"
	DynamoInt       DynamoKind = "N.int"
	DynamoUint      DynamoKind = "N.uint"
	DynamoFloat     DynamoKind = "N.float"
	DynamoBool      DynamoKind = "BOOL"
	DynamoBytes     DynamoKind = "B"
	DynamoTime      DynamoKind = "S.time"
	DynamoUnixTime  DynamoKind = "N.time"
	DynamoList      DynamoKind = "L"
	DynamoMap       DynamoKind = "M"
	DynamoStruct    DynamoKind = "M.struct"
	DynamoPointer   DynamoKind = "ptr"
	DynamoStringSet DynamoKind = "SS"
	DynamoNumberSet DynamoKind = "NS"
	DynamoBinarySet DynamoKind = "BS"
	// DynamoRaw is a dynamodb.AttributeValue field, stored as it stands. It is
	// the escape hatch for what the table above cannot express.
	DynamoRaw DynamoKind = "AV"
)

// DynamoType describes one Go type in attribute terms.
type DynamoType struct {
	Kind DynamoKind
	// Go is the type as it must be written inside the generated package.
	Go string
	// Elem is the element of a slice, map or pointer.
	Elem *DynamoType
	// MapKey is the key type of a map attribute, written as the generated
	// package must spell it. DynamoDB map keys are strings, so it is always a
	// string-kinded type.
	MapKey string
	// Struct is the named struct type of a nested item, always declared in the
	// same package as its parent.
	Struct string
	// Bits is the width a number is parsed at: 8, 16, 32 or 64, and 0 for int
	// and uint, which strconv reads as the platform width. Parsing at the
	// field's own width turns a value it cannot hold into an error instead of a
	// silent wrap.
	Bits int
}

// DynamoFieldPlan is one struct field and the attribute it maps to.
type DynamoFieldPlan struct {
	Name      string
	Attribute string
	OmitEmpty bool
	// Key is "", "partition" or "sort".
	Key  string
	Type DynamoType
}

// DynamoItemPlan is the item codec plan for one struct type.
type DynamoItemPlan struct {
	Name       string
	SourcePath string
	Doc        string
	Fields     []DynamoFieldPlan
	Usage      DynamoUsage
}

// PartitionKey returns the partition key field, if the type declares one.
func (p DynamoItemPlan) PartitionKey() (DynamoFieldPlan, bool) {
	return p.keyField("partition")
}

// SortKey returns the sort key field, if the type declares one.
func (p DynamoItemPlan) SortKey() (DynamoFieldPlan, bool) {
	return p.keyField("sort")
}

func (p DynamoItemPlan) keyField(kind string) (DynamoFieldPlan, bool) {
	for _, f := range p.Fields {
		if f.Key == kind {
			return f, true
		}
	}
	return DynamoFieldPlan{}, false
}

// DynamoPackagePlan is every item plan in one package.
type DynamoPackagePlan struct {
	Package     string
	PackagePath string
	Items       []DynamoItemPlan
}

// AnalyzeDynamoItems builds item plans for the types a package binds to
// DynamoDB, discovered from dynamobind call sites.
func AnalyzeDynamoItems(dir string) (*DynamoPackagePlan, error) {
	opts := DefaultOptions()
	opts.GenerateAll = true
	return AnalyzeDynamoItemsWithOptions(dir, opts)
}

// AnalyzeDynamoItemsWithOptions is AnalyzeDynamoItems with custom discovery.
func AnalyzeDynamoItemsWithOptions(dir string, opts Options) (*DynamoPackagePlan, error) {
	return analyzeDynamoItems(newPackageLoad(dir), opts)
}

func analyzeDynamoItems(load *packageLoad, opts Options) (*DynamoPackagePlan, error) {
	pkg, err := load.get()
	if err != nil {
		return nil, err
	}
	normalized, err := opts.normalized()
	if err != nil {
		return nil, err
	}
	plan := &DynamoPackagePlan{Package: pkg.Name, PackagePath: pkg.PkgPath}
	symbols := dynamoSymbols(normalized.symbols)
	// A declared query is a use of its result type as surely as a call is: the
	// generated function instantiates dynamobind.Query with it, which does not
	// compile without the decoder. Reading the declarations here rather than at
	// the query pass means the codec and the query agree on what is bound, and
	// that a package whose only DynamoDB use is a declaration still gets one.
	declared, err := declaredDynamoItemTypes(load.dir, opts)
	if err != nil {
		return nil, err
	}
	if len(symbols) == 0 && len(declared) == 0 && !opts.GenerateAll {
		return plan, nil
	}

	usage := map[string]DynamoUsage{}
	sources := map[string]string{}
	// Generated files are not inputs. Their declarations are also not evidence
	// of a collision: a codec regenerated over its own output would otherwise
	// find the methods it wrote last time and refuse to write them again.
	generated := map[string]bool{}
	for _, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		if skipDynamoFile(f, pkg, normalized) {
			generated[dynamoFileName(pkg, f)] = true
			continue
		}
		discovered, err := discoverGenericTypeArgs(f, pkg.TypesInfo, symbols)
		if err != nil {
			return nil, err
		}
		for name, bits := range discovered {
			usage[name] |= dynamoUsageOf(bits)
		}
		for name := range declaredStructNames(f) {
			if path := dynamoFileName(pkg, f); path != "" {
				sources[name] = path
			}
		}
	}
	if opts.GenerateAll {
		for name := range sources {
			if hasDynamoTag(pkg, name) {
				usage[name] |= DynamoEncode | DynamoDecode | DynamoKey
			}
		}
	}
	// A name the package does not declare with dynamo tags is left out, so the
	// query pass reports it against the declaration that named it rather than
	// this pass failing on a type it cannot collect.
	for _, name := range declared {
		if _, ok := sources[name]; ok && hasDynamoTag(pkg, name) {
			usage[name] |= DynamoDecode
		}
	}
	// A bound type that declares a key always gets its key builder, whether or
	// not a call was found that needs one. Remove and Update are not the only
	// callers: the documented way to read an item is Load(..., v.ItemKey()),
	// which uses the method rather than any discoverable function, and a rule
	// that waited for a discoverable use would never emit it at all. The method
	// is three lines and the linker drops it when nothing calls it.
	for name, bits := range usage {
		if bits != 0 {
			usage[name] = bits | DynamoKey
		}
	}
	if len(usage) == 0 {
		return plan, nil
	}

	collector := &dynamoCollector{pkg: pkg, plans: map[string]*DynamoItemPlan{}, generated: generated}
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
		item := collector.plans[name]
		item.SourcePath = sources[name]
		plan.Items = append(plan.Items, *item)
	}
	sort.SliceStable(plan.Items, func(i, j int) bool { return plan.Items[i].Name < plan.Items[j].Name })
	return plan, nil
}

// dynamoSymbols keeps the discovery symbols whose usage is an item operation.
func dynamoSymbols(symbols []DiscoverySymbol) []DiscoverySymbol {
	var out []DiscoverySymbol
	for _, symbol := range symbols {
		if symbol.Usage&(UsageEncodeItem|UsageDecodeItem|UsageItemKey) != 0 {
			out = append(out, symbol)
		}
	}
	return out
}

func dynamoUsageOf(usage Usage) DynamoUsage {
	var out DynamoUsage
	if usage&UsageEncodeItem != 0 {
		out |= DynamoEncode
	}
	if usage&UsageDecodeItem != 0 {
		out |= DynamoDecode
	}
	if usage&UsageItemKey != 0 {
		out |= DynamoKey
	}
	return out
}

// skipDynamoFile drops the inputs no discovery pass may read: test files and
// anything a generation run wrote, whatever it was named.
func skipDynamoFile(f *ast.File, pkg *packages.Package, normalized normalizedOptions) bool {
	base := filepath.Base(dynamoFileName(pkg, f))
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_httpbind_gen.go") ||
		strings.HasSuffix(base, "_openapi_gen.go") ||
		base == defaultDynamoOut ||
		base == defaultDynamoQueryOut ||
		base == "tinybind_gen.go" ||
		base == "tinybind_openapi_gen.go" ||
		gensource.IsGenerated(f, normalized.parserConfig.GeneratedHeaders...)
}

// dynamoFileName is the on-disk path of one syntax file.
func dynamoFileName(pkg *packages.Package, f *ast.File) string {
	if pkg == nil || pkg.Fset == nil || f == nil {
		return ""
	}
	// An unparsable file reports token.NoPos, which the FileSet has no handle
	// for; an empty path is what the callers already expect here.
	handle := pkg.Fset.File(f.Pos())
	if handle == nil {
		return ""
	}
	return handle.Name()
}

func declaredStructNames(f *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil {
				continue
			}
			if _, ok := ts.Type.(*ast.StructType); ok {
				out[ts.Name.Name] = true
			}
		}
	}
	return out
}

// hasDynamoTag reports whether a package-level struct declares at least one
// dynamo tag. Generate-all uses it so an unrelated request or config struct
// does not acquire an item codec it never asked for.
func hasDynamoTag(pkg *packages.Package, name string) bool {
	st, _, ok := lookupStruct(pkg, name)
	if !ok {
		return false
	}
	for i := range st.NumFields() {
		if _, found := reflect.StructTag(st.Tag(i)).Lookup(dynamoTag); found {
			return true
		}
	}
	return false
}

func lookupStruct(pkg *packages.Package, name string) (*types.Struct, *types.Named, bool) {
	if pkg.Types == nil || pkg.Types.Scope() == nil {
		return nil, nil, false
	}
	obj := pkg.Types.Scope().Lookup(name)
	if obj == nil {
		return nil, nil, false
	}
	named, ok := types.Unalias(obj.Type()).(*types.Named)
	if !ok {
		return nil, nil, false
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil, nil, false
	}
	return st, named, true
}

// dynamoCollector builds one plan per type, following nested structs so a
// parent's operations reach the types it contains.
type dynamoCollector struct {
	pkg   *packages.Package
	plans map[string]*DynamoItemPlan
	order []string
	// generated names the files a previous run wrote, whose declarations are
	// invisible to this one.
	generated map[string]bool
}

func (c *dynamoCollector) collect(name string, usage DynamoUsage) error {
	if usage == 0 {
		return nil
	}
	if existing, ok := c.plans[name]; ok {
		// A type reached twice only gains operations; its fields are the same.
		before := existing.Usage
		existing.Usage |= usage
		if existing.Usage == before {
			return nil
		}
		return c.recurse(*existing)
	}
	st, named, ok := lookupStruct(c.pkg, name)
	if !ok {
		return fmt.Errorf("dynamobind: %s is not a package-level struct type", name)
	}
	item, err := c.plan(name, st, named)
	if err != nil {
		return err
	}
	item.Usage = usage
	c.plans[name] = &item
	c.order = append(c.order, name)
	return c.recurse(item)
}

// recurse gives every nested struct the operations its parent needs. A key is
// never inherited: only the type a call names has a table key.
func (c *dynamoCollector) recurse(item DynamoItemPlan) error {
	inherited := item.Usage &^ DynamoKey
	if inherited == 0 {
		return nil
	}
	for _, f := range item.Fields {
		for t := &f.Type; t != nil; t = t.Elem {
			if t.Kind == DynamoStruct {
				if err := c.collect(t.Struct, inherited); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *dynamoCollector) plan(name string, st *types.Struct, named *types.Named) (DynamoItemPlan, error) {
	item := DynamoItemPlan{Name: name}
	seen := map[string]string{}
	partition, sortKey := "", ""
	for i := range st.NumFields() {
		field := st.Field(i)
		tag := reflect.StructTag(st.Tag(i))
		if field.Embedded() || !field.Exported() {
			continue
		}
		raw, tagged := tag.Lookup(dynamoTag)
		if !tagged {
			if _, sdk := tag.Lookup(sdkItemTag); sdk {
				return item, fmt.Errorf("dynamobind: %s.%s: found tag %q but no %q tag; rename it, or the field would be stored under its Go name",
					name, field.Name(), sdkItemTag, dynamoTag)
			}
		}
		if raw == "-" {
			continue
		}
		plan, err := c.field(name, field, raw)
		if err != nil {
			return item, err
		}
		if other, dup := seen[plan.Attribute]; dup {
			return item, fmt.Errorf("dynamobind: %s: fields %s and %s both map to attribute %q", name, other, plan.Name, plan.Attribute)
		}
		seen[plan.Attribute] = plan.Name
		switch plan.Key {
		case "partition":
			if partition != "" {
				return item, fmt.Errorf("dynamobind: %s: fields %s and %s are both partitionkey", name, partition, plan.Name)
			}
			partition = plan.Name
		case "sort":
			if sortKey != "" {
				return item, fmt.Errorf("dynamobind: %s: fields %s and %s are both sortkey", name, sortKey, plan.Name)
			}
			sortKey = plan.Name
		}
		item.Fields = append(item.Fields, plan)
	}
	if len(item.Fields) == 0 {
		return item, fmt.Errorf("dynamobind: %s has no mappable exported field", name)
	}
	if sortKey != "" && partition == "" {
		return item, fmt.Errorf("dynamobind: %s: field %s is sortkey without a partitionkey", name, sortKey)
	}
	if err := c.checkMethodCollision(named, item); err != nil {
		return item, err
	}
	return item, nil
}

func (c *dynamoCollector) field(typeName string, field *types.Var, raw string) (DynamoFieldPlan, error) {
	parts := strings.Split(raw, ",")
	plan := DynamoFieldPlan{Name: field.Name(), Attribute: field.Name()}
	if len(parts) > 0 && parts[0] != "" {
		plan.Attribute = parts[0]
	}
	set := DynamoKind("")
	unixTime := false
	for _, option := range parts[1:] {
		switch option {
		case "":
		case "omitempty":
			plan.OmitEmpty = true
		case "partitionkey":
			plan.Key = "partition"
		case "sortkey":
			plan.Key = "sort"
		case "stringset":
			set = DynamoStringSet
		case "numberset":
			set = DynamoNumberSet
		case "binaryset":
			set = DynamoBinarySet
		case "unixtime":
			unixTime = true
		default:
			return plan, fmt.Errorf("dynamobind: %s.%s: unknown %s tag option %q", typeName, field.Name(), dynamoTag, option)
		}
	}
	resolved, err := c.resolve(field.Type(), typeName, field.Name(), set, unixTime)
	if err != nil {
		return plan, err
	}
	plan.Type = resolved
	if plan.Key != "" {
		if _, ok := dynamoKeyAttributeType(resolved); !ok {
			return plan, fmt.Errorf("dynamobind: %s.%s: a %s key must be S, N or B, and %s is not", typeName, field.Name(), plan.Key, resolved.Go)
		}
	}
	return plan, nil
}

// dynamoKeyAttributeType reports the driver key type constant for a field, and
// whether the field can be a key at all. DynamoDB permits only S, N and B.
func dynamoKeyAttributeType(t DynamoType) (string, bool) {
	switch t.Kind {
	case DynamoString, DynamoTime:
		return "TypeString", true
	case DynamoInt, DynamoUint, DynamoFloat, DynamoUnixTime:
		return "TypeNumber", true
	case DynamoBytes:
		return "TypeBinary", true
	default:
		return "", false
	}
}

// checkMethodCollision rejects a type that already declares one of the methods
// the codec emits, by hand. A method the last generation run wrote does not
// count: it is about to be replaced.
func (c *dynamoCollector) checkMethodCollision(named *types.Named, item DynamoItemPlan) error {
	emitted := map[string]bool{"EncodeItem": true, "DecodeItem": true, "ItemKey": true}
	for i := range named.NumMethods() {
		method := named.Method(i)
		if !emitted[method.Name()] || c.wasGenerated(method.Pos()) {
			continue
		}
		return fmt.Errorf("dynamobind: %s already declares %s; the generated codec would collide with it", item.Name, method.Name())
	}
	return nil
}

// wasGenerated reports whether a declaration lives in a file this run skipped.
func (c *dynamoCollector) wasGenerated(pos token.Pos) bool {
	if c.pkg == nil || c.pkg.Fset == nil || !pos.IsValid() {
		return false
	}
	return c.generated[c.pkg.Fset.Position(pos).Filename]
}
