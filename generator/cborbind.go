package generator

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/shibukawa/tinybind-go/internal/gensource"
)

const (
	cborbindImportPath = "github.com/shibukawa/tinybind-go/cborbind"
	cborImportPath     = "github.com/shibukawa/tinygodriver/encoding/cbor"
	// cborTag names the field option. It is short and names its purpose, as
	// check, db, dynamo and firestore do.
	cborTag = "cbor"
)

// CborUsage selects which halves of a codec a type needs.
type CborUsage uint8

const (
	// CborEncode emits the append function, and AppendCBORTo for a declared type.
	CborEncode CborUsage = 1 << iota
	// CborDecode emits the decode function, and DecodeCBORFrom for a declared type.
	CborDecode
	// CborDelta emits the delta type, its codec, the diff and the apply.
	CborDelta
)

// CborFieldPlan is one struct field and what it encodes as.
type CborFieldPlan struct {
	Name string
	// Key is the world-profile map key. It is the Go field name unless a tag
	// renames it, and it is unused under the wire profile, where a field is
	// identified by its position and nothing else.
	Key string
	// IntKey is the world-profile integer key, when the type numbers its
	// fields. A numbering is all or nothing, so this is meaningful exactly when
	// the containing plan says IntKeys.
	IntKey uint64
	// Identity marks the field that names the entity this struct is. It takes
	// no mask bit in a delta and is carried by the collection instead: an
	// identity that changed is a different entity rather than a changed one.
	Identity bool
	Type     CborType
}

// CborTypePlan is the codec plan for one struct type under one profile.
type CborTypePlan struct {
	Name       string
	SourcePath string
	Doc        string
	Profile    CborProfile
	Usage      CborUsage
	// Declared is true when an annotation named this type, which is what
	// publishes AppendCBORTo and DecodeCBORFrom onto it. A type reached only as
	// a nested field gets the functions and no methods: a public method is code
	// size in every binary carrying the type, and nothing asked for it.
	Declared bool
	// IntKeys is set when every field of a world-profile type declares a
	// number, so the map is keyed by integers rather than by text.
	IntKeys bool
	// Delta is set when a declaration asked for the diff and apply of this
	// type, or of one that reaches it.
	Delta bool
	// IdentityField is the name of the field declaring the identity, and is
	// empty for a type that declares none. A root needs none; an element of a
	// diffable collection needs one.
	IdentityField string
	Fields        []CborFieldPlan
}

// CborPackagePlan is every CBOR codec plan in one package.
type CborPackagePlan struct {
	Package     string
	PackagePath string
	Types       []CborTypePlan
	// Schema is the canonical description of everything above that reaches the
	// wire, and Version is its digest. Both are emitted: a version mismatch is
	// then diagnosed by diffing two schemas rather than by observing that two
	// opaque numbers differ.
	Schema  string
	Version string
	// WholeCollections names every collection a delta carries whole because its
	// element type declares no identity. It is written into the generated file:
	// a delta the size of a snapshot otherwise looks like the feature working.
	WholeCollections []string
}

// AnalyzeCborCodecs builds codec plans for the types a package declares a CBOR
// codec for.
func AnalyzeCborCodecs(dir string) (*CborPackagePlan, error) {
	return AnalyzeCborCodecsWithOptions(dir, DefaultOptions())
}

// AnalyzeCborCodecsWithOptions is AnalyzeCborCodecs with custom discovery.
func AnalyzeCborCodecsWithOptions(dir string, opts Options) (*CborPackagePlan, error) {
	return analyzeCborCodecs(newPackageLoad(dir), opts)
}

