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

	docs := buildConfigDocIndex(pkg)

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
			// An unparsable file has no FileSet handle; see the same guard in
			// analyzeLoadedPackage.
			if handle := fset.File(f.Pos()); handle != nil {
				sourcePath = handle.Name()
				base = filepath.Base(sourcePath)
			}
		}
		if skipConfigSourceFile(base) {
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
	collector := &configFieldCollector{docs: docs, planned: map[*ast.Field]bool{}}
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
		fields, err := collector.fields(st, "")
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
				Doc:         docs.typeDoc[b.TypeName],
				Fields:      fields,
			},
		})
	}
	// Backfill runs before the caller builds IR; specs already carry the same
	// help text, so a reload is unnecessary and a second run is a no-op.
	if !options.featureDisabled(FeatureHelpBackfill) {
		if _, err := applyHelpBackfills(pkg.Fset, collector.edits); err != nil {
			return "", nil, err
		}
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
		roleType := callTypeRoleType(info, call, typeSource)
		if isTypeParam(roleType) {
			// A generic wrapper handing its own type parameter to the wrapped
			// operation names no concrete config type here; the type is only
			// known where the wrapper is instantiated, and that call site is
			// matched by its own registered pattern. Skipping keeps one such
			// wrapper from failing the whole package, which would also lose the
			// config types declared beside it.
			return true
		}
		typeName := localNamedTypeName(roleType)
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
		named, ok := unaliasPtr(signature.Recv().Type()).(*types.Named)
		if ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == pattern.Target.Method.ReceiverPackagePath && named.Obj().Name() == pattern.Target.Method.ReceiverType {
			return pattern, true
		}
	}
	return CallPattern{}, false
}

// callTypeRoleType resolves the type a pattern's type role points at, whether
// it is spelled as an explicit type argument, inferred at an instantiation, or
// taken from a value argument's type.
func callTypeRoleType(info *types.Info, call *ast.CallExpr, source TypeSource) types.Type {
	if source.GenericArgument != nil {
		args := genericTypeArgExprs(call.Fun)
		if len(args) > *source.GenericArgument {
			return info.TypeOf(args[*source.GenericArgument])
		}
		return instantiatedTypeArgAt(info, call.Fun, *source.GenericArgument)
	}
	if source.ArgumentType != nil && len(call.Args) > *source.ArgumentType {
		return info.TypeOf(call.Args[*source.ArgumentType])
	}
	return nil
}

// isTypeParam reports whether a type role resolved to a type parameter rather
// than a concrete type. That is not a resolution failure: it means this call
// site is not a generation target.
func isTypeParam(value types.Type) bool {
	if value == nil {
		return false
	}
	_, ok := unaliasPtr(value).(*types.TypeParam)
	return ok
}

func localNamedTypeName(value types.Type) string {
	named, ok := unaliasPtr(value).(*types.Named)
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

// configFieldCollector walks config structs and resolves each field's help text,
// falling back to its godoc comment and planning a source backfill when the help
// tag is absent.
type configFieldCollector struct {
	docs    *docIndex
	planned map[*ast.Field]bool
	edits   []helpBackfill
	// open are the element struct types currently being walked, so a struct that
	// holds a slice of itself is reported instead of recursing forever.
	open []*types.Named
}

// help returns the description for one field. An existing help tag always wins,
// even when its value is empty; only a missing key falls back to godoc.
func (c *configFieldCollector) help(field *types.Var, tag string) string {
	if value, ok := structTagLookup(tag, "help"); ok {
		return value
	}
	doc := c.docs.fieldDoc[field]
	if doc == "" {
		return ""
	}
	if node := c.docs.fieldAST[field]; node != nil && !c.planned[node] {
		c.planned[node] = true
		c.edits = append(c.edits, helpBackfill{path: c.docs.fieldPath[field], field: node, help: doc})
	}
	return doc
}

func (c *configFieldCollector) fields(st *types.Struct, keyPrefix string) ([]cbcg.Field, error) {
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
		help := c.help(f, tag)
		arg := structTagGet(tag, "arg")
		dependsOn := structTagGet(tag, "dependon")
		falsy := structTagGet(tag, "falsy")
		secret := structTagGet(tag, "secret")
		summary := structTagGet(tag, "summary")
		// enum is carried for the sake of a dependon condition that names values of
		// this field, which is the one thing configbind does with an allowlist; the
		// load-time check of rule:enum-value-validation is still unimplemented.
		enum := structTagGet(tag, "enum")

		// An alias is transparent here: it must bind as the type it names, not
		// fall through to that type's underlying form.
		ft := types.Unalias(f.Type())
		if named, ok := ft.(*types.Named); ok {
			if underlying, ok := named.Underlying().(*types.Struct); ok {
				nested, err := c.fields(underlying, joinConfigKey(keyPrefix, key))
				if err != nil {
					return nil, err
				}
				fields = append(fields, cbcg.Field{
					GoName:    f.Name(),
					Key:       key,
					Kind:      cbcg.FieldStruct,
					Nested:    nested,
					Default:   def,
					Opt:       opt,
					Env:       env,
					Help:      help,
					Arg:       arg,
					DependsOn: dependsOn,
					Falsy:     falsy,
					Secret:    secret,
					Summary:   summary,
					Enum:      enum,
				})
				continue
			}
		}
		if slice, ok := ft.Underlying().(*types.Slice); ok {
			elem, elemStruct, err := configElementStruct(f, slice)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f.Name(), err)
			}
			if elemStruct != nil {
				nested, err := c.elementFields(elem, elemStruct, joinConfigKey(keyPrefix, key))
				if err != nil {
					return nil, fmt.Errorf("%s: %w", f.Name(), err)
				}
				fields = append(fields, cbcg.Field{
					GoName:    f.Name(),
					Key:       key,
					Kind:      cbcg.FieldStructSlice,
					ElemType:  elem.Obj().Name(),
					Nested:    nested,
					Default:   def,
					Opt:       opt,
					Env:       env,
					Help:      help,
					DependsOn: dependsOn,
					Falsy:     falsy,
					Secret:    secret,
					Summary:   summary,
					Enum:      enum,
					Arg:       arg,
				})
				continue
			}
		}
		kind, goType, err := configFieldKind(ft)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name(), err)
		}
		fields = append(fields, cbcg.Field{
			GoName:    f.Name(),
			Key:       key,
			Kind:      kind,
			GoType:    goType,
			Default:   def,
			Opt:       opt,
			Env:       env,
			Help:      help,
			Arg:       arg,
			DependsOn: dependsOn,
			Falsy:     falsy,
			Secret:    secret,
			Summary:   summary,
			Enum:      enum,
		})
	}
	return fields, nil
}

