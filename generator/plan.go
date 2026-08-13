package generator

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"

	"github.com/shibukawa/tinybind-go/internal/gensource"
	"github.com/shibukawa/tinybind-go/internal/godoc"
)

// FieldSource is where a request field is read from.
type FieldSource string

const (
	SourceInput   FieldSource = "input"
	SourceQuery   FieldSource = "query"
	SourcePayload FieldSource = "payload"
	SourcePath    FieldSource = "path"
	SourceHeader  FieldSource = "header"
	SourceCookie  FieldSource = "cookie"
	SourceMethod  FieldSource = "method"
)

// Composite and special field kinds.
const (
	KindRestAny = "rest_any" // map[string]any with payload:"*"
	KindRestRaw = "rest_raw" // map[string]json.RawMessage with payload:"*"
	KindStruct  = "struct"
	KindSlice   = "slice"
	KindMap     = "map"
)

// FieldPlan is one struct field mapping plan (compile-time).
type FieldPlan struct {
	Name      string      // Go field name
	Wire      string      // wire / tag name ("*" for payload rest)
	Source    FieldSource // input|query|payload|path|header|cookie|method
	Kind      string      // string|int|int64|bool|float64|file|rest_*|struct|slice|map
	JSON      string      // json name for encode/document keys
	JSONSkip  bool        // json:"-": the JSON codec neither writes nor reads it
	OmitEmpty bool        // json:",omitempty": skip when it would encode as "", [] or {}
	OmitZero  bool        // json:",omitzero": skip when the field holds the Go zero value
	Check     CheckRules  // from check:"" tag; empty if absent
	Enum      EnumRule    // from enum:"" tag; unset if absent
	Default   DefaultRule // from default:"" tag; unset if absent
	TypeName  string      // KindStruct name, or element struct name for slice/map of struct
	ElemKind  string      // for slice/map: string|int|int64|bool|float64|struct
	DB        string      // SQL result column (db tag or snake_case field name)
	GroupKey  bool        // groupkey tag presence
	Doc       string      // godoc of the field (doc or line comment)
}

// HasValidation reports whether anything about the field can reject a bound
// value, across every tag that carries a constraint.
func (f FieldPlan) HasValidation() bool {
	return f.Check.HasValidation() || f.Enum.Set
}

// NeedsPresence is true when codegen must track whether the field was present:
// validation has to skip absent optional values, and a default only applies to
// a field nobody supplied.
func (f FieldPlan) NeedsPresence() bool {
	return f.HasValidation() || f.Default.Set
}

// IsRest reports whether f is a payload rest map field.
func (f FieldPlan) IsRest() bool {
	return f.Kind == KindRestAny || f.Kind == KindRestRaw
}

// IsComposite reports nested struct/slice/map kinds.
func (f FieldPlan) IsComposite() bool {
	return f.Kind == KindStruct || f.Kind == KindSlice || f.Kind == KindMap
}

// GoType returns a Go type string for generated code (e.g. NestedCustomer, []string).
func (f FieldPlan) GoType() string {
	switch f.Kind {
	case KindStruct:
		return f.TypeName
	case KindSlice:
		if f.ElemKind == KindStruct {
			return "[]" + f.TypeName
		}
		return "[]" + f.ElemKind
	case KindMap:
		if f.ElemKind == KindStruct {
			return "map[string]" + f.TypeName
		}
		return "map[string]" + f.ElemKind
	case KindRestAny:
		return "map[string]any"
	case KindRestRaw:
		return "map[string]json.RawMessage"
	case "file":
		return "httpbind.File"
	default:
		return f.Kind
	}
}

const (
	httpbindImportPath = "github.com/shibukawa/tinybind-go"
	jsonbindImportPath = "github.com/shibukawa/tinybind-go/jsonbind"
	sqlbindImportPath  = "github.com/shibukawa/tinybind-go/sqlbind"
)

