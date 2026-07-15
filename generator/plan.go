package generator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FieldSource is where a request field is read from.
type FieldSource string

const (
	SourceInput   FieldSource = "input"
	SourceQuery   FieldSource = "query"
	SourcePayload FieldSource = "payload"
	SourcePath    FieldSource = "path"
	SourceHeader  FieldSource = "header"
	SourceCookie  FieldSource = "cookie"
	SourceMethod  FieldSource = "method"
)

// FieldPlan is one struct field mapping plan (compile-time).
type FieldPlan struct {
	Name   string      // Go field name
	Wire   string      // wire / tag name
	Source FieldSource // input|query|payload|path|header|cookie|method
	Kind   string      // string|int|int64|bool|float64|file
	JSON   string      // json name for responses (default wire or lowercased)
	Check  CheckRules  // from check:"" tag; empty if absent
}

// httpbinderImportPath is the module path of this library (for recognizing File).
const httpbinderImportPath = "github.com/shibukawa/httpbind-go"

// TypePlan is the mapping plan for one struct type.
type TypePlan struct {
	Name   string
	Fields []FieldPlan
}

// PackagePlan is all type plans in a package.
type PackagePlan struct {
	Package string
	Types   []TypePlan
}

// AnalyzePackage builds field plans for all package-level structs with exported fields.
func AnalyzePackage(dir string) (*PackagePlan, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		name := info.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return false
		}
		// skip generated files when re-analyzing
		if strings.HasSuffix(name, "_httpbinder_gen.go") ||
			strings.HasSuffix(name, "_openapi_gen.go") ||
			name == "httpbinder_gen.go" ||
			name == "httpbinder_openapi_gen.go" {
			return false
		}
		return true
	}, 0)
	if err != nil {
		return nil, err
	}
	var pkg *ast.Package
	for name, p := range pkgs {
		if strings.HasSuffix(name, "_test") {
			continue
		}
		pkg = p
		break
	}
	if pkg == nil {
		return nil, fmt.Errorf("no package in %s", dir)
	}

	plan := &PackagePlan{Package: pkg.Name}
	for _, f := range pkg.Files {
		binderNames := httpbinderImportNames(f)
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				tp, ok, err := analyzeStruct(ts.Name.Name, st, binderNames)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", ts.Name.Name, err)
				}
				if ok {
					plan.Types = append(plan.Types, tp)
				}
			}
		}
	}
	return plan, nil
}

// httpbinderImportNames returns local identifiers that refer to this library
// (default name "httpbinder" or explicit/aliased imports).
func httpbinderImportNames(f *ast.File) map[string]bool {
	out := make(map[string]bool)
	if f == nil {
		return out
	}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != httpbinderImportPath {
			continue
		}
		if imp.Name != nil {
			switch imp.Name.Name {
			case "_":
				// side-effect import only
			case ".":
				// dot-import: File is bare Ident; handled separately if needed
			default:
				out[imp.Name.Name] = true
			}
			continue
		}
		// default: last path element is "httpbind-go" but the package name is httpbinder
		out["httpbinder"] = true
	}
	return out
}

func analyzeStruct(name string, st *ast.StructType, binderNames map[string]bool) (TypePlan, bool, error) {
	var fields []FieldPlan
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue // embedded
		}
		for _, id := range f.Names {
			if id == nil || !exported(id.Name) {
				continue
			}
			kind, ok := typeKind(f.Type, binderNames)
			if !ok {
				continue // unsupported field type for v1
			}
			src, wire := parseFieldTag(id.Name, f.Tag)
			if kind == "file" {
				// File is payload-only (multipart part); input defaults to payload.
				switch src {
				case SourceInput, SourcePayload:
					src = SourcePayload
				default:
					return TypePlan{}, false, fmt.Errorf("field %s: httpbinder.File only supports payload/input tags, got %s", id.Name, src)
				}
			}
			jsonName := wire
			if jsonName == "" {
				jsonName = lowerFirst(id.Name)
			}
			// optional json tag override for response encoding name
			if jt := tagValue(f.Tag, "json"); jt != "" && jt != "-" {
				jsonName = strings.Split(jt, ",")[0]
			}
			checkRaw := tagValue(f.Tag, "check")
			check, err := ParseCheckTag(checkRaw, kind)
			if err != nil {
				return TypePlan{}, false, fmt.Errorf("field %s: %w", id.Name, err)
			}
			fields = append(fields, FieldPlan{
				Name:   id.Name,
				Wire:   wire,
				Source: src,
				Kind:   kind,
				JSON:   jsonName,
				Check:  check,
			})
		}
	}
	if len(fields) == 0 {
		return TypePlan{}, false, nil
	}
	return TypePlan{Name: name, Fields: fields}, true, nil
}

func exported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

func typeKind(expr ast.Expr, binderNames map[string]bool) (string, bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string", "int", "int64", "bool", "float64":
			return t.Name, true
		}
	case *ast.SelectorExpr:
		if t.Sel != nil && t.Sel.Name == "File" {
			if pkg, ok := t.X.(*ast.Ident); ok && binderNames[pkg.Name] {
				return "file", true
			}
		}
		// time.Time etc. unsupported in v1
	}
	return "", false
}

func parseFieldTag(fieldName string, tag *ast.BasicLit) (FieldSource, string) {
	defaultWire := lowerFirst(fieldName)
	if tag == nil {
		return SourceInput, defaultWire
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return SourceInput, defaultWire
	}
	// priority: explicit source tags
	for _, src := range []FieldSource{SourceInput, SourceQuery, SourcePayload, SourcePath, SourceHeader, SourceCookie, SourceMethod} {
		if v := lookupTag(raw, string(src)); v != "" {
			if v == "-" {
				continue
			}
			return src, v
		}
	}
	// bare field with only json tags etc. → input
	return SourceInput, defaultWire
}

func tagValue(tag *ast.BasicLit, key string) string {
	if tag == nil {
		return ""
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return ""
	}
	return lookupTag(raw, key)
}

func lookupTag(raw, key string) string {
	// struct tag format: key:"value" key2:"value2"
	for _, part := range strings.Fields(raw) {
		k, v, ok := strings.Cut(part, ":")
		if !ok || k != key {
			continue
		}
		val, err := strconv.Unquote(v)
		if err != nil {
			return strings.Trim(v, `"`)
		}
		return val
	}
	return ""
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}
