package generator

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
	templatesql "github.com/shibukawa/tinybind-go/templates/sqlbind"
)

const (
	DefaultTemplatesName       = "tinybind_templates_gen.go"
	DefaultHTMLTemplatePattern = "*.tb.html"
	DefaultSQLTemplatePattern  = "*.tb.sql"
)

type templateKind uint8

const (
	htmlTemplate templateKind = iota
	sqlTemplate
)

type templateFile struct {
	path string
	kind templateKind
}

// TemplateFiles returns the .tb.html and .tb.sql files directly contained in
// dir. A generator invocation targets one Go package and therefore does not
// descend into child package directories.
func TemplateFiles(dir string) ([]string, error) {
	return TemplateFilesWithPatterns(dir, DefaultHTMLTemplatePattern, DefaultSQLTemplatePattern)
}

// TemplateFilesWithPatterns returns files directly contained in dir whose base
// names match the filepath.Match patterns for HTML and SQL templates.
func TemplateFilesWithPatterns(dir, htmlPattern, sqlPattern string) ([]string, error) {
	discovered, err := discoverTemplateFiles(dir, htmlPattern, sqlPattern)
	if err != nil {
		return nil, err
	}
	files := make([]string, len(discovered))
	for i, file := range discovered {
		files[i] = file.path
	}
	return files, nil
}