// TypePlan is the mapping plan for one struct type.
type TypePlan struct {
	Name string
	// SourcePath is the Go file that declares the type. It is the owning source
	// of every artifact generated for this type.
	SourcePath string
	Fields     []FieldPlan
	// Doc is the godoc of the type declaration.
	Doc string
	// Usage records which generated entry points are referenced by source code.
	// Zero means the type is unused and emits no mapping paths.
	Usage Usage
	// DirectUsage excludes usage inherited from containing structs.
	DirectUsage Usage
}

// Usage selects generated mapping entry points.
type Usage uint32

const (
	UsageBind Usage = 1 << iota
	UsageWrite
	UsageDecodeJSON
	UsageEncodeJSON
	UsageScanRows
	UsageEncodeItem
	UsageDecodeItem
	UsageItemKey
	UsageEncodeEntity
	UsageDecodeEntity
	UsageEntityKey
	UsageCacheKey
	UsageAll = UsageBind | UsageWrite | UsageDecodeJSON | UsageEncodeJSON
	// UsageItem is every DynamoDB item entry point. It stays out of UsageAll:
	// the item codec has its own generate-all rule, which requires a dynamo tag,
	// so an unrelated request struct never acquires one.
	UsageItem = UsageEncodeItem | UsageDecodeItem | UsageItemKey
	// UsageEntity is every Firestore entity entry point, and stays out of
	// UsageAll for the same reason, requiring a firestore tag instead.
	UsageEntity = UsageEncodeEntity | UsageDecodeEntity | UsageEntityKey
)

// DiscoverySymbol identifies a generic function and the entry point it needs.
// PackagePath is matched by go/types identity, so import aliases are supported.
type DiscoverySymbol struct {
	PackagePath         string
	Name                string
	ReceiverPackagePath string
	ReceiverType        string
	Usage               Usage
	TypeArgument        int
	ArgumentType        *int
}

// PackagePlan is all type plans in a package.
type PackagePlan struct {
	Package     string
	PackagePath string
	Types       []TypePlan
	// Discovered lists type names referenced by configured generic call sites.
	Discovered []string
}

// AnalyzePackage builds field plans for all package-level structs with exported fields.
// Generic call discovery (Bind/Write/DecodeJSON/EncodeJSON) uses go/types symbol identity.
func AnalyzePackage(dir string) (*PackagePlan, error) {
	opts := DefaultOptions()
	opts.GenerateAll = true
	return AnalyzePackageWithOptions(dir, opts)
}

// AnalyzePackageWithOptions is AnalyzePackage with customizable call targets.
func AnalyzePackageWithOptions(dir string, opts Options) (*PackagePlan, error) {
	return analyzeLoadedPackage(newPackageLoad(dir), opts)
}

