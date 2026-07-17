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
	Name     string      // Go field name
	Wire     string      // wire / tag name ("*" for payload rest)
	Source   FieldSource // input|query|payload|path|header|cookie|method
	Kind     string      // string|int|int64|bool|float64|file|rest_*|struct|slice|map
	JSON     string      // json name for encode/document keys
	Check    CheckRules  // from check:"" tag; empty if absent
	TypeName string      // KindStruct name, or element struct name for slice/map of struct
	ElemKind string      // for slice/map: string|int|int64|bool|float64|struct
	DB       string      // SQL result column (db tag or snake_case field name)
	GroupKey bool        // groupkey tag presence
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
		return "httpbinder.File"
	default:
		return f.Kind
	}
}

// httpbinderImportPath is the module path of this library (for recognizing File).
const httpbinderImportPath = "github.com/shibukawa/httpbind-go"

// TypePlan is the mapping plan for one struct type.
type TypePlan struct {
	Name   string
	Fields []FieldPlan
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
	UsageAll = UsageBind | UsageWrite | UsageDecodeJSON | UsageEncodeJSON
)

// DiscoverySymbol identifies a generic function and the entry point it needs.
// PackagePath is matched by go/types identity, so import aliases are supported.
type DiscoverySymbol struct {
	PackagePath string
	Name        string
	Usage       Usage
}

// PackagePlan is all type plans in a package.
type PackagePlan struct {
	Package string
	Types   []TypePlan
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
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
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
	for _, p := range pkgs {
		if p.Name != "" && !strings.HasSuffix(p.ID, ".test") {
			pkg = p
			break
		}
	}
	if pkg.TypesInfo == nil {
		return nil, fmt.Errorf("type-check failed for %s: %v", abs, pkg.Errors)
	}

	plan := &PackagePlan{Package: pkg.Name}
	discovered := map[string]Usage{}
	normalized := opts.normalized()
	symbols := normalized.symbols
	fset := pkg.Fset
	for _, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		base := ""
		if fset != nil {
			base = filepath.Base(fset.File(f.Pos()).Name())
		}
		if strings.HasSuffix(base, "_test.go") ||
			strings.HasSuffix(base, "_httpbinder_gen.go") ||
			strings.HasSuffix(base, "_openapi_gen.go") ||
			base == "httpbinder_gen.go" ||
			base == "httpbinder_openapi_gen.go" {
			continue
		}
		binderNames := configuredTypeNames(f, normalized.fileTypes, pkg.Imports)
		for name, usage := range discoverGenericTypeArgs(f, pkg.TypesInfo, symbols) {
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
				tp, ok, err := analyzeStruct(ts.Name.Name, st, binderNames)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", ts.Name.Name, err)
				}
				if ok {
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

// discoverGenericTypeArgs finds type arguments of httpbinder Bind/Write/DecodeJSON/EncodeJSON
// using go/types-resolved function identity (import-alias safe).
func discoverGenericTypeArgs(f *ast.File, info *types.Info, symbols []DiscoverySymbol) map[string]Usage {
	out := map[string]Usage{}
	if f == nil || info == nil {
		return out
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		obj := objectOfCall(info, call.Fun)
		usage := usageForSymbol(obj, symbols)
		if usage == 0 {
			return true
		}
		for _, a := range genericTypeArgExprs(call.Fun) {
			if id, ok := a.(*ast.Ident); ok && id.Name != "" {
				out[id.Name] |= usage
			}
		}
		if len(genericTypeArgExprs(call.Fun)) == 0 {
			if name := instantiatedTypeName(info, call.Fun); name != "" {
				out[name] |= usage
			}
		}
		return true
	})
	return out
}

func instantiatedTypeName(info *types.Info, fun ast.Expr) string {
	for {
		switch e := fun.(type) {
		case *ast.ParenExpr:
			fun = e.X
		case *ast.IndexExpr:
			fun = e.X
		case *ast.IndexListExpr:
			fun = e.X
		case *ast.SelectorExpr:
			if inst, ok := info.Instances[e.Sel]; ok && inst.TypeArgs.Len() > 0 {
				return namedTypeName(inst.TypeArgs.At(0))
			}
			return ""
		case *ast.Ident:
			if inst, ok := info.Instances[e]; ok && inst.TypeArgs.Len() > 0 {
				return namedTypeName(inst.TypeArgs.At(0))
			}
			return ""
		default:
			return ""
		}
	}
}

func namedTypeName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok && n.Obj() != nil {
		return n.Obj().Name()
	}
	return ""
}

func usageForSymbol(obj types.Object, symbols []DiscoverySymbol) Usage {
	f, ok := obj.(*types.Func)
	if !ok || f.Pkg() == nil {
		return 0
	}
	var usage Usage
	for _, s := range symbols {
		if f.Pkg().Path() == s.PackagePath && f.Name() == s.Name {
			usage |= s.Usage
		}
	}
	return usage
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

func isHTTPBinderGeneric(obj types.Object) bool {
	f, ok := obj.(*types.Func)
	if !ok || f.Pkg() == nil {
		return false
	}
	if f.Pkg().Path() != httpbinderImportPath {
		return false
	}
	switch f.Name() {
	case "Bind", "Write", "WriteStatus", "DecodeJSON", "EncodeJSON", "NewStream":
		return true
	default:
		return false
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

// httpbinderImportNames returns local identifiers that refer to this library
// (default name "httpbinder" or explicit/aliased imports).
func httpbinderImportNames(f *ast.File) map[string]bool {
	out := make(map[string]bool)
	if f == nil {
		return out
	}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != httpbinderImportPath {
			continue
		}
		if imp.Name != nil {
			switch imp.Name.Name {
			case "_":
				// side-effect import only
			case ".":
				// dot-import
			default:
				out[imp.Name.Name] = true
			}
			continue
		}
		out["httpbinder"] = true
	}
	return out
}

func analyzeStruct(name string, st *ast.StructType, binderNames map[string]bool) (TypePlan, bool, error) {
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
			fp, ok, err := analyzeField(id.Name, f.Type, f.Tag, src, wire, binderNames)
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
					return TypePlan{}, false, fmt.Errorf("field %s: httpbinder.File only supports payload/input tags, got %s", id.Name, src)
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
	return TypePlan{Name: name, Fields: fields}, true, nil
}

func analyzeField(fieldName string, typ ast.Expr, tag *ast.BasicLit, src FieldSource, wire string, binderNames map[string]bool) (FieldPlan, bool, error) {
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
	if jt := tagValue(tag, "json"); jt != "" && jt != "-" {
		jsonName = strings.Split(jt, ",")[0]
	}
	checkRaw := tagValue(tag, "check")
	check, err := ParseCheckTag(checkRaw, kind)
	if err != nil {
		return FieldPlan{}, false, fmt.Errorf("field %s: %w", fieldName, err)
	}
	return FieldPlan{
		Name:     fieldName,
		Wire:     wire,
		Source:   src,
		Kind:     kind,
		JSON:     jsonName,
		Check:    check,
		TypeName: typeName,
		ElemKind: elemKind,
		DB:       dbColumn(fieldName, tag),
		GroupKey: tagPresent(tag, "groupkey"),
	}, true, nil
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
