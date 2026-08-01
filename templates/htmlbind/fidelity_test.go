package htmlbind

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// The HTML fidelity invariant from rule:template-format-fidelity: formatting
// may reshape a whitespace run, so the trees are compared after the compiler's
// own normalization rather than before it. Two sources that normalize to the
// same tree generate the same bytes, which is what the rule actually requires.

func normalizedAST(path string, source []byte) (string, error) {
	module, err := Parse(path, source)
	if err != nil {
		return "", err
	}
	for _, decl := range module.Declarations {
		template, ok := decl.(*syntax.TemplateDecl)
		if !ok {
			continue
		}
		body, ok := template.Body.([]syntax.Node)
		if !ok {
			continue
		}
		normalized, err := normalizeWhitespace(path, body, true)
		if err != nil {
			return "", err
		}
		template.Body = normalized
	}
	encoded, err := json.Marshal(module)
	if err != nil {
		return "", err
	}
	var generic any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		return "", err
	}
	stripped, err := json.Marshal(stripPositions(generic))
	if err != nil {
		return "", err
	}
	return string(stripped), nil
}

// stripPositions removes what a formatter is allowed to move.
func stripPositions(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			switch key {
			case "pos", "errorPos", "comments":
				continue
			}
			out[key] = stripPositions(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = stripPositions(item)
		}
		return out
	default:
		return value
	}
}

func formatBytes(t *testing.T, path string, source []byte) []byte {
	t.Helper()
	module, err := Parse(path, source)
	if err != nil {
		t.Fatalf("%s: parse: %v", path, err)
	}
	out, err := syntax.PrintModule(module, []syntax.RootPrinter{RootPrinter()}, syntax.PrintOptions{})
	if err != nil {
		t.Fatalf("%s: print: %v", path, err)
	}
	return []byte(out)
}

func TestFormatKeepsGeneratedMarkup(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".tb.html") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no HTML templates found")
	}
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before, err := normalizedAST(path, source)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		formatted := formatBytes(t, path, source)
		after, err := normalizedAST(path, formatted)
		if err != nil {
			t.Errorf("%s: reparse: %v\n%s", path, err, formatted)
			continue
		}
		if before != after {
			t.Errorf("%s: normalized tree changed\nformatted:\n%s", path, formatted)
		}
	}
}
