package htmlbind

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// The property test behind rule:template-format-fidelity: every template in
// the module, under every width, both whitespace modes, and a few mutations of
// the source, formats to something that parses, settles on the second pass,
// keeps the normalized tree, and generates the same bytes. Each of the
// mutations is a way a file arrives that the checked-in fixtures never show.

var propertyLeadingWS = regexp.MustCompile(`(?m)^[ \t]+`)

var propertyMutations = []struct {
	name string
	fn   func([]byte) []byte
}{
	{"as-is", func(b []byte) []byte { return b }},
	{"deindent", func(b []byte) []byte { return propertyLeadingWS.ReplaceAll(b, nil) }},
	{"crlf", func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n")) }},
	{"blank-lines", func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\n"), []byte("\n\n")) }},
}

func propertyTemplates(t *testing.T) []string {
	t.Helper()
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
	return paths
}

func TestFormatProperties(t *testing.T) {
	t.Parallel()
	for _, path := range propertyTemplates(t) {
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range propertyMutations {
			source := m.fn(original)
			if _, err := Parse(path, source); err != nil {
				continue // the mutation broke the source, which is not the formatter's concern
			}
			for _, preserve := range []bool{false, true} {
				for _, width := range []int{30, 60, 100, 200} {
					variant := fmt.Sprintf("%s %s preserve=%v width=%d", filepath.Base(path), m.name, preserve, width)
					checkFormatProperties(t, variant, path, source, syntax.PrintOptions{Width: width, PreserveWhitespace: preserve})
				}
			}
		}
	}
}

func checkFormatProperties(t *testing.T, variant, path string, source []byte, options syntax.PrintOptions) {
	t.Helper()
	format := func(src []byte) ([]byte, error) {
		module, err := Parse(path, src)
		if err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}
		out, err := syntax.PrintModule(module, []syntax.RootPrinter{RootPrinter()}, options)
		if err != nil {
			return nil, fmt.Errorf("print: %w", err)
		}
		return []byte(out), nil
	}
	once, err := format(source)
	if err != nil {
		t.Errorf("%s: %v", variant, err)
		return
	}
	twice, err := format(once)
	if err != nil {
		t.Errorf("%s: formatted output does not parse: %v\n%s", variant, err, once)
		return
	}
	if !bytes.Equal(once, twice) {
		t.Errorf("%s: not idempotent\nfirst:\n%s\nsecond:\n%s", variant, once, twice)
	}
	collapse := !options.PreserveWhitespace
	before, err1 := normalizedASTWith(path, source, collapse)
	after, err2 := normalizedASTWith(path, once, collapse)
	if err1 != nil || err2 != nil {
		t.Errorf("%s: %v %v", variant, err1, err2)
	} else if before != after {
		t.Errorf("%s: normalized tree changed\nformatted:\n%s", variant, once)
	}
	generate := GenerateOptions{PreserveWhitespace: options.PreserveWhitespace}
	g1, e1 := Generate(path, source, generate)
	if e1 != nil {
		return // a fixture that needs context it does not have here
	}
	g2, e2 := Generate(path, once, generate)
	if e2 != nil {
		t.Errorf("%s: formatted output does not generate: %v\n%s", variant, e2, once)
		return
	}
	if !bytes.Equal(sourcePositionLine.ReplaceAll(g1, nil), sourcePositionLine.ReplaceAll(g2, nil)) {
		t.Errorf("%s: generated output changed\nformatted:\n%s", variant, once)
	}
}

// normalizedASTWith is normalizedAST with the collapse switch exposed: with
// collapse off the tree has to match byte for byte, whitespace included.
func normalizedASTWith(path string, source []byte, collapse bool) (string, error) {
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
		normalized, err := normalizeWhitespace(path, body, collapse)
		if err != nil {
			return "", err
		}
		template.Body = normalized
	}
	return marshalStripped(module)
}
