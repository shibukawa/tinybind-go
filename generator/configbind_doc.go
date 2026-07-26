package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// docIndex holds normalized godoc text for config struct types and fields plus
// the AST needed to backfill missing help tags. go/types carries no comments, so
// the field objects are correlated back to their declarations here.
type docIndex struct {
	fset      *token.FileSet
	typeDoc   map[string]string
	fieldDoc  map[*types.Var]string
	fieldAST  map[*types.Var]*ast.Field
	fieldPath map[*types.Var]string
}

// helpBackfill is one planned help tag insertion on a config struct field.
type helpBackfill struct {
	path  string
	field *ast.Field
	help  string
}

func newDocIndex(fset *token.FileSet) *docIndex {
	return &docIndex{
		fset:      fset,
		typeDoc:   map[string]string{},
		fieldDoc:  map[*types.Var]string{},
		fieldAST:  map[*types.Var]*ast.Field{},
		fieldPath: map[*types.Var]string{},
	}
}

// buildConfigDocIndex reads doc comments from the package's hand-written files.
// Generated and test files are skipped so they are never scanned or rewritten.
func buildConfigDocIndex(pkg *packages.Package) *docIndex {
	index := newDocIndex(pkg.Fset)
	if pkg.TypesInfo == nil {
		return index
	}
	for _, file := range pkg.Syntax {
		if file == nil {
			continue
		}
		path := ""
		if pkg.Fset != nil {
			if handle := pkg.Fset.File(file.Pos()); handle != nil {
				path = handle.Name()
			}
		}
		if skipConfigSourceFile(filepath.Base(path)) || hasGeneratedHeader(file) {
			continue
		}
		collectFileDocs(index, pkg.TypesInfo, file, path)
	}
	return index
}

func collectFileDocs(index *docIndex, info *types.Info, file *ast.File, path string) {
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.GenDecl:
			if n.Tok != token.TYPE {
				return true
			}
			for _, spec := range n.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				doc := typeSpec.Doc
				if doc == nil && len(n.Specs) == 1 {
					doc = n.Doc
				}
				if text := normalizeDocComment(doc); text != "" {
					index.typeDoc[typeSpec.Name.Name] = text
				}
			}
		case *ast.StructType:
			if n.Fields == nil {
				return true
			}
			for _, field := range n.Fields.List {
				text := normalizeDocComment(field.Doc)
				if text == "" {
					text = normalizeDocComment(field.Comment)
				}
				for _, name := range field.Names {
					object, _ := info.Defs[name].(*types.Var)
					if object == nil {
						continue
					}
					index.fieldAST[object] = field
					index.fieldPath[object] = path
					if text != "" {
						index.fieldDoc[object] = text
					}
				}
			}
		}
		return true
	})
}

// skipConfigSourceFile reports whether a base file name is generated or a test.
func skipConfigSourceFile(base string) bool {
	if strings.HasSuffix(base, "_test.go") {
		return true
	}
	switch base {
	case defaultConfigBindOut, "tinybind_gen.go", "tinybind_openapi_gen.go":
		return true
	}
	return strings.HasSuffix(base, "_gen.go")
}

func hasGeneratedHeader(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() > file.Package {
			break
		}
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			if strings.HasPrefix(text, "Code generated ") && strings.HasSuffix(text, "DO NOT EDIT.") {
				return true
			}
		}
	}
	return false
}

var ignoredDocPrefixes = []string{"go:", "nolint", "lint:", "revive:", "nosec", "+build"}

// normalizeDocComment converts a doc comment into one single-line description:
// markers and directives are dropped, only the first paragraph is kept, runs of
// whitespace collapse, and one trailing period is removed. The leading Go
// identifier is deliberately kept so the text is never turned into a fragment.
func normalizeDocComment(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	var words []string
	for _, line := range strings.Split(group.Text(), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(words) > 0 {
				break
			}
			continue
		}
		if isIgnoredDocLine(trimmed) {
			continue
		}
		words = append(words, strings.Fields(trimmed)...)
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.Join(words, " "), "."))
}

func isIgnoredDocLine(line string) bool {
	for _, prefix := range ignoredDocPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// applyHelpBackfills inserts help tags into the declaring source files. Edits are
// applied from the end of each file so earlier offsets stay valid, and the result
// is gofmt-formatted before it is written. Returns the changed paths.
func applyHelpBackfills(fset *token.FileSet, edits []helpBackfill) ([]string, error) {
	if fset == nil || len(edits) == 0 {
		return nil, nil
	}
	byPath := map[string][]helpBackfill{}
	for _, edit := range edits {
		if edit.path == "" || edit.field == nil {
			continue
		}
		byPath[edit.path] = append(byPath[edit.path], edit)
	}
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var changed []string
	for _, path := range paths {
		updated, err := backfillFile(fset, path, byPath[path])
		if err != nil {
			return nil, err
		}
		if updated {
			changed = append(changed, path)
		}
	}
	return changed, nil
}

type tagInsertion struct {
	offset int
	text   string
}

func backfillFile(fset *token.FileSet, path string, edits []helpBackfill) (bool, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("configbind: help backfill %s: %w", path, err)
	}
	var insertions []tagInsertion
	for _, edit := range edits {
		position, text, ok := helpTagInsertion(edit.field, edit.help)
		if !ok {
			continue
		}
		offset := fset.Position(position).Offset
		if offset < 0 || offset > len(source) {
			return false, fmt.Errorf("configbind: help backfill %s: offset %d out of range", path, offset)
		}
		insertions = append(insertions, tagInsertion{offset: offset, text: text})
	}
	if len(insertions) == 0 {
		return false, nil
	}
	sort.Slice(insertions, func(i, j int) bool { return insertions[i].offset > insertions[j].offset })

	var updated bytes.Buffer
	rest := source
	for _, insertion := range insertions {
		updated.Reset()
		updated.Write(rest[:insertion.offset])
		updated.WriteString(insertion.text)
		updated.Write(rest[insertion.offset:])
		rest = append([]byte(nil), updated.Bytes()...)
	}
	formatted, err := format.Source(rest)
	if err != nil {
		return false, fmt.Errorf("configbind: help backfill %s: %w", path, err)
	}
	if bytes.Equal(formatted, source) {
		return false, nil
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, formatted, mode); err != nil {
		return false, fmt.Errorf("configbind: help backfill %s: %w", path, err)
	}
	return true, nil
}

// helpTagInsertion returns the position and literal text that adds help to a
// field tag. Existing tag keys keep their order; a field without a tag gains one.
func helpTagInsertion(field *ast.Field, help string) (token.Pos, string, bool) {
	if field == nil || help == "" || strings.Contains(help, "`") {
		return 0, "", false
	}
	entry := "help:" + strconv.Quote(help)
	if field.Tag == nil {
		if field.Type == nil {
			return 0, "", false
		}
		return field.Type.End(), " `" + entry + "`", true
	}
	value := field.Tag.Value
	if len(value) < 2 || !strings.HasPrefix(value, "`") || !strings.HasSuffix(value, "`") {
		return 0, "", false
	}
	if strings.TrimSpace(value[1:len(value)-1]) != "" {
		entry = " " + entry
	}
	return field.Tag.End() - 1, entry, true
}
