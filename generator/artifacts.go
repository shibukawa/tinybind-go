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
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// ArtifactKind classifies one generated output unit.
type ArtifactKind string

const (
	ArtifactHTMLTemplate ArtifactKind = "html_template"
	ArtifactSQLTemplate  ArtifactKind = "sql_template"
	ArtifactBinding      ArtifactKind = "binding"
	ArtifactConfigBind   ArtifactKind = "configbind"
	ArtifactDynamoItem   ArtifactKind = "dynamo_item"
	ArtifactDynamoQuery  ArtifactKind = "dynamo_query"

	ArtifactFirestoreEntity ArtifactKind = "firestore_entity"
	ArtifactFirestoreQuery  ArtifactKind = "firestore_query"
	ArtifactOpenAPI         ArtifactKind = "openapi"
	// ArtifactStylesheet is the component CSS extracted from one template.
	ArtifactStylesheet ArtifactKind = "stylesheet"
	// ArtifactScript is the component JavaScript extracted from one template.
	ArtifactScript ArtifactKind = "script"
	// ArtifactDerivedAsset is a file a reference hook transform produced from an
	// authored source the template points at.
	ArtifactDerivedAsset ArtifactKind = "derived_asset"
)

// ArtifactDestination says where an artifact is written, because a stylesheet
// is served, not compiled.
type ArtifactDestination string

const (
	DestinationGoPackage   ArtifactDestination = "go_package"
	DestinationPublicAsset ArtifactDestination = "public_asset"
)

// Artifact extensions, without a leading dot.
const (
	ExtensionGo  = "go"
	ExtensionCSS = "css"
	ExtensionJS  = "js"
)

// Artifact is one generated output unit and the source file that owns it. The
// caller maps OutputBase to its own generated file name; nothing is written to
// disk by the API that produces Artifacts.
//
// Go formatting and import correctness apply to a go_package destination only;
// a public asset is written verbatim.
type Artifact struct {
	// SourcePath is the real on-disk path of the owning source. It is empty for
	// package-wide artifacts.
	SourcePath  string
	Kind        ArtifactKind
	Destination ArtifactDestination
	// OutputBase is the suggested output base name, without directory,
	// extension, or generated-file suffix.
	OutputBase string
	// Extension is the output file extension without a dot.
	Extension string
	// PackageName is meaningful for a go_package destination only.
	PackageName string
	Content     []byte
	// PublicPath is the URL a public asset is referenced by. It is empty for a
	// go_package destination.
	PublicPath string
}