func analyzeCborCodecs(load *packageLoad, opts Options) (*CborPackagePlan, error) {
	pkg, err := load.get()
	if err != nil {
		return nil, err
	}
	normalized, err := opts.normalized()
	if err != nil {
		return nil, err
	}
	plan := &CborPackagePlan{Package: pkg.Name, PackagePath: pkg.PkgPath}
	symbols := cborSymbols(normalized.symbols)
	if len(symbols) == 0 {
		return plan, nil
	}

	// A codec is declaration-driven and nothing else. There is no generate-all
	// rule and no tag that implies one: a CBOR codec is a protocol, and giving
	// every struct in a package one would publish a wire format nobody declared.
	usage := map[cborKey]CborUsage{}
	sources := map[string]string{}
	generated := map[string]bool{}
	for _, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		if skipCborFile(f, pkg, normalized) {
			if name := dynamoFileName(pkg, f); name != "" {
				generated[name] = true
			}
			continue
		}
		discovered, err := discoverGenericTypeArgs(f, pkg.TypesInfo, symbols, pkg.PkgPath)
		if err != nil {
			return nil, err
		}
		for name, bits := range discovered {
			for key, use := range cborUsageOf(name, bits) {
				usage[key] |= use
			}
		}
		for name := range declaredStructNames(f) {
			if path := dynamoFileName(pkg, f); path != "" {
				sources[name] = path
			}
		}
	}
	if len(usage) == 0 {
		return plan, nil
	}

	keys := make([]cborKey, 0, len(usage))
	for key := range usage {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		return keys[i].profile < keys[j].profile
	})

	// A type declared under both profiles cannot publish two AppendCBORTo
	// methods, so it is refused here rather than emitted as a file that does
	// not compile inside a DO NOT EDIT header.
	profiles := map[string][]CborProfile{}
	for _, key := range keys {
		profiles[key.name] = append(profiles[key.name], key.profile)
	}
	for _, key := range keys {
		if len(profiles[key.name]) > 1 {
			return nil, fmt.Errorf(
				"cborbind: %s is declared for both the wire and the world profile, and one type cannot publish two AppendCBORTo methods; declare a separate type per profile",
				key.name)
		}
	}

	collector := &cborCollector{pkg: pkg, plans: map[cborKey]*CborTypePlan{}, generated: generated}
	for _, key := range keys {
		if _, ok := sources[key.name]; !ok {
			return nil, fmt.Errorf(
				"cborbind: a CBOR codec is declared for %s, which this package does not declare; move the declaration to the package that declares the type",
				key.name)
		}
		// A declared type carrying its own methods already has a codec, and
		// emitting a second one would either fail to compile or quietly replace
		// what the author wrote. Either way the declaration is the thing to
		// remove.
		obj := pkg.Types.Scope().Lookup(key.name)
		if obj == nil {
			return nil, fmt.Errorf("cborbind: %s is not declared in this package", key.name)
		}
		if canAppend, canDecode := collector.selfCodec(obj.Type()); canAppend || canDecode {
			return nil, fmt.Errorf(
				"cborbind: %s already carries its own CBOR codec, so the declaration would generate a second one; remove the declaration, or remove the hand-written method",
				key.name)
		}
		collector.profile, collector.usage = key.profile, usage[key]
		collector.delta = usage[key]&CborDelta != 0
		if err := collector.collect(key.name); err != nil {
			return nil, err
		}
		item := collector.plans[key]
		item.Declared = true
		item.Usage |= usage[key]
	}

	for _, key := range collector.order {
		item := collector.plans[key]
		item.SourcePath = sources[item.Name]
		plan.Types = append(plan.Types, *item)
	}
	sort.SliceStable(plan.Types, func(i, j int) bool {
		if plan.Types[i].Name != plan.Types[j].Name {
			return plan.Types[i].Name < plan.Types[j].Name
		}
		return plan.Types[i].Profile < plan.Types[j].Profile
	})
	plan.WholeCollections = collector.whole
	plan.Schema, plan.Version = cborSchemaAndVersion(plan)
	return plan, nil
}

// cborKey identifies one plan. A nested struct reached from a wire root and
// from a world root is two codecs, because the two profiles disagree about the
// container it encodes as; keying on the pair is what lets both exist.
type cborKey struct {
	name    string
	profile CborProfile
}

// cborSymbols keeps the discovery symbols whose usage is a CBOR declaration.
func cborSymbols(symbols []DiscoverySymbol) []DiscoverySymbol {
	var out []DiscoverySymbol
	for _, symbol := range symbols {
		if symbol.Usage&UsageCBOR != 0 {
			out = append(out, symbol)
		}
	}
	return out
}

func cborUsageOf(name string, usage Usage) map[cborKey]CborUsage {
	out := map[cborKey]CborUsage{}
	if usage&UsageCBORWireEncode != 0 {
		out[cborKey{name, CborWire}] |= CborEncode
	}
	if usage&UsageCBORWireDecode != 0 {
		out[cborKey{name, CborWire}] |= CborDecode
	}
	if usage&UsageCBORWorldEncode != 0 {
		out[cborKey{name, CborWorld}] |= CborEncode
	}
	if usage&UsageCBORWorldDecode != 0 {
		out[cborKey{name, CborWorld}] |= CborDecode
	}
	if usage&UsageCBORWireDelta != 0 {
		out[cborKey{name, CborWire}] |= CborDelta
	}
	if usage&UsageCBORWorldDelta != 0 {
		out[cborKey{name, CborWorld}] |= CborDelta
	}
	return out
}