func discoverTemplateFiles(dir, htmlPattern, sqlPattern string) ([]templateFile, error) {
	htmlPattern = templatePattern(htmlPattern, DefaultHTMLTemplatePattern)
	sqlPattern = templatePattern(sqlPattern, DefaultSQLTemplatePattern)
	if _, err := filepath.Match(htmlPattern, ""); err != nil {
		return nil, fmt.Errorf("invalid HTML template pattern %q: %w", htmlPattern, err)
	}
	if _, err := filepath.Match(sqlPattern, ""); err != nil {
		return nil, fmt.Errorf("invalid SQL template pattern %q: %w", sqlPattern, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []templateFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		html, err := filepath.Match(htmlPattern, name)
		if err != nil {
			return nil, err // patterns were validated above
		}
		sql, err := filepath.Match(sqlPattern, name)
		if err != nil {
			return nil, err // patterns were validated above
		}
		if html && sql {
			return nil, fmt.Errorf("template file %q matches both HTML pattern %q and SQL pattern %q", name, htmlPattern, sqlPattern)
		}
		if html {
			files = append(files, templateFile{path: filepath.Join(dir, name), kind: htmlTemplate})
		} else if sql {
			files = append(files, templateFile{path: filepath.Join(dir, name), kind: sqlTemplate})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files, nil
}

// GenerateTemplates discovers files using the configured template patterns and
// writes one Go file containing all generated declarations, plus the static
// files extracted from component style and script blocks. It returns an empty
// path when no templates exist.
func (g *Generator) GenerateTemplates(dir, outDir, outName string) (string, error) {
	outputs, err := g.generateTemplateFiles(dir, outDir, outName)
	return outputs.goPath, err
}

// templateOutputs is everything one template phase wrote and learned.
//
// The hook fields are carried out rather than logged here, because the caller
// owns what a measurement means: this package reports what it did.
type templateOutputs struct {
	goPath string
	// assetPaths holds the extracted stylesheets and scripts, then the files
	// reference hook transforms produced, each in generation order.
	assetPaths []string
	rewrites   []htmlbind.Rewrite
	// readSet holds every authored file a transform reported reading, sorted
	// and deduplicated across the templates of the run.
	readSet []string
	dynamic []htmlbind.DynamicReference
}

// generateTemplateFiles writes the generated Go file, every extracted asset,
// and every file a reference hook produced.
func (g *Generator) generateTemplateFiles(dir, outDir, outName string) (templateOutputs, error) {
	files, err := discoverTemplateFiles(dir, g.Options.HTMLTemplatePattern, g.Options.SQLTemplatePattern)
	if err != nil {
		return templateOutputs{}, err
	}
	if len(files) == 0 {
		return templateOutputs{}, nil
	}
	if err := checkPublicAssetPairing(g.Options.PublicDir, g.Options.PublicURLBase); err != nil {
		return templateOutputs{}, err
	}
	// The dialect is a configuration error, not a template diagnostic, so it is
	// reported once against the discovered set and before anything is written.
	if err := checkSQLDialect(files, g.Options.SQLDialect); err != nil {
		return templateOutputs{}, err
	}
	pkg, err := g.templatePackageName(dir, files)
	if err != nil {
		return templateOutputs{}, err
	}
	withContext, err := contextExternals(dir)
	if err != nil {
		return templateOutputs{}, err
	}
	cache := newConversionCache(g.Options.ConversionCacheDir)
	hooks := runScopedHooks(g.Options.ReferenceHooks, cache)
	var produced []htmlbind.ProducedFile
	var generated [][]byte
	var assets []htmlbind.Asset
	var outputs templateOutputs
	read := map[string]bool{}
	for _, file := range files {
		source, err := os.ReadFile(file.path)
		if err != nil {
			return templateOutputs{}, err
		}
		code, compiled, err := g.generateTemplate(file, source, pkg, withContext, hooks)
		if err != nil {
			return templateOutputs{}, err
		}
		generated = append(generated, code)
		assets = append(assets, compiled.Assets...)
		produced = append(produced, compiled.Produced...)
		outputs.rewrites = append(outputs.rewrites, compiled.Rewrites...)
		outputs.dynamic = append(outputs.dynamic, compiled.DynamicReferences...)
		for _, name := range compiled.ReadSet {
			read[name] = true
		}
	}
	// A source named by a cache key is a build input whether its conversion ran
	// or was answered from the store, so an edit to it regenerates either way.
	for _, source := range cache.namedSources() {
		read[source] = true
	}
	outputs.readSet = sortedKeys(read)
	combined, err := combineGeneratedTemplates(pkg, generated)
	if err != nil {
		return templateOutputs{}, err
	}
	if outDir == "" {
		outDir = dir
	}
	if outName == "" {
		outName = DefaultTemplatesName
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return templateOutputs{}, err
	}
	path := filepath.Join(outDir, outName)
	if err := os.WriteFile(path, combined, 0o644); err != nil {
		return templateOutputs{}, err
	}
	assetPaths, err := g.writeAssets(assets)
	if err != nil {
		return templateOutputs{}, err
	}
	derivedPaths, err := g.writeProduced(produced)
	if err != nil {
		return templateOutputs{}, err
	}
	outputs.assetPaths = append(assetPaths, derivedPaths...)
	outputs.goPath = path
	if abs, err := filepath.Abs(path); err == nil {
		outputs.goPath = abs
	}
	return outputs, nil
}

func sortedKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// writeProduced writes the files reference hook transforms created, into the
// configured derived directory.
//
// A transform could write these itself and hand back only the rewritten string,
// and that is exactly what must not happen: a file written behind the generator
// is absent from the recorded output set, so --check cannot compare it, the
// skip path cannot verify it, and nothing can ever clean it up. Byte production
// belongs to the transform; the bookkeeping belongs here.
func (g *Generator) writeProduced(produced []htmlbind.ProducedFile) ([]string, error) {
	if len(produced) == 0 {
		return nil, nil
	}
	if g.Options.DerivedAssetDir == "" {
		return nil, ErrDerivedAssetDir
	}
	written := map[string]bool{}
	var paths []string
	for _, file := range produced {
		if written[file.Name] {
			continue
		}
		written[file.Name] = true
		path := filepath.Join(g.Options.DerivedAssetDir, filepath.FromSlash(file.Name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, file.Content, 0o644); err != nil {
			return nil, err
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// ErrDerivedAssetDir reports a hook that produced a file with nowhere to put
// it. Discarding it silently would leave the rewritten reference dangling,
// which is the one property this seam exists to guarantee.
var ErrDerivedAssetDir = errors.New(
	"generator: a reference hook produced a file but DerivedAssetDir is not set; " +
		"it is not derived from PublicDir, because a transform chooses the URL it rewrites to " +
		"and only the caller knows which directory is served there")

// writeAssets writes extracted static files into the configured public
// directory, exactly as the generator writes Go artifacts. A file name carries
// no separator, so a written asset cannot escape that directory.
func (g *Generator) writeAssets(assets []htmlbind.Asset) ([]string, error) {
	if len(assets) == 0 {
		return nil, nil
	}
	publicDir := g.Options.resolvedPublicDir()
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		return nil, err
	}
	written := map[string]bool{}
	var paths []string
	for _, asset := range assets {
		name := asset.FileName()
		if written[name] {
			continue
		}
		written[name] = true
		path := filepath.Join(publicDir, name)
		if err := os.WriteFile(path, asset.Content, 0o644); err != nil {
			return nil, err
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// templatePackageName resolves the Go package the generated templates join:
// the package of the Go sources in dir, or the package declared by the first
// template that declares one.
func (g *Generator) templatePackageName(dir string, files []templateFile) (string, error) {
	pkg, err := packageName(dir)
	if err != nil {
		return "", err
	}
	if pkg != "" {
		return pkg, nil
	}
	for _, file := range files {
		source, err := os.ReadFile(file.path)
		if err != nil {
			return "", err
		}
		declared := ""
		if file.kind == htmlTemplate {
			module, err := htmlbind.Parse(file.path, source)
			if err != nil {
				return "", err
			}
			if module.Package != nil {
				declared = module.Package.Name
			}
		} else {
			module, err := templatesql.Parse(file.path, source)
			if err != nil {
				return "", err
			}
			if module.Package != nil {
				declared = module.Package.Name
			}
		}
		if declared == "" {
			continue
		}
		if i := strings.LastIndex(declared, "."); i >= 0 {
			declared = declared[i+1:]
		}
		return goTemplateIdentifier(declared), nil
	}
	return "templates", nil
}

// generateTemplate compiles one discovered template source with the configured
// generated API shape, returning the Go source and the static files extracted
// from it. Diagnostics keep the discovered path, so custom input suffixes are
// reported exactly as they exist on disk.
func (g *Generator) generateTemplate(file templateFile, source []byte, pkg string, contextExternals map[string]bool, hooks []htmlbind.ReferenceHook) ([]byte, htmlbind.Result, error) {
	if file.kind == htmlTemplate {
		module, err := htmlbind.Parse(file.path, source)
		if err != nil {
			return nil, htmlbind.Result{}, err
		}
		if err := checkTemplatePackage(file.path, module.Package, pkg); err != nil {
			return nil, htmlbind.Result{}, err
		}
		result, err := htmlbind.GenerateModule(file.path, source, htmlbind.GenerateOptions{
			Package:             pkg,
			Unit:                artifactBase(file.path),
			PublicURLBase:       g.Options.resolvedPublicURLBase(),
			ContextExternals:    contextExternals,
			PreserveWhitespace:  g.Options.PreserveTemplateWhitespace,
			DataAttributePrefix: g.Options.DataAttributePrefix,
			ReferenceHooks:      hooks,
		})
		if err != nil {
			return nil, htmlbind.Result{}, err
		}
		return result.GoSource, result, nil
	}
	module, err := templatesql.Parse(file.path, source)
	if err != nil {
		return nil, htmlbind.Result{}, err
	}
	if err := checkTemplatePackage(file.path, module.Package, pkg); err != nil {
		return nil, htmlbind.Result{}, err
	}
	options := templatesql.GenerateOptions{
		Package:     pkg,
		Dialect:     g.Options.SQLDialect,
		ContextAPI:  g.Options.SQLContextAPI || g.Options.SQLContextOnlyAPI,
		ContextOnly: g.Options.SQLContextOnlyAPI,
	}
	if resolver := g.Options.SQLExecutorResolver; resolver != nil {
		options.ExecutorResolver = &templatesql.ExecutorResolver{PackagePath: resolver.PackagePath, Name: resolver.Name}
	}
	code, err := templatesql.Generate(file.path, source, options)
	return code, htmlbind.Result{}, err
}

// checkSQLDialect validates the configured dialect when the run discovers a SQL
// template. A package holding only HTML templates needs no dialect.
func checkSQLDialect(files []templateFile, dialect string) error {
	found := false
	for _, file := range files {
		if file.kind == sqlTemplate {
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	if err := templatesql.ValidateDialect(dialect); err != nil {
		return fmt.Errorf("%w; set Options.SQLDialect or -sql-dialect", err)
	}
	return nil
}

func checkTemplatePackage(filename string, declaration *htmlbind.PackageDecl, pkg string) error {
	if declaration == nil || pkg == "" {
		return nil
	}
	name := declaration.Name
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if goTemplateIdentifier(name) != pkg {
		return fmt.Errorf("%s: template package %q does not match Go package %q", filename, name, pkg)
	}
	return nil
}

func packageName(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, parser.PackageClauseOnly)
		if err == nil {
			return file.Name.Name, nil
		}
	}
	return "", nil
}

// contextExternals names the package-level functions in dir whose first
// parameter is a context.Context.
//
// An external is an ordinary Go function, so the template declaration says
// nothing about a context. Reading the implementation lets a function that needs
// one receive it without a second declaration form: write the parameter and it
// is passed, leave it out and the function is called plainly.
//
// An async external gets the boundary's context, which is what lets a call that
// can abort observe cancellation. A synchronous one gets the render context at
// the position it occupies, which is what lets a value belonging to the request
// — a CSRF token, a nonce — render inline without travelling through the
// parameter struct of every page that needs it.
//
// Detection is syntactic on purpose. It runs before the package compiles, so a
// file that does not parse is skipped rather than failing generation; a call
// shape that then does not match is an ordinary Go compile error at the
// generated call site.
func contextExternals(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			// A method cannot be an external, so a receiver rules it out.
			if !ok || function.Recv != nil || function.Name == nil {
				continue
			}
			if takesLeadingContext(function.Type) {
				found[function.Name.Name] = true
			}
		}
	}
	return found, nil
}

func takesLeadingContext(signature *ast.FuncType) bool {
	if signature.Params == nil || len(signature.Params.List) == 0 {
		return false
	}
	selector, ok := signature.Params.List[0].Type.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Context" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "context"
}

func combineGeneratedTemplates(pkg string, sources [][]byte) ([]byte, error) {
	fset := token.NewFileSet()
	imports := map[string]*ast.ImportSpec{}
	seen := map[string]bool{}
	var declarations []ast.Decl
	for index, source := range sources {
		file, err := parser.ParseFile(fset, fmt.Sprintf("template_%d.go", index), source, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, item := range file.Imports {
			alias := ""
			if item.Name != nil {
				alias = item.Name.Name
			}
			imports[alias+"\x00"+item.Path.Value] = item
		}
		for _, declaration := range file.Decls {
			if gen, ok := declaration.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
				continue
			}
			// Every declaration now derives from its own template source, so a
			// repeated name is a genuine conflict rather than a shared runtime
			// helper that each file happened to emit.
			names := declarationNames(declaration)
			for _, name := range names {
				if seen[name] {
					return nil, fmt.Errorf("duplicate generated template declaration %s", name)
				}
			}
			declarations = append(declarations, declaration)
			for _, name := range names {
				seen[name] = true
			}
		}
	}
	keys := make([]string, 0, len(imports))
	for key := range imports {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		specs := make([]ast.Spec, 0, len(keys))
		for _, key := range keys {
			specs = append(specs, imports[key])
		}
		declarations = append([]ast.Decl{&ast.GenDecl{Tok: token.IMPORT, Specs: specs}}, declarations...)
	}
	file := &ast.File{Name: ast.NewIdent(pkg), Decls: declarations}
	var out strings.Builder
	out.WriteString(templateGeneratedHeader)
	if err := format.Node(&out, fset, file); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	// Merging several template files unions their import blocks, so an import
	// one file needed can be unused by the combined declarations.
	return dropUnusedImports([]byte(out.String()))
}

func declarationNames(declaration ast.Decl) []string {
	var names []string
	switch d := declaration.(type) {
	case *ast.FuncDecl:
		names = append(names, d.Name.Name)
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				names = append(names, s.Name.Name)
			case *ast.ValueSpec:
				for _, name := range s.Names {
					names = append(names, name.Name)
				}
			}
		}
	}
	return names
}
func goTemplateIdentifier(value string) string {
	var out strings.Builder
	for i, r := range value {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9') {
			out.WriteRune(r)
		} else {
			out.WriteRune('_')
		}
	}
	result := out.String()
	if result == "" {
		return "templates"
	}
	if templateGoKeywords[result] {
		return "_" + result
	}
	return result
}

var templateGoKeywords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true, "select": true,
	"case": true, "defer": true, "go": true, "map": true, "struct": true,
	"chan": true, "else": true, "goto": true, "package": true, "switch": true,
	"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
	"continue": true, "for": true, "import": true, "return": true, "var": true,
}