// analyzeLoadedPackage builds the plan from a package the run already loaded.
func analyzeLoadedPackage(load *packageLoad, opts Options) (*PackagePlan, error) {
	pkg, err := load.get()
	if err != nil {
		return nil, err
	}

	plan := &PackagePlan{Package: pkg.Name, PackagePath: pkg.PkgPath}
	discovered := map[string]Usage{}
	normalized, err := opts.normalized()
	if err != nil {
		return nil, err
	}
	symbols := normalized.symbols
	fset := pkg.Fset
	for _, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		base, sourcePath := "", ""
		if fset != nil {
			// A file the parser could not read still arrives as a syntax
			// entry, but it reports token.NoPos and the FileSet has no handle
			// for it. Leaving the path empty is what the fset == nil arm above
			// already does, and everything downstream tolerates it.
			if handle := fset.File(f.Pos()); handle != nil {
				sourcePath = handle.Name()
				base = filepath.Base(sourcePath)
			}
		}
		if strings.HasSuffix(base, "_test.go") ||
			strings.HasSuffix(base, "_httpbind_gen.go") ||
			strings.HasSuffix(base, "_openapi_gen.go") ||
			base == "httpbind_gen.go" ||
			base == "httpbind_openapi_gen.go" ||
			base == "tinybind_gen.go" ||
			base == "tinybind_openapi_gen.go" ||
			// Nothing a generation run wrote is an input to what it reads,
			// whatever the output file was named or headed with.
			gensource.IsGenerated(f, normalized.parserConfig.GeneratedHeaders...) {
			continue
		}
		binderNames := configuredTypeNames(f, normalized.fileTypes, pkg.Imports)
		discoveredInFile, err := discoverGenericTypeArgs(f, pkg.TypesInfo, symbols)
		if err != nil {
			return nil, err
		}
		for name, usage := range discoveredInFile {
			discovered[name] |= usage
		}
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
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				// An ungrouped declaration carries its doc on the GenDecl;
				// a group's own doc describes the group, not each spec.
				var declDoc *ast.CommentGroup
				if len(gd.Specs) == 1 {
					declDoc = gd.Doc
				}
				tp, ok, err := analyzeStruct(ts.Name.Name, godoc.Text(ts.Doc, declDoc), st, binderNames)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", ts.Name.Name, err)
				}
				if ok {
					tp.SourcePath = sourcePath
					plan.Types = append(plan.Types, tp)
				}
			}
		}
	}
	for name := range discovered {
		plan.Discovered = append(plan.Discovered, name)
	}
	for i := range plan.Types {
		plan.Types[i].Usage = discovered[plan.Types[i].Name]
		plan.Types[i].DirectUsage = plan.Types[i].Usage
	}
	if opts.GenerateAll {
		for i := range plan.Types {
			plan.Types[i].Usage |= UsageAll & normalized.enabledUsage
			plan.Types[i].DirectUsage |= UsageAll & normalized.enabledUsage
		}
	}
	propagateNestedUsage(plan.Types)
	return plan, nil
}

func configuredTypeNames(f *ast.File, configured []TypePattern, imports map[string]*packages.Package) map[string]bool {
	out := map[string]bool{}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		local := filepath.Base(path)
		if imported := imports[path]; imported != nil && imported.Name != "" {
			local = imported.Name
		}
		if imp.Name != nil {
			local = imp.Name.Name
		}
		if local == "_" || local == "." {
			continue
		}
		for _, q := range configured {
			if path == q.PackagePath {
				out[local+"."+q.Name] = true
			}
		}
	}
	return out
}

// discoverGenericTypeArgs finds type arguments of httpbind Bind/Write/DecodeJSON/EncodeJSON
// using go/types-resolved function identity (import-alias safe).
func discoverGenericTypeArgs(f *ast.File, info *types.Info, symbols []DiscoverySymbol) (map[string]Usage, error) {
	out := map[string]Usage{}
	if f == nil || info == nil {
		return out, nil
	}
	var discoveryErr error
	ast.Inspect(f, func(n ast.Node) bool {
		if discoveryErr != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		obj := objectOfCall(info, call.Fun)
		for _, symbol := range symbols {
			if !discoverySymbolMatches(obj, symbol) {
				continue
			}
			signature, _ := obj.Type().(*types.Signature)
			if symbol.ArgumentType != nil {
				if signature == nil || signature.Params().Len() <= *symbol.ArgumentType {
					discoveryErr = fmt.Errorf("generator: call pattern %s.%s argument_type index %d exceeds wrapper signature", symbol.PackagePath, symbol.Name, *symbol.ArgumentType)
					return false
				}
			} else if signature == nil || signature.TypeParams().Len() <= symbol.TypeArgument {
				discoveryErr = fmt.Errorf("generator: call pattern %s.%s generic_argument index %d exceeds wrapper signature", symbol.PackagePath, symbol.Name, symbol.TypeArgument)
				return false
			}
			args := genericTypeArgExprs(call.Fun)
			if symbol.ArgumentType != nil {
				if len(call.Args) > *symbol.ArgumentType {
					if name := argumentTypeName(info.TypeOf(call.Args[*symbol.ArgumentType])); name != "" {
						out[name] |= symbol.Usage
					}
				}
				continue
			}
			if len(args) > symbol.TypeArgument {
				if name := namedTypeName(info.TypeOf(args[symbol.TypeArgument])); name != "" {
					out[name] |= symbol.Usage
				}
				continue
			}
			if name := instantiatedTypeNameAt(info, call.Fun, symbol.TypeArgument); name != "" {
				out[name] |= symbol.Usage
			}
		}
		return true
	})
	return out, discoveryErr
}

