package generator

import (
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"

	cbcg "github.com/shibukawa/tinybind-go/configbind/codegen"
)

// ArtifactKind classifies one generated Go source unit.
type ArtifactKind string

const (
	// ArtifactPackageShared holds declarations reused by several per-source
	// artifacts of one package.
	ArtifactPackageShared ArtifactKind = "package_shared"
	ArtifactHTMLTemplate  ArtifactKind = "html_template"
	ArtifactSQLTemplate   ArtifactKind = "sql_template"
	ArtifactBinding       ArtifactKind = "binding"
	ArtifactConfigBind    ArtifactKind = "configbind"
	ArtifactOpenAPI       ArtifactKind = "openapi"
)

// Artifact is one formatted Go source unit and the source file that owns it.
// The caller maps OutputBase to its own generated file name; nothing is written
// to disk by the API that produces Artifacts.
type Artifact struct {
	// SourcePath is the real on-disk path of the owning source. It is empty for
	// package-wide artifacts.
	SourcePath string
	Kind       ArtifactKind
	// OutputBase is the suggested output base name, without directory,
	// extension, or generated-file suffix.
	OutputBase  string
	PackageName string
	GoSource    []byte
}

// Base names for the package-wide artifacts.
const (
	sharedArtifactBase  = "tinybind_shared"
	openAPIArtifactBase = "tinybind_openapi"
)

