package generator

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	cbcg "github.com/shibukawa/tinybind-go/configbind/codegen"
)

const (
	configbindImportPath = "github.com/shibukawa/tinybind-go/configbind"
	defaultConfigBindOut = "configbind_gen.go"
)

// ConfigBindBinding is one discovered configbind.Bind[T](prefix) call.
type ConfigBindBinding struct {
	TypeName   string
	Prefix     string
	SubCommand bool
	Name       string
	Help       string
	// SourcePath is the Go file containing the discovered call.
	SourcePath string
}

// ConfigBindSpec pairs one generated config definition with the source file
// whose call declared it.
type ConfigBindSpec struct {
	SourcePath string
	Spec       cbcg.Spec
}

// AnalyzeConfigBind discovers default Bind[T](prefix) registrations.
func AnalyzeConfigBind(dir string) (pkgName string, specs []cbcg.Spec, err error) {
	return AnalyzeConfigBindWithOptions(dir, DefaultOptions())
}

// AnalyzeConfigBindWithOptions discovers configured config-bind calls.
func AnalyzeConfigBindWithOptions(dir string, options Options) (pkgName string, specs []cbcg.Spec, err error) {
	pkgName, sourced, err := AnalyzeConfigBindSources(dir, options)
	if err != nil {
		return "", nil, err
	}
	specs = make([]cbcg.Spec, 0, len(sourced))
	for _, spec := range sourced {
		specs = append(specs, spec.Spec)
	}
	return pkgName, specs, nil
}

// AnalyzeConfigBindSources is AnalyzeConfigBindWithOptions with the owning
// source file retained for every discovered definition.
func AnalyzeConfigBindSources(dir string, options Options) (pkgName string, specs []ConfigBindSpec, err error) {
	return configBindSources(newPackageLoad(dir), options)
}

// configBindSources discovers config-bind definitions in a package the run
// already loaded.
func configBindSources(load *packageLoad, options Options) (pkgName string, specs []ConfigBindSpec, err error) {
	pkg, err := load.get()
	if err != nil {
		return "", nil, err
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

	patterns, err := options.callPatterns()
	if err != nil {
		return "", nil, err
	}
	var configPatterns []CallPattern
	for _, pattern := range patterns {
		if pattern.Operation == OperationConfigBind || pattern.Operation == OperationConfigSubCommand {
			configPatterns = append(configPatterns, pattern)
		}
	}

	var bindings []ConfigBindBinding
	fset := pkg.Fset
	for _, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		base, sourcePath := "", ""
		if fset != nil {
			sourcePath = fset.File(f.Pos()).Name()
			base = filepath.Base(sourcePath)
		}
		if strings.HasSuffix(base, "_test.go") ||
			base == "configbind_gen.go" ||
			base == "tinybind_gen.go" ||
			base == "tinybind_openapi_gen.go" {
			continue
		}
		discovered, err := discoverConfigBindCalls(f, pkg.TypesInfo, configPatterns)
		if err != nil {
			return "", nil, err
		}
		for i := range discovered {
			discovered[i].SourcePath = sourcePath
		}
		bindings = append(bindings, discovered...)
	}

	// Deduplicate Bind by TypeName+Prefix and subcommands by TypeName+Name.
	seen := map[string]bool{}
	for _, b := range bindings {
		key := b.TypeName + "\x00" + b.Prefix
		if b.SubCommand {
			key = "subcommand\x00" + b.TypeName + "\x00" + b.Name
		}
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
		specs = append(specs, ConfigBindSpec{
			SourcePath: b.SourcePath,
			Spec: cbcg.Spec{
				PackagePath: pkg.PkgPath,
				TypeName:    b.TypeName,
				Prefix:      b.Prefix,
				SubCommand:  b.SubCommand,
				Name:        b.Name,
				Help:        b.Help,
				Fields:      fields,
			},
		})
	}
	return pkg.Name, specs, nil
}

