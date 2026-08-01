package templatefmt_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
	"github.com/shibukawa/tinybind-go/templates/templatefmt"
)

// repoTemplates walks the module for every real template source, so the
// fidelity invariants are checked against files people actually wrote rather
// than against examples chosen to pass.
func repoTemplates(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".tb.html") || strings.HasSuffix(name, ".tb.sql") || strings.HasSuffix(name, ".tb.dynamo") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no template sources found")
	}
	return paths
}

// TestFormatIsIdempotent is the invariant that lets the tool run in CI: without
// it a diff never settles.
func TestFormatIsIdempotent(t *testing.T) {
	for _, path := range repoTemplates(t) {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		once, err := templatefmt.Source(path, source, templatefmt.Options{})
		if err != nil {
			t.Errorf("%s: format: %v", path, err)
			continue
		}
		twice, err := templatefmt.Source(path, once, templatefmt.Options{})
		if err != nil {
			t.Errorf("%s: reformat: %v", path, err)
			continue
		}
		if string(once) != string(twice) {
			t.Errorf("%s: not idempotent\nfirst:\n%s\nsecond:\n%s", path, once, twice)
		}
	}
}

// TestFormatPreservesAST checks that the tree the compiler reads is the tree it
// read before. HTML is checked in its own package instead, because there the
// invariant holds only after the compiler's whitespace normalization, which is
// not exported.
func TestFormatPreservesAST(t *testing.T) {
	for _, path := range repoTemplates(t) {
		if !strings.HasSuffix(path, ".tb.sql") {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		formatted, err := templatefmt.Source(path, source, templatefmt.Options{})
		if err != nil {
			t.Errorf("%s: format: %v", path, err)
			continue
		}
		before, err := astJSON(path, source)
		if err != nil {
			t.Errorf("%s: parse: %v", path, err)
			continue
		}
		after, err := astJSON(path, formatted)
		if err != nil {
			t.Errorf("%s: reparse: %v\n%s", path, err, formatted)
			continue
		}
		if before != after {
			t.Errorf("%s: AST changed\nformatted:\n%s", path, formatted)
		}
	}
}

// astJSON parses a source and serializes the tree, with positions removed: a
// formatter moves things, and where a node sits is exactly what it is allowed
// to change.
func astJSON(path string, source []byte) (string, error) {
	tree, err := sqlbind.Parse(path, source)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(tree)
	if err != nil {
		return "", err
	}
	var generic any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		return "", err
	}
	stripped, err := json.Marshal(stripVolatile(generic))
	if err != nil {
		return "", err
	}
	return string(stripped), nil
}

// stripVolatile removes what a formatter is allowed to move: source positions,
// and the comments whose own placement is the thing being formatted.
func stripVolatile(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			switch key {
			case "pos", "errorPos", "comments":
				continue
			}
			out[key] = stripVolatile(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = stripVolatile(item)
		}
		return out
	default:
		return value
	}
}
