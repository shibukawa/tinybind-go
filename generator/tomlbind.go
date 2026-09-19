package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/types"
	"os"
	"path/filepath"
	"sort"

	cbcg "github.com/shibukawa/tinybind-go/minitoml/codegen"
)

// tomlbindImportPath is the package whose entry points ask for a TOML decoder.
const tomlbindImportPath = "github.com/shibukawa/tinybind-go/tomlbind"

const defaultTOMLBindOut = "tomlbind_gen.go"

// TOML decoder emission for requirement:tomlbind-standalone-decoder.
//
// A call to a tomlbind entry point, or a GenerateDecoder declaration, is the
// ask. The struct is walked by the configbind field collector, so the key and
// default tags read here are the ones configbind reads and one struct serves
// both; the body is the minitoml/codegen apply function, emitted under this
// pass's own name and error prefix with the document root as its prefix.
//
// Nothing is registered and no init is emitted: the entry point calls the
// generated DecodeTOMLFrom through a pointer constraint, per
// decision:tomlbind-dispatch-shape, so a type with no decoder fails to build.

// canonicalTOMLBindCalls declares the tomlbind entry points. All three name
// the type as the first type argument, since T appears only in the result.
func canonicalTOMLBindCalls() []CallPattern {
	path := tomlbindImportPath
	return []CallPattern{
		TOMLDecodeCall(Function(path, "DecodeTOML"), GenericType("decode", 0)),
		TOMLDecodeCall(Function(path, "DecodeTOMLBytes"), GenericType("decode", 0)),
		TOMLDecodeCall(Function(path, "GenerateDecoder"), GenericType("decode", 0)),
	}
}

// TOMLBindSpec is one type a tomlbind entry point named, with the source file
// whose call named it.
type TOMLBindSpec struct {
	SourcePath string
	Spec       cbcg.Spec
}

// tomlBindSources discovers the types tomlbind decoders are generated for in
// a package the run already loaded.
func tomlBindSources(load *packageLoad, options Options) (pkgName string, specs []TOMLBindSpec, err error) {
	if options.featureDisabled(FeatureDecodeTOML) {
		return "", nil, nil
	}
	pkg, err := load.get()
	if err != nil {
		return "", nil, err
	}
	patterns, err := options.callPatterns()
	if err != nil {
		return "", nil, err
	}
	var tomlPatterns []CallPattern
	for _, pattern := range patterns {
		if pattern.Operation == OperationTOMLDecode {
			tomlPatterns = append(tomlPatterns, pattern)
		}
	}
	if len(tomlPatterns) == 0 {
		return "", nil, nil
	}

	type discovered struct {
		typeName, sourcePath string
	}
	var found []discovered
	fset := pkg.Fset
	for _, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		base, sourcePath := "", ""
		if fset != nil {
			if handle := fset.File(f.Pos()); handle != nil {
				sourcePath = handle.Name()
				base = filepath.Base(sourcePath)
			}
		}
		if skipConfigSourceFile(base) {
			continue
		}
		names, err := discoverTOMLBindCalls(f, pkg.TypesInfo, pkg.PkgPath, tomlPatterns)
		if err != nil {
			return "", nil, err
		}
		for _, name := range names {
			found = append(found, discovered{typeName: name, sourcePath: sourcePath})
		}
	}
	if len(found) == 0 {
		return pkg.Name, nil, nil
	}
	// One decoder per type, owned by the first file that named it; the order
	// is by name so the output does not move with the call sites.
	sort.SliceStable(found, func(i, j int) bool { return found[i].typeName < found[j].typeName })
	seen := map[string]bool{}
	scope := pkg.Types.Scope()
	collector := &configFieldCollector{docs: buildConfigDocIndex(pkg), planned: map[*ast.Field]bool{}, floats: true}
	for _, d := range found {
		if seen[d.typeName] {
			continue
		}
		seen[d.typeName] = true
		tn, _ := scope.Lookup(d.typeName).(*types.TypeName)
		if tn == nil {
			return "", nil, fmt.Errorf("tomlbind: type %s not found in package", d.typeName)
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			return "", nil, fmt.Errorf("tomlbind: %s is not a struct; a TOML document decodes into a struct", d.typeName)
		}
		fields, err := collector.fields(st, "")
		if err != nil {
			return "", nil, fmt.Errorf("tomlbind: %s: %w", d.typeName, err)
		}
		specs = append(specs, TOMLBindSpec{
			SourcePath: d.sourcePath,
			Spec:       cbcg.Spec{TypeName: d.typeName, Fields: fields},
		})
	}
	return pkg.Name, specs, nil
}