// GenerateArtifacts runs every enabled generation phase and returns the result
// as per-source artifacts. It writes no file, so the same call serves both
// generation and --check.
func (g *Generator) GenerateArtifacts(ctx context.Context, request GenerateRequest) ([]Artifact, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.Dir == "" {
		request.Dir = "."
	}
	runner := New(request.applyTo(g.Options))
	normalized, err := runner.Options.normalized()
	if err != nil {
		return nil, err
	}

	artifacts, err := runner.templateArtifacts(request.Dir)
	if err != nil {
		return nil, fmt.Errorf("generate templates: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	binding, err := runner.bindingArtifacts(request.Dir)
	if err != nil {
		return nil, fmt.Errorf("generate mapping: %w", err)
	}
	artifacts = append(artifacts, binding...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	config, err := runner.configBindArtifacts(request.Dir)
	if err != nil {
		return nil, fmt.Errorf("generate configbind: %w", err)
	}
	artifacts = append(artifacts, config...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.OpenAPI && normalized.openAPI {
		openAPI, err := runner.openAPIArtifact(request.Dir)
		if err != nil {
			return nil, fmt.Errorf("generate OpenAPI: %w", err)
		}
		artifacts = append(artifacts, openAPI...)
	}
	return artifacts, nil
}

// applyTo layers the per-run request switches over base options.
func (request GenerateRequest) applyTo(base Options) Options {
	options := base
	options.GenerateAll = options.GenerateAll || request.GenerateAll
	options.SQLContextAPI = options.SQLContextAPI || request.SQLContextAPI
	options.SQLContextOnlyAPI = options.SQLContextOnlyAPI || request.SQLContextOnlyAPI
	if request.HTMLTemplatePattern != "" {
		options.HTMLTemplatePattern = request.HTMLTemplatePattern
	}
	if request.SQLTemplatePattern != "" {
		options.SQLTemplatePattern = request.SQLTemplatePattern
	}
	return options
}

// artifactBase strips every extension from a source base name, so
// handlers/home.pw.html and handlers/home.go both suggest the base "home".
func artifactBase(sourcePath string) string {
	base := filepath.Base(sourcePath)
	if index := strings.Index(base, "."); index > 0 {
		return base[:index]
	}
	return base
}

func (g *Generator) templateArtifacts(dir string) ([]Artifact, error) {
	files, err := discoverTemplateFiles(dir, g.Options.HTMLTemplatePattern, g.Options.SQLTemplatePattern)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	pkg, err := g.templatePackageName(dir, files)
	if err != nil {
		return nil, err
	}
	generated := make([][]byte, len(files))
	for i, file := range files {
		source, err := os.ReadFile(file.path)
		if err != nil {
			return nil, err
		}
		if generated[i], err = g.generateTemplate(file, source, pkg); err != nil {
			return nil, err
		}
	}
	return splitTemplateArtifacts(pkg, files, generated)
}

// splitTemplateArtifacts separates the template runtime helpers shared by a
// package from the declarations owned by each template source, so several
// sources of one package can be generated into separate files without
// redeclaring an identifier.
func splitTemplateArtifacts(pkg string, files []templateFile, generated [][]byte) ([]Artifact, error) {
	fset := token.NewFileSet()
	var imports []*ast.ImportSpec
	importKeys := map[string]bool{}
	shared := map[string]bool{}
	var sharedDecls []ast.Decl
	specific := make([][]ast.Decl, len(files))
	owned := map[string]string{}

	for index, source := range generated {
		file, err := parser.ParseFile(fset, files[index].path, source, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, item := range file.Imports {
			alias := ""
			if item.Name != nil {
				alias = item.Name.Name
			}
			key := alias + "\x00" + item.Path.Value
			if importKeys[key] {
				continue
			}
			importKeys[key] = true
			imports = append(imports, item)
		}
		for _, declaration := range file.Decls {
			if gen, ok := declaration.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
				continue
			}
			names := declarationNames(declaration)
			if isSharedTemplateDecl(names) {
				fresh := false
				for _, name := range names {
					if !shared[name] {
						shared[name] = true
						fresh = true
					}
				}
				if fresh {
					sharedDecls = append(sharedDecls, declaration)
				}
				continue
			}
			for _, name := range names {
				if previous, exists := owned[name]; exists {
					return nil, fmt.Errorf("duplicate generated template declaration %s in %s and %s", name, previous, files[index].path)
				}
				owned[name] = files[index].path
			}
			specific[index] = append(specific[index], declaration)
		}
	}

	artifacts := make([]Artifact, 0, len(files)+1)
	if len(sharedDecls) > 0 {
		source, err := renderArtifactFile(fset, pkg, imports, sharedDecls, templateGeneratedHeader)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, Artifact{
			Kind:        ArtifactPackageShared,
			OutputBase:  sharedArtifactBase,
			PackageName: pkg,
			GoSource:    source,
		})
	}
	for index, file := range files {
		if len(specific[index]) == 0 {
			continue
		}
		source, err := renderArtifactFile(fset, pkg, imports, specific[index], templateGeneratedHeader)
		if err != nil {
			return nil, err
		}
		kind := ArtifactHTMLTemplate
		if file.kind == sqlTemplate {
			kind = ArtifactSQLTemplate
		}
		artifacts = append(artifacts, Artifact{
			SourcePath:  file.path,
			Kind:        kind,
			OutputBase:  artifactBase(file.path),
			PackageName: pkg,
			GoSource:    source,
		})
	}
	return artifacts, nil
}

const templateGeneratedHeader = "// Code generated by tinybind templates; DO NOT EDIT.\n\n"

// isSharedTemplateDecl reports whether a declaration belongs to the package
// runtime that every generated template file repeats.
func isSharedTemplateDecl(names []string) bool {
	if len(names) == 0 {
		return false
	}
	for _, name := range names {
		if !templateRuntimeName(name) {
			return false
		}
	}
	return true
}

// renderArtifactFile prints one artifact carrying only the imports its own
// declarations reference, so each artifact compiles on its own.
func renderArtifactFile(fset *token.FileSet, pkg string, imports []*ast.ImportSpec, declarations []ast.Decl, header string) ([]byte, error) {
	decls := declarations
	if len(imports) > 0 {
		specs := make([]ast.Spec, len(imports))
		for i, spec := range imports {
			specs[i] = spec
		}
		decls = append([]ast.Decl{&ast.GenDecl{Tok: token.IMPORT, Specs: specs}}, declarations...)
	}
	var out strings.Builder
	out.WriteString(header)
	if err := format.Node(&out, fset, &ast.File{Name: ast.NewIdent(pkg), Decls: decls}); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return dropUnusedImports([]byte(out.String()))
}

// dropUnusedImports removes imports the source does not reference. It reparses
// the rendered source so identifier resolution is available.
func dropUnusedImports(source []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "artifact.go", source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse generated artifact: %w\n%s", err, source)
	}
	for _, spec := range append([]*ast.ImportSpec(nil), file.Imports...) {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if astutil.UsesImport(file, path) {
			continue
		}
		alias := ""
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		astutil.DeleteNamedImport(fset, file, alias, path)
	}
	var out strings.Builder
	if err := format.Node(&out, fset, file); err != nil {
		return nil, err
	}
	formatted, err := format.Source([]byte(out.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated artifact: %w\n%s", err, out.String())
	}
	return formatted, nil
}

func (g *Generator) bindingArtifacts(dir string) ([]Artifact, error) {
	plan, err := g.Analyze(dir)
	if err != nil {
		return nil, err
	}
	grouped := map[string][]string{}
	var order []string
	for _, t := range plan.Types {
		if t.Usage == 0 {
			continue
		}
		source := t.SourcePath
		if _, seen := grouped[source]; !seen {
			order = append(order, source)
		}
		grouped[source] = append(grouped[source], t.Name)
	}
	sort.Strings(order)
	artifacts := make([]Artifact, 0, len(order))
	for _, source := range order {
		selected := make(map[string]bool, len(grouped[source]))
		for _, name := range grouped[source] {
			selected[name] = true
		}
		code, err := emitSelected(plan, selected)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, Artifact{
			SourcePath:  source,
			Kind:        ArtifactBinding,
			OutputBase:  artifactBase(source),
			PackageName: plan.Package,
			GoSource:    code,
		})
	}
	return artifacts, nil
}

func (g *Generator) configBindArtifacts(dir string) ([]Artifact, error) {
	pkgName, specs, err := AnalyzeConfigBindSources(dir, g.Options)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, nil
	}
	var artifacts []Artifact
	for start := 0; start < len(specs); {
		end := start + 1
		for end < len(specs) && specs[end].SourcePath == specs[start].SourcePath {
			end++
		}
		group := make([]cbcg.Spec, 0, end-start)
		for _, spec := range specs[start:end] {
			group = append(group, spec.Spec)
		}
		// The offset keeps register<Type>Definition<N> unique across the
		// separately generated files of one package.
		code, err := cbcg.GenerateGroup(pkgName, group, start)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, Artifact{
			SourcePath:  specs[start].SourcePath,
			Kind:        ArtifactConfigBind,
			OutputBase:  artifactBase(specs[start].SourcePath),
			PackageName: pkgName,
			GoSource:    code,
		})
		start = end
	}
	return artifacts, nil
}

func (g *Generator) openAPIArtifact(dir string) ([]Artifact, error) {
	doc, err := g.BuildOpenAPI(dir)
	if err != nil {
		return nil, err
	}
	plan, err := g.Analyze(dir)
	if err != nil {
		return nil, err
	}
	code, err := EmitOpenAPIFragment(plan.Package, plan.PackagePath, doc)
	if err != nil {
		return nil, err
	}
	return []Artifact{{
		Kind:        ArtifactOpenAPI,
		OutputBase:  openAPIArtifactBase,
		PackageName: plan.Package,
		GoSource:    code,
	}}, nil
}