// skipCborFile drops the inputs no discovery pass may read.
func skipCborFile(f *ast.File, pkg *packages.Package, normalized normalizedOptions) bool {
	base := filepath.Base(dynamoFileName(pkg, f))
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_httpbind_gen.go") ||
		strings.HasSuffix(base, "_openapi_gen.go") ||
		base == defaultCborOut ||
		base == defaultDynamoOut ||
		base == defaultDynamoQueryOut ||
		base == "tinybind_gen.go" ||
		base == "tinybind_openapi_gen.go" ||
		gensource.IsGenerated(f, normalized.parserConfig.GeneratedHeaders...)
}

// cborCollector walks the types reachable from a declaration.
//
// Reachability is the definition of the checked set: no annotation marks it,
// and every type a codec can touch is one whose encoding has to be
// reproducible. That is why resolve refuses rather than skips.
type cborCollector struct {
	pkg       *packages.Package
	plans     map[cborKey]*CborTypePlan
	order     []cborKey
	generated map[string]bool
	profile   CborProfile
	// usage is the direction set the root being collected asked for. It rides
	// down to every nested type, because emitting a decoder for a message the
	// client only ever sends is code size on a wasm target -- which is the
	// whole reason the annotations name a direction.
	usage CborUsage
	// delta says the root being collected asked for a diff and an apply, which
	// rides down to every nested type the same way the directions do.
	delta bool
	// whole names the collections carried whole for want of an identity, in the
	// order they were met, so the generated file can say which they were.
	whole []string
	// active guards against a type that reaches itself. A recursive struct has
	// no fixed shape under either profile, and without this the collector would
	// recurse until the stack ran out.
	active map[cborKey]bool
}

func (c *cborCollector) collect(name string) error {
	key := cborKey{name, c.profile}
	if done, ok := c.plans[key]; ok {
		// Reached again from a second root. A nested type shared by an
		// encode-only root and a decode-only one needs both halves, so the
		// directions accumulate rather than the first one winning.
		done.Usage |= c.usage
		return nil
	}
	if c.active == nil {
		c.active = map[cborKey]bool{}
	}
	if c.active[key] {
		return fmt.Errorf("cborbind: %s reaches itself, and a recursive type has no fixed shape to generate a codec for", name)
	}
	c.active[key] = true
	defer delete(c.active, key)

	obj := c.pkg.Types.Scope().Lookup(name)
	if obj == nil {
		return fmt.Errorf("cborbind: %s is not declared in this package", name)
	}
	structType, ok := obj.Type().Underlying().(*types.Struct)
	if !ok {
		return fmt.Errorf("cborbind: %s is %s underneath, and a codec is generated for a struct", name, obj.Type().Underlying())
	}

	item := &CborTypePlan{Name: name, Profile: c.profile, Usage: c.usage, Delta: c.delta, Doc: cborDoc(c.pkg, name)}
	numbered, unnumbered := 0, 0
	seenKeys := map[string]bool{}
	seenNums := map[uint64]bool{}
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		if !field.Exported() {
			continue
		}
		tag := reflect.StructTag(structType.Tag(i)).Get(cborTag)
		wire, options, err := parseCborTag(name, field.Name(), tag)
		if err != nil {
			return err
		}
		if wire == "-" {
			continue
		}
		resolved, err := c.resolve(field.Type(), name, field.Name())
		if err != nil {
			return err
		}
		plan := CborFieldPlan{Name: field.Name(), Key: field.Name(), Type: resolved}
		if wire != "" {
			plan.Key = wire
		}
		if options.identity {
			if item.IdentityField != "" {
				return fmt.Errorf("cborbind: %s declares two identity fields, %s and %s; an entity has one name",
					name, item.IdentityField, field.Name())
			}
			switch resolved.Kind {
			case CborUint, CborInt, CborText:
			default:
				return fmt.Errorf("cborbind: %s.%s: an identity is compared and sorted, so it must be an integer or a string, not %s",
					name, field.Name(), resolved.Go)
			}
			item.IdentityField = field.Name()
			plan.Identity = true
		}
		if options.hasKey {
			plan.IntKey = options.key
			numbered++
			if seenNums[options.key] {
				return fmt.Errorf("cborbind: %s.%s: key %d is already used by another field", name, field.Name(), options.key)
			}
			seenNums[options.key] = true
		} else {
			unnumbered++
		}
		if seenKeys[plan.Key] {
			return fmt.Errorf("cborbind: %s.%s: map key %q is already used by another field", name, field.Name(), plan.Key)
		}
		seenKeys[plan.Key] = true
		item.Fields = append(item.Fields, plan)
	}
	if len(item.Fields) == 0 {
		return fmt.Errorf("cborbind: %s has no encodable field", name)
	}
	// A numbering is all or nothing. A map half keyed by integers and half by
	// text is legal CBOR and an unreadable schema, and the two ends would have
	// to agree about which fields are which by reading this generator.
	if numbered > 0 && unnumbered > 0 {
		return fmt.Errorf("cborbind: %s numbers some fields and not others; give every field a key option, or none", name)
	}
	item.IntKeys = numbered > 0 && c.profile == CborWorld
	if c.profile == CborWorld {
		sortCborFields(item)
	}

	c.plans[key] = item
	c.order = append(c.order, key)
	if c.delta {
		// A collection whose element names no identity is carried whole, which
		// is legal and is sometimes right. It is recorded rather than warned
		// about, because the cost only matters against the size of the
		// collection and this pass does not know that.
		for _, f := range item.Fields {
			if f.Type.Kind == CborSlice && f.Type.Elem.Kind == CborStruct && f.Type.ElemIdentity == "" {
				c.whole = append(c.whole, name+"."+f.Name+" ("+f.Type.Elem.Struct+")")
			}
		}
	}
	return nil
}