func discoverySymbolMatches(obj types.Object, symbol DiscoverySymbol) bool {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != symbol.PackagePath || fn.Name() != symbol.Name {
		return false
	}
	signature, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}
	if symbol.ReceiverType == "" {
		return signature.Recv() == nil
	}
	if signature.Recv() == nil {
		return false
	}
	named, ok := unaliasPtr(signature.Recv().Type()).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == symbol.ReceiverPackagePath && named.Obj().Name() == symbol.ReceiverType
}

func instantiatedTypeNameAt(info *types.Info, fun ast.Expr, index int) string {
	return namedTypeName(instantiatedTypeArgAt(info, fun, index))
}

// instantiatedTypeArgAt returns the type argument inferred for an instantiated
// generic call, for the calls that spell no explicit type argument.
func instantiatedTypeArgAt(info *types.Info, fun ast.Expr, index int) types.Type {
	for {
		switch e := fun.(type) {
		case *ast.ParenExpr:
			fun = e.X
		case *ast.IndexExpr:
			fun = e.X
		case *ast.IndexListExpr:
			fun = e.X
		case *ast.SelectorExpr:
			if inst, ok := info.Instances[e.Sel]; ok && inst.TypeArgs.Len() > index {
				return inst.TypeArgs.At(index)
			}
			return nil
		case *ast.Ident:
			if inst, ok := info.Instances[e]; ok && inst.TypeArgs.Len() > index {
				return inst.TypeArgs.At(index)
			}
			return nil
		default:
			return nil
		}
	}
}

// argumentTypeName is namedTypeName through one slice or array, so a call that
// takes a batch of values discovers the element type. A scalar argument behaves
// exactly as before.
func argumentTypeName(t types.Type) string {
	if name := namedTypeName(t); name != "" {
		return name
	}
	switch collection := types.Unalias(t).(type) {
	case *types.Slice:
		return namedTypeName(collection.Elem())
	case *types.Array:
		return namedTypeName(collection.Elem())
	}
	return ""
}

func namedTypeName(t types.Type) string {
	if n, ok := unaliasPtr(t).(*types.Named); ok && n.Obj() != nil {
		return n.Obj().Name()
	}
	return ""
}

// unaliasPtr resolves type aliases and strips one pointer indirection. Since
// Go 1.24 the go/types default is gotypesalias=1, so an alias arrives as
// *types.Alias and passes no *types.Named assertion on its own; every named
// type test here goes through this helper so an alias behaves as the type it
// names. A defined type stays itself: only aliases are transparent.
func unaliasPtr(t types.Type) types.Type {
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	return t
}

func propagateNestedUsage(plans []TypePlan) {
	index := make(map[string]int, len(plans))
	for i := range plans {
		index[plans[i].Name] = i
	}
	changed := true
	for changed {
		changed = false
		for i := range plans {
			u := plans[i].Usage
			var nested Usage
			if u&(UsageBind|UsageDecodeJSON) != 0 {
				nested |= UsageDecodeJSON
			}
			if u&(UsageWrite|UsageEncodeJSON) != 0 {
				nested |= UsageEncodeJSON
			}
			if u&UsageScanRows != 0 {
				nested |= UsageScanRows
			}
			for _, f := range plans[i].Fields {
				if f.TypeName == "" {
					continue
				}
				j, ok := index[f.TypeName]
				if ok && plans[j].Usage|nested != plans[j].Usage {
					plans[j].Usage |= nested
					changed = true
				}
			}
		}
	}
}

