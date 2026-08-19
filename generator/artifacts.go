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
	"github.com/shibukawa/tinybind-go/internal/externalscan"
	"github.com/shibukawa/tinybind-go/internal/linedirective"
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
	// ArtifactTransport is the other transport's copy of the package's
	// handlers, derived from the authored net/http source.
	ArtifactTransport ArtifactKind = "transport"
	// ArtifactTransportBinding is the derived backend's binder registry. The
	// runtime's Bind reads a registry the generated init fills, so a package
	// missing this file compiles and fails on the first request instead.
	ArtifactTransportBinding ArtifactKind = "transport_binding"
	// ArtifactTransportRoutes registers the derived handlers on the router
	// TransformOptions.Router names.
	ArtifactTransportRoutes ArtifactKind = "transport_routes"
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
	transport, _, err := runner.transportArtifacts(load)
	if err != nil {
		return nil, fmt.Errorf("generate transport: %w", err)
	}
	artifacts = append(artifacts, transport...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	transportBinding, err := runner.transportBindingArtifacts(load)
	if err != nil {
		return nil, fmt.Errorf("generate transport binders: %w", err)
	}
	artifacts = append(artifacts, transportBinding...)
	transportRoutes, err := runner.transportRoutesArtifacts(load)
	if err != nil {
		return nil, fmt.Errorf("generate transport routes: %w", err)
	}
	artifacts = append(artifacts, transportRoutes...)
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
	options.DynamoParameterAPI = options.DynamoParameterAPI || request.DynamoParameterAPI
	options.FirestoreParameterAPI = options.FirestoreParameterAPI || request.FirestoreParameterAPI
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
	signatures, err := externalscan.Scan(dir)
	if err != nil {
		return nil, err
	}
	cache := newConversionCache(g.Options.ConversionCacheDir)
	hooks := runScopedHooks(g.Options.ReferenceHooks, cache)
	g.prewarmConversions(files, hooks)
	var produced []htmlbind.ProducedFile
	generated := make([][]byte, len(files))
	assets := make([][]htmlbind.Asset, len(files))
	for i, file := range files {
		source, err := os.ReadFile(file.path)
		if err != nil {
			return nil, err
		}
		code, compiled, err := g.generateTemplate(file, source, pkg, signatures, hooks)
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
	declared := make([]int, len(files))
	owned := map[string]string{}

	for index, source := range generated {
		file, err := parser.ParseFile(fset, files[index].path, source, parser.ParseComments)
		if err != nil {
			return nil, err
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
			declared[index]++
		}
	}

	artifacts := make([]Artifact, 0, len(files))
	for index, file := range files {
		if declared[index] == 0 {
			continue
		}
		// One template's output is already a whole file, so combining its own
		// bytes with nothing is what renders it. Rebuilding it from the parsed
		// declarations would drop every comment that is not a declaration doc,
		// and a requirement:template-source-positions directive is one of those.
		source, err := combineGeneratedTemplates(pkg, [][]byte{generated[index]})
		if err != nil {
			return nil, err
		}
		// The lines are final here and the name is not: an artifact states a
		// suggested base and the caller chooses the suffix it writes under. See
		// [ResolveTemplatePositions].
		source = linedirective.Finalize(source)
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

// ResolveTemplatePositions names the file an artifact is written as, in the
// line directives requirement:template-source-positions emitted into it.
//
// A mapped span ends with a directive returning positions to the generated
// file, and that directive has to state the file's name and the physical line
// the reader is on. This package knows the line, because the bytes are final;
// it does not know the name, because [Artifact] states a suggested base and the
// caller chooses the suffix it writes. A caller writing artifacts under its own
// name calls this with that name before writing:
//
//	content := generator.ResolveTemplatePositions(artifact.Content, artifact.OutputBase+"_pw_gen.go")
//
// For an [Artifact] only the name is filled in, because the line numbers were
// already correct when the artifact was built: they describe bytes nothing has
// moved since. Content taken straight out of a template package's own Generate
// entry point, which cannot know the name either, has both filled in here.
//
// It is a no-op on content holding no such directive, so it is safe to call on
// whatever the run generated and whether or not Options.TemplateLineDirectives
// is set. Skipping it leaves the directives naming a synthetic file that does
// not exist, which misreports the position of generated scaffolding and nothing
// else: a template position is unaffected.
func ResolveTemplatePositions(content []byte, fileName string) []byte {
	return linedirective.Resolve(content, fileName)
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

func (g *Generator) bindingArtifacts(load *packageLoad) ([]Artifact, error) {
	plan, err := analyzeLoadedPackage(load, g.Options)
	if err != nil {
		return nil, err
	}
	grouped := map[string][]string{}
	var order []string
	seeSource := func(source string) {
		if _, seen := grouped[source]; !seen {
			order = append(order, source)
			grouped[source] = nil
		}
	}
	for _, t := range plan.Types {
		if t.Usage == 0 {
			continue
		}
		seeSource(t.SourcePath)
		grouped[t.SourcePath] = append(grouped[t.SourcePath], t.Name)
	}
	// An action declaring no parameter contributes no type, so its file would
	// otherwise have no artifact and its entry point would be written nowhere.
	for _, action := range plan.ServerActions {
		seeSource(actionSourceKey(action.SourcePath, order))
	}
	sort.Strings(order)
	artifacts := make([]Artifact, 0, len(order))
	for _, source := range order {
		selected := make(map[string]bool, len(grouped[source]))
		for _, name := range grouped[source] {
			selected[name] = true
		}
		// Scoped rather than filtered inside the emitter: the grouping is this
		// function's, and an emitter given the whole list has no way to know
		// which artifact it is writing. Emitting them all in every artifact is
		// what declared the entry point once per source file of the package.
		scoped := *plan
		scoped.ServerActions = actionsForSource(plan.ServerActions, source, order)
		code, err := emitSelected(&scoped, selected)
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

// actionSourceKey maps an action's declaring file onto the source key its
// artifact is grouped under.
//
// routetree reports the file as it walked it and the plan carries the path the
// loader reported, which need not be spelled alike. A package is one directory,
// so the base name identifies the file within it, and matching on that is
// immune to one side being absolute and the other relative.
func actionSourceKey(declaredIn string, order []string) string {
	if declaredIn == "" {
		return ""
	}
	base := filepath.Base(declaredIn)
	for _, source := range order {
		if source != "" && filepath.Base(source) == base {
			return source
		}
	}
	return declaredIn
}

// actionsForSource selects the actions declared in one source file.
func actionsForSource(actions []ServerAction, source string, order []string) []ServerAction {
	var out []ServerAction
	for _, action := range actions {
		if actionSourceKey(action.SourcePath, order) == source {
			out = append(out, action)
		}
	}
	return out
}

// transportArtifactBase names the generated transport file. It is package-wide
// rather than per-source, because the transform closes over the call graph and
// a handler's helper may be authored in another file.
const transportArtifactBase = "tinybind_transport"

// transportArtifacts derives the other transport's handlers from the authored
// source. It runs only when a backend is selected, so a default run produces
// byte-identical output to one predating the feature.
//
// A refusal stops the run. decision:backend-build-tag-mode leaves no adapter,
// so there is nowhere for an untransformable handler to go, and emitting the
// rest would produce a package that silently serves fewer routes.
func (g *Generator) transportArtifacts(load *packageLoad) ([]Artifact, []string, error) {
	if g.Options.Transform == nil {
		return nil, nil, nil
	}
	pkg, err := load.get()
	if err != nil {
		return nil, nil, err
	}
	transform := g.transformOptions()
	plan, err := AnalyzeTransform(pkg, transform)
	if err != nil {
		return nil, nil, err
	}
	if len(plan.Refusals) > 0 {
		return nil, nil, plan.Refusals
	}
	if len(plan.Admitted) == 0 {
		return nil, nil, nil
	}
	out, err := RewriteTransform(pkg, plan, transform)
	if err != nil {
		return nil, nil, err
	}
	return []Artifact{{
		Kind:        ArtifactTransport,
		Destination: DestinationGoPackage,
		OutputBase:  transportArtifactBase,
		Extension:   ExtensionGo,
		PackageName: pkg.Name,
		Content:     out.Source,
	}}, out.LayoutWarnings, nil
}

// transportBindingArtifactBase and transportRoutesArtifactBase name the files
// completing the derived backend, matching what GeneratePackage writes.
const (
	transportBindingArtifactBase = "tinybind_fasthttp"
	transportRoutesArtifactBase  = "tinybind_routes"
)

// transportBindingArtifacts is the derived backend's half of the binder pair.
// bindingArtifacts emits the authored net/http target alone, and a caller
// generating through the artifact API would otherwise get handlers and routes
// for a backend whose binders were never registered.
func (g *Generator) transportBindingArtifacts(load *packageLoad) ([]Artifact, error) {
	if g.Options.Transform == nil {
		return nil, nil
	}
	plan, err := analyzeLoadedPackage(load, g.Options)
	if err != nil {
		return nil, err
	}
	if len(plan.Types) == 0 {
		return nil, nil
	}
	code, err := emitSelectedFor(plan, nil, fasthttpTarget())
	if err != nil {
		return nil, err
	}
	return []Artifact{{
		Kind:        ArtifactTransportBinding,
		Destination: DestinationGoPackage,
		OutputBase:  transportBindingArtifactBase,
		Extension:   ExtensionGo,
		PackageName: plan.Package,
		Content:     code,
	}}, nil
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