// configElementStruct reports whether a slice field is an array of tables: a
// slice of a named struct declared in the same package as the config struct,
// which the generated code names directly. Other slices return a nil struct so
// the caller falls through to the scalar-slice kinds.
func configElementStruct(field *types.Var, slice *types.Slice) (*types.Named, *types.Struct, error) {
	elem := types.Unalias(slice.Elem())
	if pointer, ok := elem.(*types.Pointer); ok {
		if named, ok := types.Unalias(pointer.Elem()).(*types.Named); ok {
			if _, ok := named.Underlying().(*types.Struct); ok {
				return nil, nil, fmt.Errorf("use []%s instead of []*%s for an array of tables",
					named.Obj().Name(), named.Obj().Name())
			}
		}
		return nil, nil, nil
	}
	named, ok := elem.(*types.Named)
	if !ok {
		return nil, nil, nil
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil, nil, nil
	}
	if named.Obj().Pkg() == nil || field.Pkg() == nil || named.Obj().Pkg().Path() != field.Pkg().Path() {
		return nil, nil, fmt.Errorf("array-of-tables element type %s must be declared in the config struct's package", named.Obj().Name())
	}
	return named, st, nil
}

// elementFields walks an array-of-tables element struct, refusing a struct that
// reaches itself.
func (c *configFieldCollector) elementFields(elem *types.Named, st *types.Struct, keyPrefix string) ([]cbcg.Field, error) {
	for _, open := range c.open {
		if open == elem {
			return nil, fmt.Errorf("recursive config struct %s is not supported", elem.Obj().Name())
		}
	}
	c.open = append(c.open, elem)
	defer func() { c.open = c.open[:len(c.open)-1] }()
	return c.fields(st, keyPrefix)
}

// configFieldKind classifies a field type. For an integer it also returns the
// Go integer type the generated code must assign, so a sized field keeps its
// width instead of being narrowed to int.
func configFieldKind(t types.Type) (cbcg.FieldKind, string, error) {
	// time.Duration must be matched by name: its underlying type is int64, so
	// the basic switch below would bind it as an int and reject "5s".
	if isTimeDuration(t) {
		return cbcg.FieldDuration, "", nil
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch u.Kind() {
		case types.String:
			return cbcg.FieldString, "", nil
		case types.Bool:
			return cbcg.FieldBool, "", nil
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
			goType, err := integerGoTypeName(t)
			if err != nil {
				return 0, "", err
			}
			return cbcg.FieldInt, goType, nil
		default:
			return 0, "", fmt.Errorf("unsupported basic type %s", u)
		}
	case *types.Slice:
		if b, ok := u.Elem().Underlying().(*types.Basic); ok && b.Kind() == types.String {
			return cbcg.FieldStringSlice, "", nil
		}
		return 0, "", fmt.Errorf("only []string slices supported in configbind v1")
	default:
		return 0, "", fmt.Errorf("unsupported field type %s", t)
	}
}

// integerGoTypeName is the Go type name the generated assignment converts to.
// Only a predeclared integer type, or an alias of one, is supported: a defined
// type would need its own name for the conversion and its underlying width for
// the parse, and carrying both is out of scope for v1. Until then it fails
// here rather than emitting an assignment that does not compile.
func integerGoTypeName(t types.Type) (string, error) {
	basic, ok := types.Unalias(t).(*types.Basic)
	if !ok {
		return "", fmt.Errorf("defined integer type %s is unsupported; use a predeclared integer type", t)
	}
	return basic.Name(), nil
}

// isTimeDuration reports whether t is exactly time.Duration, including through
// an alias. A locally defined named type whose underlying type is
// time.Duration does not qualify.
func isTimeDuration(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "time" && named.Obj().Name() == "Duration"
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
	value, _ := structTagLookup(tag, key)
	return value
}

// structTagLookup is a minimal parser for `key:"value"`. The bool distinguishes
// an absent key from one explicitly set to the empty string.
func structTagLookup(tag, key string) (string, bool) {
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
		closed := false
		for j < len(tag) {
			if tag[j] == '\\' {
				j += 2
				continue
			}
			if tag[j] == '"' {
				closed = true
				break
			}
			j++
		}
		if !closed {
			break
		}
		val := tag[1:j]
		if name == key {
			// unquote simple escapes
			s, err := strconv.Unquote(`"` + val + `"`)
			if err != nil {
				return val, true
			}
			return s, true
		}
		tag = strings.TrimSpace(tag[j+1:])
	}
	return "", false
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
