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

const DefaultTemplatesName = "tinybind_templates_gen.go"

// TemplateFiles returns the .tb.html and .tb.sql files directly contained in
// dir. A generator invocation targets one Go package and therefore does not
// descend into child package directories.
func TemplateFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".tb.html") || strings.HasSuffix(name, ".tb.sql") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)
	return files, nil
}

// GenerateTemplates discovers the standard template extensions and writes one
// Go file containing all generated declarations. It returns an empty path when
// no templates exist.
func (g *Generator) GenerateTemplates(dir, outDir, outName string) (string, error) {
	files, err := TemplateFiles(dir)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	pkg, err := packageName(dir)
	if err != nil {
		return "", err
	}
	if pkg == "" {
		for _, path := range files {
			source, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", readErr
			}
			declared := ""
			if strings.HasSuffix(path, ".tb.html") {
				module, parseErr := htmlbind.Parse(path, source)
				if parseErr != nil {
					return "", parseErr
				}
				if module.Package != nil {
					declared = module.Package.Name
				}
			} else {
				module, parseErr := templatesql.Parse(path, source)
				if parseErr != nil {
					return "", parseErr
				}
				if module.Package != nil {
					declared = module.Package.Name
				}
			}
			if declared != "" {
				if i := strings.LastIndex(declared, "."); i >= 0 {
					declared = declared[i+1:]
				}
				pkg = goTemplateIdentifier(declared)
				break
			}
		}
		if pkg == "" {
			pkg = "templates"
		}
	}
	var generated [][]byte
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var code []byte
		if strings.HasSuffix(path, ".tb.html") {
			module, parseErr := htmlbind.Parse(path, source)
			if parseErr != nil {
				return "", parseErr
			}
			if err := checkTemplatePackage(path, module.Package, pkg); err != nil {
				return "", err
			}
			code, err = htmlbind.Generate(path, source, htmlbind.GenerateOptions{Package: pkg})
		} else {
			module, parseErr := templatesql.Parse(path, source)
			if parseErr != nil {
				return "", parseErr
			}
			if err := checkTemplatePackage(path, module.Package, pkg); err != nil {
				return "", err
			}
			code, err = templatesql.Generate(path, source, templatesql.GenerateOptions{Package: pkg})
		}
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
			names := declarationNames(declaration)
			duplicate := ""
			allRuntime := len(names) > 0
			for _, name := range names {
				if seen[name] && !templateRuntimeName(name) {
					duplicate = name
					break
				}
				if !templateRuntimeName(name) {
					allRuntime = false
				}
			}
			if duplicate != "" {
				return nil, fmt.Errorf("duplicate generated template declaration %s", duplicate)
			}
			if allRuntime {
				skip := true
				for _, name := range names {
					if !seen[name] {
						skip = false
					}
				}
				if skip {
					continue
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
	out.WriteString("// Code generated by tinybind templates; DO NOT EDIT.\n\n")
	if err := format.Node(&out, fset, file); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return []byte(out.String()), nil
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
func templateRuntimeName(name string) bool {
	switch name {
	case "HTML", "TrustedHTML", "TrustedCSS", "TrustedJavaScript", "ScriptJSON", "Statement", "SQLExecer", "SQLQuerier", "_tinybindSQLBuilder":
		return true
	}
	return strings.HasPrefix(name, "_tinybindJSON") || name == "_tinybindWrite" || name == "_tinybindEscape" || name == "_tinybindBool" || name == "_tinybindInt" || name == "_tinybindFloat" || name == "_tinybindSQLArgs" || name == "_tinybindStatement" || name == "_tinybindSafeMutation"
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