func objectOfCall(info *types.Info, fun ast.Expr) types.Object {
	if info == nil || fun == nil {
		return nil
	}
	for {
		switch e := fun.(type) {
		case *ast.ParenExpr:
			fun = e.X
			continue
		case *ast.IndexExpr:
			fun = e.X
			continue
		case *ast.IndexListExpr:
			fun = e.X
			continue
		case *ast.Ident:
			return info.Uses[e]
		case *ast.SelectorExpr:
			if sel, ok := info.Selections[e]; ok && sel != nil {
				return sel.Obj()
			}
			if e.Sel != nil {
				return info.Uses[e.Sel]
			}
			return nil
		default:
			return nil
		}
	}
}

func genericTypeArgExprs(fun ast.Expr) []ast.Expr {
	switch f := fun.(type) {
	case *ast.IndexExpr:
		return []ast.Expr{f.Index}
	case *ast.IndexListExpr:
		return f.Indices
	default:
		return nil
	}
}

func analyzeStruct(name, doc string, st *ast.StructType, binderNames map[string]bool) (TypePlan, bool, error) {
	var fields []FieldPlan
	restCount := 0
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue // embedded
		}
		for _, id := range f.Names {
			if id == nil || !exported(id.Name) {
				continue
			}
			src, wire := parseFieldTag(id.Name, f.Tag)
			fp, ok, err := analyzeField(id.Name, godoc.Text(f.Doc, f.Comment), f.Type, f.Tag, src, wire, binderNames)
			if err != nil {
				return TypePlan{}, false, err
			}
			if !ok {
				continue
			}
			if fp.Kind == "file" {
				switch src {
				case SourceInput, SourcePayload:
					fp.Source = SourcePayload
				default:
					return TypePlan{}, false, fmt.Errorf("field %s: httpbind.File only supports payload/input tags, got %s", id.Name, src)
				}
			}
			if fp.IsRest() {
				fp.Source = SourcePayload
				fp.Wire = "*"
				restCount++
				if restCount > 1 {
					return TypePlan{}, false, fmt.Errorf("field %s: at most one payload:\"*\" rest field allowed", id.Name)
				}
			}
			// Nested composites are JSON-oriented; force payload when tagged input for body nesting.
			if fp.IsComposite() {
				switch fp.Source {
				case SourceInput, SourcePayload:
					// keep; JSON bind uses body
				case SourceQuery, SourcePath, SourceHeader, SourceCookie, SourceMethod:
					return TypePlan{}, false, fmt.Errorf("field %s: nested %s only supports payload/input sources", id.Name, fp.Kind)
				}
			}
			fields = append(fields, fp)
		}
	}
	if len(fields) == 0 {
		return TypePlan{}, false, nil
	}
	return TypePlan{Name: name, Doc: doc, Fields: fields}, true, nil
}

