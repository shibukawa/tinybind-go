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

	"github.com/shibukawa/tinybind-go/internal/godoc"
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
				// A grouped declaration's doc describes the group, not one spec.
				var declDoc *ast.CommentGroup
				if len(n.Specs) == 1 {
					declDoc = n.Doc
				}
				if text := normalizeDocComment(typeSpec.Doc, declDoc); text != "" {
					index.typeDoc[typeSpec.Name.Name] = text
				}
			}
		case *ast.StructType:
			if n.Fields == nil {
				return true
			}
			for _, field := range n.Fields.List {
				text := normalizeDocComment(field.Doc, field.Comment)
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

// normalizeDocComment reduces a doc comment to one single-line description. It
// shares godoc.Text with OpenAPI extraction but diverges afterwards: a help tag
// and a scaffold comment must fit on one line, so only the first paragraph is
// kept, whitespace collapses, and one trailing period is removed. The leading Go
// identifier stays so the text is never turned into a sentence fragment.
//
// Groups are tried in order, matching godoc.Text: pass field.Doc then
// field.Comment to prefer a doc comment over a trailing line comment.
func normalizeDocComment(groups ...*ast.CommentGroup) string {
	var words []string
	for _, line := range strings.Split(godoc.Text(groups...), "\n") {
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
	if err := replaceSource(path, formatted, mode); err != nil {
		return false, fmt.Errorf("configbind: help backfill %s: %w", path, err)
	}
	return true, nil
}

// replaceSource writes content over path with no state in between in which path
// is neither the old file nor the new one.
//
// The backfill is the one write generation performs into a hand-written source,
// and a caller generating directories at once may be type-checking this very
// file in another goroutine while it happens, as an import of the package it is
// generating. A truncating write leaves a window in which that reader sees an
// empty or half-written file and reports a parse error against a source nobody
// edited. Written beside the file and renamed over it there is no such window:
// a reader gets the old bytes or the new ones. It is the shape the conversion
// cache already uses, for the same reason.
//
// The temporary name begins with a dot so the Go tool ignores it, which matters
// because it is created in a directory something may be listing.
func replaceSource(path string, content []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		os.Remove(name)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	// The mode is set on the temporary file rather than after the rename, so
	// the file that appears at path already has it.
	if err := os.Chmod(name, mode); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
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