func discoverConfigBindCalls(f *ast.File, info *types.Info, patterns []CallPattern) ([]ConfigBindBinding, error) {
	var out []ConfigBindBinding
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
		if obj == nil || obj.Pkg() == nil {
			return true
		}
		pattern, ok := matchingCallPattern(obj, patterns)
		if !ok {
			return true
		}
		signature, _ := obj.Type().(*types.Signature)
		typeSource := pattern.TypeRoles["config"]
		if typeSource.GenericArgument != nil && (signature == nil || signature.TypeParams().Len() <= *typeSource.GenericArgument) {
			discoveryErr = fmt.Errorf("generator: %s pattern %s generic_argument index %d exceeds wrapper signature", pattern.Operation, callTargetKey(pattern.Target), *typeSource.GenericArgument)
			return false
		}
		if typeSource.ArgumentType != nil && (signature == nil || signature.Params().Len() <= *typeSource.ArgumentType) {
			discoveryErr = fmt.Errorf("generator: %s pattern %s argument_type index %d exceeds wrapper signature", pattern.Operation, callTargetKey(pattern.Target), *typeSource.ArgumentType)
			return false
		}
		typeName := callTypeRoleName(info, call, typeSource)
		if typeName == "" {
			discoveryErr = fmt.Errorf("generator: %s pattern %s could not resolve a same-package config type", pattern.Operation, callTargetKey(pattern.Target))
			return false
		}
		if pattern.Operation == OperationConfigSubCommand {
			name, ok := checkedStringRole(info, call, signature, pattern, "name")
			if !ok {
				discoveryErr = fmt.Errorf("generator: config_subcommand pattern %s requires a compile-time string name", callTargetKey(pattern.Target))
				return false
			}
			help, ok := checkedStringRole(info, call, signature, pattern, "help")
			if !ok {
				discoveryErr = fmt.Errorf("generator: config_subcommand pattern %s requires compile-time string help", callTargetKey(pattern.Target))
				return false
			}
			out = append(out, ConfigBindBinding{TypeName: typeName, SubCommand: true, Name: name, Help: help})
			return true
		}
		prefix, ok := checkedStringRole(info, call, signature, pattern, "prefix")
		if !ok {
			discoveryErr = fmt.Errorf("generator: config_bind pattern %s requires a compile-time string prefix", callTargetKey(pattern.Target))
			return false
		}
		out = append(out, ConfigBindBinding{TypeName: typeName, Prefix: prefix})
		return true
	})
	return out, discoveryErr
}

func checkedStringRole(info *types.Info, call *ast.CallExpr, signature *types.Signature, pattern CallPattern, role string) (string, bool) {
	source := pattern.ArgumentRoles[role]
	if source.Argument != nil && (signature == nil || signature.Params().Len() <= *source.Argument) {
		return "", false
	}
	return callStringRole(info, call, source)
}

func matchingCallPattern(obj types.Object, patterns []CallPattern) (CallPattern, bool) {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil {
		return CallPattern{}, false
	}
	for _, pattern := range patterns {
		if pattern.Target.Function != nil {
			target := pattern.Target.Function
			if fn.Pkg().Path() == target.PackagePath && fn.Name() == target.Name {
				if signature, ok := fn.Type().(*types.Signature); ok && signature.Recv() == nil {
					return pattern, true
				}
			}
			continue
		}
		if pattern.Target.Method == nil || fn.Pkg().Path() != pattern.Target.Method.PackagePath || fn.Name() != pattern.Target.Method.Name {
			continue
		}
		signature, ok := fn.Type().(*types.Signature)
		if !ok || signature.Recv() == nil {
			continue
		}
		receiver := signature.Recv().Type()
		if pointer, ok := receiver.(*types.Pointer); ok {
			receiver = pointer.Elem()
		}
		named, ok := receiver.(*types.Named)
		if ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == pattern.Target.Method.ReceiverPackagePath && named.Obj().Name() == pattern.Target.Method.ReceiverType {
			return pattern, true
		}
	}
	return CallPattern{}, false
}

func callTypeRoleName(info *types.Info, call *ast.CallExpr, source TypeSource) string {
	if source.GenericArgument != nil {
		args := genericTypeArgExprs(call.Fun)
		if len(args) > *source.GenericArgument {
			return localNamedTypeName(info.TypeOf(args[*source.GenericArgument]))
		}
		return instantiatedTypeNameAt(info, call.Fun, *source.GenericArgument)
	}
	if source.ArgumentType != nil && len(call.Args) > *source.ArgumentType {
		return localNamedTypeName(info.TypeOf(call.Args[*source.ArgumentType]))
	}
	return ""
}

func localNamedTypeName(value types.Type) string {
	if pointer, ok := value.(*types.Pointer); ok {
		value = pointer.Elem()
	}
	named, ok := value.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return ""
	}
	return named.Obj().Name()
}

func callStringRole(info *types.Info, call *ast.CallExpr, source ValueSource) (string, bool) {
	if source.IsConstant {
		value, ok := source.Constant.(string)
		return value, ok
	}
	if source.Argument == nil || len(call.Args) <= *source.Argument {
		return "", false
	}
	typed := info.Types[call.Args[*source.Argument]]
	if typed.Value == nil || typed.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(typed.Value), true
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
		env := structTagGet(tag, "env")
		help := structTagGet(tag, "help")
		arg := structTagGet(tag, "arg")

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
					Env:     env,
					Help:    help,
					Arg:     arg,
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
			Env:     env,
			Help:    help,
			Arg:     arg,
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
	return g.generateConfigBind(newPackageLoad(dir), outDir, outName)
}

// generateConfigBind is GenerateConfigBind over a package the run already loaded.
func (g *Generator) generateConfigBind(load *packageLoad, outDir, outName string) (string, error) {
	dir := load.dir
	pkgName, sourced, err := configBindSources(load, g.Options)
	if err != nil {
		return "", err
	}
	specs := make([]cbcg.Spec, 0, len(sourced))
	for _, spec := range sourced {
		specs = append(specs, spec.Spec)
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