func analyzeField(fieldName, doc string, typ ast.Expr, tag *ast.BasicLit, src FieldSource, wire string, binderNames map[string]bool) (FieldPlan, bool, error) {
	kind, typeName, elemKind, ok, err := fieldTypeKind(typ, binderNames, src, wire, fieldName)
	if err != nil {
		return FieldPlan{}, false, err
	}
	if !ok {
		return FieldPlan{}, false, nil
	}
	jsonName := wire
	if jsonName == "" || jsonName == "*" {
		jsonName = lowerFirst(fieldName)
	}
	jsonSkip, jsonTagName, omitEmpty, omitZero, err := parseJSONTag(tagValue(tag, "json"))
	if err != nil {
		return FieldPlan{}, false, fmt.Errorf("field %s: %w", fieldName, err)
	}
	if jsonTagName != "" {
		jsonName = jsonTagName
	}
	checkRaw := tagValue(tag, "check")
	check, err := ParseCheckTag(checkRaw, kind)
	if err != nil {
		return FieldPlan{}, false, fmt.Errorf("field %s: %w", fieldName, err)
	}
	var enum EnumRule
	if enumRaw, ok := tagLookup(tag, "enum"); ok {
		enum, err = ParseEnumTag(enumRaw, kind)
		if err != nil {
			return FieldPlan{}, false, fmt.Errorf("field %s: %w", fieldName, err)
		}
	}
	// Looked up rather than read, so that default:"" is an empty-string default
	// and not the same thing as carrying no default tag at all.
	var def DefaultRule
	if defRaw, ok := tagLookup(tag, "default"); ok {
		def, err = ParseDefaultTag(defRaw, kind)
		if err != nil {
			return FieldPlan{}, false, fmt.Errorf("field %s: %w", fieldName, err)
		}
	}
	return FieldPlan{
		Name:      fieldName,
		Wire:      wire,
		Source:    src,
		Kind:      kind,
		JSON:      jsonName,
		JSONSkip:  jsonSkip,
		OmitEmpty: omitEmpty,
		OmitZero:  omitZero,
		Check:     check,
		Enum:      enum,
		Default:   def,
		TypeName:  typeName,
		ElemKind:  elemKind,
		DB:        dbColumn(fieldName, tag),
		GroupKey:  tagPresent(tag, "groupkey"),
		Doc:       doc,
	}, true, nil
}

// parseJSONTag splits a json tag into the parts the codec acts on. The name and
// the option list follow encoding/json, including its one piece of punctuation
// trivia: a bare "-" excludes the field, while "-," names it "-".
//
// Only omitempty and omitzero are recognised as options; an unknown one is an
// error rather than a tag that silently does nothing, since a misspelled option
// looks exactly like a working one until someone diffs the output.
func parseJSONTag(raw string) (skip bool, name string, omitEmpty, omitZero bool, err error) {
	if raw == "" {
		return false, "", false, false, nil
	}
	if raw == "-" {
		return true, "", false, false, nil
	}
	parts := strings.Split(raw, ",")
	for _, opt := range parts[1:] {
		switch opt {
		case "omitempty":
			omitEmpty = true
		case "omitzero":
			omitZero = true
		case "":
			// A trailing or doubled comma carries no option.
		default:
			return false, "", false, false, fmt.Errorf("unknown json tag option %q", opt)
		}
	}
	return false, parts[0], omitEmpty, omitZero, nil
}

func dbColumn(fieldName string, tag *ast.BasicLit) string {
	if v := tagValue(tag, "db"); v != "" {
		return strings.Split(v, ",")[0]
	}
	var b strings.Builder
	for i, r := range fieldName {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func tagPresent(tag *ast.BasicLit, key string) bool {
	if tag == nil {
		return false
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return false
	}
	for _, part := range strings.Fields(raw) {
		k, _, ok := strings.Cut(part, ":")
		if ok && k == key {
			return true
		}
	}
	return false
}

func exported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// fieldTypeKind resolves a field's bind kind.
func fieldTypeKind(expr ast.Expr, binderNames map[string]bool, src FieldSource, wire, fieldName string) (kind, typeName, elemKind string, ok bool, err error) {
	if restKind, isRest := mapRestKind(expr); isRest {
		if wire != "*" {
			return "", "", "", false, nil
		}
		if src != SourcePayload {
			return "", "", "", false, fmt.Errorf("field %s: rest map requires payload:\"*\", got %s:%q", fieldName, src, wire)
		}
		return restKind, "", "", true, nil
	}
	if wire == "*" {
		return "", "", "", false, fmt.Errorf("field %s: payload:\"*\" requires map[string]any or map[string]json.RawMessage", fieldName)
	}

	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string", "int", "int64", "bool", "float64":
			return t.Name, "", "", true, nil
		case "any", "error":
			return "", "", "", false, nil
		default:
			// Named type in the same package → nested struct.
			if t.Name != "" {
				return KindStruct, t.Name, "", true, nil
			}
		}
	case *ast.SelectorExpr:
		if t.Sel != nil {
			if pkg, ok := t.X.(*ast.Ident); ok && binderNames[pkg.Name+"."+t.Sel.Name] {
				return "file", "", "", true, nil
			}
		}
	case *ast.ArrayType:
		ek, et, _, eok, eerr := fieldTypeKind(t.Elt, binderNames, src, wire, fieldName)
		if eerr != nil {
			return "", "", "", false, eerr
		}
		if !eok {
			return "", "", "", false, nil
		}
		switch ek {
		case "string", "int", "int64", "bool", "float64":
			return KindSlice, "", ek, true, nil
		case KindStruct:
			return KindSlice, et, KindStruct, true, nil
		default:
			return "", "", "", false, nil
		}
	case *ast.MapType:
		key, ok := t.Key.(*ast.Ident)
		if !ok || key.Name != "string" {
			return "", "", "", false, nil
		}
		ek, et, _, eok, eerr := fieldTypeKind(t.Value, binderNames, src, wire, fieldName)
		if eerr != nil {
			return "", "", "", false, eerr
		}
		if !eok {
			return "", "", "", false, nil
		}
		switch ek {
		case "string", "int", "int64", "bool", "float64":
			return KindMap, "", ek, true, nil
		case KindStruct:
			return KindMap, et, KindStruct, true, nil
		default:
			return "", "", "", false, nil
		}
	}
	return "", "", "", false, nil
}