// openAPIArtifactBase names the only remaining package-wide artifact.
const openAPIArtifactBase = "tinybind_openapi"

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
	if err := request.validate(); err != nil {
		return nil, err
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
	// Every remaining phase reads the same type-checked package.
	load := newPackageLoad(request.Dir)
	binding, err := runner.bindingArtifacts(load)
	if err != nil {
		return nil, fmt.Errorf("generate mapping: %w", err)
	}
	artifacts = append(artifacts, binding...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	config, err := runner.configBindArtifacts(load)
	if err != nil {
		return nil, fmt.Errorf("generate configbind: %w", err)
	}
	artifacts = append(artifacts, config...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items, err := runner.dynamoItemArtifacts(load)
	if err != nil {
		return nil, fmt.Errorf("generate dynamobind: %w", err)
	}
	artifacts = append(artifacts, items...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	queries, err := runner.dynamoQueryArtifacts(load)
	if err != nil {
		return nil, fmt.Errorf("generate dynamobind queries: %w", err)
	}
	artifacts = append(artifacts, queries...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entities, err := runner.firestoreEntityArtifacts(load)
	if err != nil {
		return nil, fmt.Errorf("generate firestorebind: %w", err)
	}
	artifacts = append(artifacts, entities...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	firestoreQueries, err := runner.firestoreQueryArtifacts(load)
	if err != nil {
		return nil, fmt.Errorf("generate firestorebind queries: %w", err)
	}
	artifacts = append(artifacts, firestoreQueries...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.OpenAPI && normalized.openAPI {
		openAPI, err := runner.openAPIArtifact(load)
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
	if request.PublicDir != "" {
		options.PublicDir = request.PublicDir
	}
	if request.PublicURLBase != "" {
		options.PublicURLBase = request.PublicURLBase
	}
	if request.SQLDialect != "" {
		options.SQLDialect = request.SQLDialect
	}
	return options
}

// validate rejects a request that half-configures the public asset pair, which
// the option defaults would otherwise silently complete.
func (request GenerateRequest) validate() error {
	return checkPublicAssetPairing(request.PublicDir, request.PublicURLBase)
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
	if err := checkSQLDialect(files, g.Options.SQLDialect); err != nil {
		return nil, err
	}
	pkg, err := g.templatePackageName(dir, files)
	if err != nil {
		return nil, err
	}
	withContext, err := contextExternals(dir)
	if err != nil {
		return nil, err
	}
	cache := newConversionCache(g.Options.ConversionCacheDir)
	hooks := runScopedHooks(g.Options.ReferenceHooks, cache)
	var produced []htmlbind.ProducedFile
	generated := make([][]byte, len(files))
	assets := make([][]htmlbind.Asset, len(files))
	for i, file := range files {
		source, err := os.ReadFile(file.path)
		if err != nil {
			return nil, err
		}
		code, compiled, err := g.generateTemplate(file, source, pkg, withContext, hooks)
		if err != nil {
			return nil, err
		}
		generated[i], assets[i] = code, compiled.Assets
		produced = append(produced, compiled.Produced...)
	}
	artifacts, err := splitTemplateArtifacts(pkg, files, generated)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, assetArtifacts(files, assets)...)
	return append(artifacts, producedArtifacts(produced)...), nil
}

// producedArtifacts turns the files a run's conversions created into artifacts,
// so a caller taking artifacts instead of written files still receives them.
//
// They carry no source path: the batch spans the run, and a file two templates
// both depend on belongs to neither of them.
//
// The output base carries the whole relative name, extension included, because
// a hook owns naming: an image hook appends to make a.png.webp and a TypeScript
// hook replaces to make app.js, and no rule here can satisfy both.
func producedArtifacts(produced []htmlbind.ProducedFile) []Artifact {
	artifacts := make([]Artifact, 0, len(produced))
	for _, output := range produced {
		artifacts = append(artifacts, Artifact{
			Kind:        ArtifactDerivedAsset,
			Destination: DestinationPublicAsset,
			OutputBase:  output.Name,
			Content:     output.Content,
		})
	}
	return artifacts
}

// assetArtifacts turns extracted static files into artifacts bound to the
// template that produced them. Two templates emitting the same file name emit
// identical bytes, so the duplicate is dropped rather than written twice.
func assetArtifacts(files []templateFile, assets [][]htmlbind.Asset) []Artifact {
	var artifacts []Artifact
	seen := map[string]bool{}
	for index, file := range files {
		for _, asset := range assets[index] {
			if seen[asset.FileName()] {
				continue
			}
			seen[asset.FileName()] = true
			kind := ArtifactStylesheet
			if asset.Kind == htmlbind.AssetScript {
				kind = ArtifactScript
			}
			artifacts = append(artifacts, Artifact{
				SourcePath:  file.path,
				Kind:        kind,
				Destination: DestinationPublicAsset,
				OutputBase:  asset.Base,
				Extension:   asset.Extension,
				Content:     asset.Content,
				PublicPath:  asset.URL,
			})
		}
	}
	return artifacts
}

// splitTemplateArtifacts turns each template source into its own artifact.
// Generated code declares no runtime, so every declaration belongs to exactly
// one source and no package-wide artifact is needed.
func splitTemplateArtifacts(pkg string, files []templateFile, generated [][]byte) ([]Artifact, error) {
	fset := token.NewFileSet()
	var imports []*ast.ImportSpec
	importKeys := map[string]bool{}
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
			for _, name := range declarationNames(declaration) {
				if previous, exists := owned[name]; exists {
					return nil, fmt.Errorf("duplicate generated template declaration %s in %s and %s", name, previous, files[index].path)
				}
				owned[name] = files[index].path
			}
			specific[index] = append(specific[index], declaration)
		}
	}

	artifacts := make([]Artifact, 0, len(files))
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
			Destination: DestinationGoPackage,
			OutputBase:  artifactBase(file.path),
			Extension:   ExtensionGo,
			PackageName: pkg,
			Content:     source,
		})
	}
	return artifacts, nil
}

const templateGeneratedHeader = "// Code generated by tinybind templates; DO NOT EDIT.\n\n"

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

func (g *Generator) bindingArtifacts(load *packageLoad) ([]Artifact, error) {
	plan, err := analyzeLoadedPackage(load, g.Options)
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
			Destination: DestinationGoPackage,
			OutputBase:  artifactBase(source),
			Extension:   ExtensionGo,
			PackageName: plan.Package,
			Content:     code,
		})
	}
	return artifacts, nil
}

func (g *Generator) configBindArtifacts(load *packageLoad) ([]Artifact, error) {
	pkgName, specs, err := configBindSources(load, g.Options)
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

// dynamoItemArtifacts emits one item codec artifact per source file that
// declares a bound type, so a package with two DynamoDB sources generates two
// files and neither owns the other's declarations.
func (g *Generator) dynamoItemArtifacts(load *packageLoad) ([]Artifact, error) {
	if g.Options.featureDisabled(FeatureItemCodec) {
		return nil, nil
	}
	plan, err := analyzeDynamoItems(load, g.Options)
	if err != nil {
		return nil, err
	}
	grouped := map[string][]string{}
	var order []string
	for _, item := range plan.Items {
		if item.Usage == 0 {
			continue
		}
		if _, seen := grouped[item.SourcePath]; !seen {
			order = append(order, item.SourcePath)
		}
		grouped[item.SourcePath] = append(grouped[item.SourcePath], item.Name)
	}
	sort.Strings(order)
	emitTable := !g.Options.featureDisabled(FeatureItemTable)
	artifacts := make([]Artifact, 0, len(order))
	for _, source := range order {
		selected := make(map[string]bool, len(grouped[source]))
		for _, name := range grouped[source] {
			selected[name] = true
		}
		code, err := emitDynamoSelected(plan, selected, emitTable)
		if err != nil {
			return nil, err
		}
		if len(code) == 0 {
			continue
		}
		artifacts = append(artifacts, Artifact{
			SourcePath:  source,
			Kind:        ArtifactDynamoItem,
			Destination: DestinationGoPackage,
			OutputBase:  artifactBase(source),
			Extension:   ExtensionGo,
			PackageName: plan.Package,
			Content:     code,
		})
	}
	return artifacts, nil
}

func (g *Generator) openAPIArtifact(load *packageLoad) ([]Artifact, error) {
	doc, err := g.buildOpenAPI(load)
	if err != nil {
		return nil, err
	}
	plan, err := analyzeLoadedPackage(load, g.Options)
	if err != nil {
		return nil, err
	}
	code, err := EmitOpenAPIFragment(plan.Package, plan.PackagePath, doc)
	if err != nil {
		return nil, err
	}
	return []Artifact{{
		Kind:        ArtifactOpenAPI,
		Destination: DestinationGoPackage,
		OutputBase:  openAPIArtifactBase,
		Extension:   ExtensionGo,
		PackageName: plan.Package,
		Content:     code,
	}}, nil
}
