package generator

import (
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
// writes one Go file containing all generated declarations. It returns an empty
// path when no templates exist.
func (g *Generator) GenerateTemplates(dir, outDir, outName string) (string, error) {
	files, err := discoverTemplateFiles(dir, g.Options.HTMLTemplatePattern, g.Options.SQLTemplatePattern)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	pkg, err := g.templatePackageName(dir, files)
	if err != nil {
		return "", err
	}
	withContext, err := contextExternals(dir)
	if err != nil {
		return "", err
	}
	var generated [][]byte
	for _, file := range files {
		source, err := os.ReadFile(file.path)
		if err != nil {
			return "", err
		}
		code, err := g.generateTemplate(file, source, pkg, withContext)
		if err != nil {
			return "", err
		}
		generated = append(generated, code)
	}
	combined, err := combineGeneratedTemplates(pkg, generated)
	if err != nil {
		return "", err
	}
	if outDir == "" {
		outDir = dir
	}
	if outName == "" {
		outName = DefaultTemplatesName
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, outName)
	if err := os.WriteFile(path, combined, 0o644); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
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
// generated API shape. Diagnostics keep the discovered path, so custom input
// suffixes are reported exactly as they exist on disk.
func (g *Generator) generateTemplate(file templateFile, source []byte, pkg string, contextExternals map[string]bool) ([]byte, error) {
	if file.kind == htmlTemplate {
		module, err := htmlbind.Parse(file.path, source)
		if err != nil {
			return nil, err
		}
		if err := checkTemplatePackage(file.path, module.Package, pkg); err != nil {
			return nil, err
		}
		return htmlbind.Generate(file.path, source, htmlbind.GenerateOptions{
			Package:            pkg,
			ContextExternals:   contextExternals,
			PreserveWhitespace: g.Options.PreserveTemplateWhitespace,
		})
	}
	module, err := templatesql.Parse(file.path, source)
	if err != nil {
		return nil, err
	}
	if err := checkTemplatePackage(file.path, module.Package, pkg); err != nil {
		return nil, err
	}
	options := templatesql.GenerateOptions{
		Package:     pkg,
		ContextAPI:  g.Options.SQLContextAPI || g.Options.SQLContextOnlyAPI,
		ContextOnly: g.Options.SQLContextOnlyAPI,
	}
	if resolver := g.Options.SQLExecutorResolver; resolver != nil {
		options.ExecutorResolver = &templatesql.ExecutorResolver{PackagePath: resolver.PackagePath, Name: resolver.Name}
	}
	return templatesql.Generate(file.path, source, options)
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
// An async external is an ordinary blocking Go function, so the template
// declaration says nothing about a context. Reading the implementation lets a
// function that can abort receive the boundary's context without a second
// declaration form: write the parameter and it is passed, leave it out and the
// function is called plainly.
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