func mapRestKind(expr ast.Expr) (string, bool) {
	mt, ok := expr.(*ast.MapType)
	if !ok {
		return "", false
	}
	key, ok := mt.Key.(*ast.Ident)
	if !ok || key.Name != "string" {
		return "", false
	}
	switch v := mt.Value.(type) {
	case *ast.Ident:
		if v.Name == "any" {
			return KindRestAny, true
		}
	case *ast.InterfaceType:
		if v.Methods == nil || len(v.Methods.List) == 0 {
			return KindRestAny, true
		}
	case *ast.SelectorExpr:
		if v.Sel != nil && v.Sel.Name == "RawMessage" {
			if pkg, ok := v.X.(*ast.Ident); ok && pkg.Name == "json" {
				return KindRestRaw, true
			}
		}
	}
	return "", false
}

func parseFieldTag(fieldName string, tag *ast.BasicLit) (FieldSource, string) {
	defaultWire := lowerFirst(fieldName)
	if tag == nil {
		return SourceInput, defaultWire
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return SourceInput, defaultWire
	}
	for _, src := range []FieldSource{SourceInput, SourceQuery, SourcePayload, SourcePath, SourceHeader, SourceCookie, SourceMethod} {
		if v := lookupTag(raw, string(src)); v != "" {
			if v == "-" {
				continue
			}
			return src, v
		}
	}
	return SourceInput, defaultWire
}

func tagValue(tag *ast.BasicLit, key string) string {
	if tag == nil {
		return ""
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return ""
	}
	return lookupTag(raw, key)
}

// tagLookup reports the value of key and whether the tag carried it at all,
// which tagValue cannot express for tags whose empty value is meaningful.
func tagLookup(tag *ast.BasicLit, key string) (string, bool) {
	if tag == nil {
		return "", false
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return "", false
	}
	for _, part := range strings.Fields(raw) {
		k, v, ok := strings.Cut(part, ":")
		if !ok || k != key {
			continue
		}
		val, err := strconv.Unquote(v)
		if err != nil {
			return strings.Trim(v, `"`), true
		}
		return val, true
	}
	return "", false
}

func lookupTag(raw, key string) string {
	for _, part := range strings.Fields(raw) {
		k, v, ok := strings.Cut(part, ":")
		if !ok || k != key {
			continue
		}
		val, err := strconv.Unquote(v)
		if err != nil {
			return strings.Trim(v, `"`)
		}
		return val
	}
	return ""
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}