// cborTagOptions is what a cbor tag carries beyond the wire name.
type cborTagOptions struct {
	hasKey   bool
	key      uint64
	identity bool
}

// parseCborTag reads one cbor struct tag. An unknown option is a generation
// error rather than a silently inert tag: a tag the author believed was doing
// something is worse than no tag at all.
func parseCborTag(typeName, fieldName, tag string) (string, cborTagOptions, error) {
	var out cborTagOptions
	if tag == "" {
		return "", out, nil
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	for _, option := range parts[1:] {
		option = strings.TrimSpace(option)
		if option == "" {
			continue
		}
		if option == "identity" {
			out.identity = true
			continue
		}
		value, ok := strings.CutPrefix(option, "key=")
		if !ok {
			return "", out, fmt.Errorf("cborbind: %s.%s: unknown cbor tag option %q", typeName, fieldName, option)
		}
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return "", out, fmt.Errorf("cborbind: %s.%s: cbor tag key must be a non-negative integer, got %q", typeName, fieldName, value)
		}
		out.hasKey, out.key = true, n
	}
	return name, out, nil
}

// sortCborFields puts a world-profile type's fields in the order its map keys
// are emitted in: bytewise over the encoded key, which is the core
// deterministic encoding of RFC 8949 and the order the driver's World profile
// enforces. Sorting here rather than at emission means the plan, the schema
// description and the bytes all agree.
func sortCborFields(item *CborTypePlan) {
	sort.SliceStable(item.Fields, func(i, j int) bool {
		return string(cborEncodedKey(item, item.Fields[i])) < string(cborEncodedKey(item, item.Fields[j]))
	})
}

// cborEncodedKey is the key exactly as it reaches the wire, which is what the
// ordering compares.
func cborEncodedKey(item *CborTypePlan, field CborFieldPlan) []byte {
	if item.IntKeys {
		return cborAppendHead(nil, 0, field.IntKey)
	}
	return append(cborAppendHead(nil, 3, uint64(len(field.Key))), field.Key...)
}

// cborAppendHead writes one CBOR head in its shortest form, mirroring the
// driver's own encoder. It exists here because the ordering above has to know
// the bytes before anything is generated, not because generated code needs it.
func cborAppendHead(dst []byte, major byte, n uint64) []byte {
	switch {
	case n < 24:
		return append(dst, major<<5|byte(n))
	case n <= 0xff:
		return append(dst, major<<5|24, byte(n))
	case n <= 0xffff:
		return append(dst, major<<5|25, byte(n>>8), byte(n))
	case n <= 0xffffffff:
		return append(dst, major<<5|26, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	default:
		return append(dst, major<<5|27,
			byte(n>>56), byte(n>>48), byte(n>>40), byte(n>>32),
			byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
}

// cborDoc is the godoc of a type declaration, used to head the generated codec.
func cborDoc(pkg *packages.Package, name string) string {
	for _, f := range pkg.Syntax {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil || ts.Name.Name != name {
					continue
				}
				if ts.Doc != nil {
					return strings.TrimSpace(ts.Doc.Text())
				}
				if gen.Doc != nil {
					return strings.TrimSpace(gen.Doc.Text())
				}
			}
		}
	}
	return ""
}