// discoverTOMLBindCalls returns the local type names the tomlbind patterns in
// f name. A call naming a type from another package generates nothing here
// and is left to the compiler, which reports the unsatisfied constraint at the
// call site; a declaration naming one is refused, since a declaration is
// written to be found and a silent nothing is what it exists to remove.
func discoverTOMLBindCalls(f *ast.File, info *types.Info, pkgPath string, patterns []CallPattern) ([]string, error) {
	var out []string
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
		typeSource := pattern.TypeRoles["decode"]
		roleType := callTypeRoleType(info, call, typeSource)
		if roleType == nil || isTypeParam(roleType) {
			return true
		}
		named, ok := unaliasPtr(roleType).(*types.Named)
		if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
			discoveryErr = fmt.Errorf("generator: tomlbind: %s names %s, which is not a named struct type", obj.Name(), roleType)
			return false
		}
		if named.Obj().Pkg().Path() != pkgPath {
			if obj.Name() == "GenerateDecoder" && obj.Pkg().Path() == tomlbindImportPath {
				discoveryErr = fmt.Errorf("generator: tomlbind: GenerateDecoder[%s] names a type declared in %s; write the declaration in that package, beside the type",
					named.Obj().Name(), named.Obj().Pkg().Path())
				return false
			}
			return true
		}
		out = append(out, named.Obj().Name())
		return true
	})
	return out, discoveryErr
}

// generateTOMLBindSource emits the decoders and their methods as one file.
func generateTOMLBindSource(pkgName string, specs []TOMLBindSpec) ([]byte, error) {
	plain := make([]cbcg.Spec, 0, len(specs))
	for _, spec := range specs {
		plain = append(plain, spec.Spec)
	}
	var body bytes.Buffer
	for _, spec := range plain {
		fn := "decode" + spec.TypeName + "TOML"
		if err := cbcg.EmitApply(&body, spec, cbcg.ApplyOptions{FuncName: fn, ErrorPrefix: "tomlbind", AllowEmptyPrefix: true}); err != nil {
			return nil, err
		}
		fmt.Fprintf(&body, "// DecodeTOMLFrom decodes one TOML document into v, for tomlbind.DecodeTOML.\n")
		fmt.Fprintf(&body, "func (v *%s) DecodeTOMLFrom(data []byte) error {\n", spec.TypeName)
		body.WriteString("\tdoc, err := minitoml.Parse(data)\n")
		body.WriteString("\tif err != nil {\n\t\treturn err\n\t}\n")
		// The document replaces the value, as jsonbind's DecodeJSONFrom does.
		fmt.Fprintf(&body, "\t*v = %s{}\n", spec.TypeName)
		fmt.Fprintf(&body, "\treturn %s(v, doc)\n}\n\n", fn)
	}
	var out bytes.Buffer
	out.WriteString("// Code generated by tinybind; DO NOT EDIT.\n\n")
	fmt.Fprintf(&out, "package %s\n\n", pkgName)
	out.WriteString("import (\n\t\"fmt\"\n")
	if cbcg.SpecsNeedTime(plain) {
		out.WriteString("\t\"time\"\n")
	}
	out.WriteString("\n\t\"github.com/shibukawa/tinybind-go/minitoml\"\n)\n\n")
	out.Write(body.Bytes())
	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("tomlbind: format: %w\n%s", err, out.String())
	}
	return formatted, nil
}

// generateTOMLBind writes the package's TOML decoders, or nothing when no
// entry point is called.
func (g *Generator) generateTOMLBind(load *packageLoad, outDir, outName string) (string, error) {
	pkgName, specs, err := tomlBindSources(load, g.Options)
	if err != nil {
		return "", err
	}
	if len(specs) == 0 {
		return "", nil
	}
	src, err := generateTOMLBindSource(pkgName, specs)
	if err != nil {
		return "", err
	}
	if outDir == "" {
		outDir = load.dir
	}
	if outName == "" {
		outName = defaultTOMLBindOut
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

// tomlBindArtifacts emits one decoder artifact per source file whose call
// named a type, the way the configbind artifacts are split.
func (g *Generator) tomlBindArtifacts(load *packageLoad) ([]Artifact, error) {
	pkgName, specs, err := tomlBindSources(load, g.Options)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, nil
	}
	sort.SliceStable(specs, func(i, j int) bool { return specs[i].SourcePath < specs[j].SourcePath })
	var artifacts []Artifact
	for start := 0; start < len(specs); {
		end := start + 1
		for end < len(specs) && specs[end].SourcePath == specs[start].SourcePath {
			end++
		}
		code, err := generateTOMLBindSource(pkgName, specs[start:end])
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, Artifact{
			SourcePath:  specs[start].SourcePath,
			Kind:        ArtifactTOMLBind,
			Destination: DestinationGoPackage,
			OutputBase:  artifactBase(specs[start].SourcePath),
			Extension:   ExtensionGo,
			PackageName: pkgName,
			Content:     code,
		})
		start = end
	}
	return artifacts, nil
}
