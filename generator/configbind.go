package generator

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	cbcg "github.com/shibukawa/tinybind-go/configbind/codegen"
	"golang.org/x/tools/go/packages"
)

const (
	configbindImportPath = "github.com/shibukawa/tinybind-go/configbind"
	defaultConfigBindOut = "configbind_gen.go"
)

// ConfigBindBinding is one discovered configbind.Bind[T](prefix) call.
type ConfigBindBinding struct {
	TypeName string
	Prefix   string
}

// AnalyzeConfigBind discovers Bind[T](prefix) registrations and builds codegen specs.
func AnalyzeConfigBind(dir string) (pkgName string, specs []cbcg.Spec, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, err
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
		return "", nil, fmt.Errorf("packages.Load %s: %w", abs, err)
	}
	if len(pkgs) == 0 {
		return "", nil, fmt.Errorf("no package in %s", abs)
	}
	pkg := pkgs[0]
	for _, p := range pkgs {
		if p.Name != "" && !strings.HasSuffix(p.ID, ".test") {
			pkg = p
			break
		}
	}
	if pkg.TypesInfo == nil {
		return "", nil, fmt.Errorf("type-check failed for %s: %v", abs, pkg.Errors)
	}

	// Map type name -> *types.Struct
	structs := map[string]*types.Struct{}
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		if st, ok := tn.Type().Underlying().(*types.Struct); ok {
			structs[name] = st
		}
	}

	var bindings []ConfigBindBinding
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
			base == "configbind_gen.go" ||
			base == "tinybind_gen.go" ||
			base == "tinybind_openapi_gen.go" {
			continue
		}
		bindings = append(bindings, discoverConfigBindCalls(f, pkg.TypesInfo)...)
	}

	// Deduplicate by TypeName+Prefix
	seen := map[string]bool{}
	for _, b := range bindings {
		key := b.TypeName + "\x00" + b.Prefix
		if seen[key] {
			continue
		}
		seen[key] = true
		st, ok := structs[b.TypeName]
		if !ok {
			return "", nil, fmt.Errorf("configbind: type %s not found in package", b.TypeName)
		}
		fields, err := configFieldsFromStruct(st, "")
		if err != nil {
			return "", nil, fmt.Errorf("configbind: %s: %w", b.TypeName, err)
		}
		specs = append(specs, cbcg.Spec{
			TypeName: b.TypeName,
			Prefix:   b.Prefix,
			Fields:   fields,
		})
	}
	return pkg.Name, specs, nil
}

func discoverConfigBindCalls(f *ast.File, info *types.Info) []ConfigBindBinding {
	var out []ConfigBindBinding
	if f == nil || info == nil {
		return out
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 1 {
			return true
		}
		obj := objectOfCall(info, call.Fun)
		if obj == nil || obj.Pkg() == nil {
			return true
		}
		if obj.Pkg().Path() != configbindImportPath || obj.Name() != "Bind" {
			return true
		}
		typeName := ""
		for _, a := range genericTypeArgExprs(call.Fun) {
			if id, ok := a.(*ast.Ident); ok {
				typeName = id.Name
				break
			}
		}
		if typeName == "" {
			if name := instantiatedTypeName(info, call.Fun); name != "" {
				typeName = name
			}
		}
		if typeName == "" {
			return true
		}
		prefix, ok := stringLit(call.Args[0])
		if !ok {
			return true
		}
		out = append(out, ConfigBindBinding{TypeName: typeName, Prefix: prefix})
		return true
	})
	return out
}

func stringLit(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func configFieldsFromStruct(st *types.Struct, keyPrefix string) ([]cbcg.Field, error) {
	var fields []cbcg.Field
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Exported() {
			continue
		}
		tag := st.Tag(i)
		key := fieldKeyFromName(f.Name())
		if k := structTagGet(tag, "key"); k != "" {
			key = k
		}
		// convention: toml/json snake from name if no key tag — already snake from fieldKeyFromName
		def := structTagGet(tag, "default")
		opt := structTagGet(tag, "opt")
		help := structTagGet(tag, "help")

		ft := f.Type()
		if named, ok := ft.(*types.Named); ok {
			if underlying, ok := named.Underlying().(*types.Struct); ok {
				nested, err := configFieldsFromStruct(underlying, joinConfigKey(keyPrefix, key))
				if err != nil {
					return nil, err
				}
				fields = append(fields, cbcg.Field{
					GoName:  f.Name(),
					Key:     key,
					Kind:    cbcg.FieldStruct,
					Nested:  nested,
					Default: def,
					Opt:     opt,
					Help:    help,
				})
				continue
			}
		}
		kind, err := configFieldKind(ft)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name(), err)
		}
		fields = append(fields, cbcg.Field{
			GoName:  f.Name(),
			Key:     key,
			Kind:    kind,
			Default: def,
			Opt:     opt,
			Help:    help,
		})
	}
	return fields, nil
}

func configFieldKind(t types.Type) (cbcg.FieldKind, error) {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch u.Kind() {
		case types.String:
			return cbcg.FieldString, nil
		case types.Bool:
			return cbcg.FieldBool, nil
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
			return cbcg.FieldInt, nil
		default:
			return 0, fmt.Errorf("unsupported basic type %s", u)
		}
	case *types.Slice:
		if b, ok := u.Elem().Underlying().(*types.Basic); ok && b.Kind() == types.String {
			return cbcg.FieldStringSlice, nil
		}
		return 0, fmt.Errorf("only []string slices supported in configbind v1")
	default:
		return 0, fmt.Errorf("unsupported field type %s", t)
	}
}

func fieldKeyFromName(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				// Insert underscore at lower→Upper or acronym boundary (XMLParser → xml_parser).
				nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func structTagGet(tag, key string) string {
	// minimal parser for `key:"value"`
	tag = strings.TrimSpace(tag)
	for tag != "" {
		i := strings.IndexByte(tag, ':')
		if i < 0 {
			break
		}
		name := strings.TrimSpace(tag[:i])
		tag = tag[i+1:]
		if !strings.HasPrefix(tag, `"`) {
			break
		}
		// scan quoted
		j := 1
		for j < len(tag) {
			if tag[j] == '\\' {
				j += 2
				continue
			}
			if tag[j] == '"' {
				val := tag[1:j]
				if name == key {
					// unquote simple escapes
					s, err := strconv.Unquote(`"` + val + `"`)
					if err != nil {
						return val
					}
					return s
				}
				tag = strings.TrimSpace(tag[j+1:])
				break
			}
			j++
		}
	}
	return ""
}

func joinConfigKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if key == "" {
		return prefix
	}
	return prefix + "." + key
}

// GenerateConfigBind analyzes dir for configbind.Bind usage and writes configbind_gen.go.
// Returns the absolute path written, or "" if no Bind calls found.
func (g *Generator) GenerateConfigBind(dir, outDir, outName string) (string, error) {
	pkgName, specs, err := AnalyzeConfigBind(dir)
	if err != nil {
		return "", err
	}
	if len(specs) == 0 {
		return "", nil
	}
	src, err := cbcg.Generate(pkgName, specs)
	if err != nil {
		return "", err
	}
	if outDir == "" {
		outDir = dir
	}
	if outName == "" {
		outName = defaultConfigBindOut
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, outName)
	if err := os.WriteFile(path, src, 0o644); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

// silence unused in case of build tags
var _ = utf8.RuneError
