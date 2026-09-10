package sqlbind_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// The property test behind rule:template-format-fidelity for SQL: every
// template in the module, under every width and a few mutations of the source,
// formats to something that parses, settles on the second pass, and keeps the
// body's token stream. The mutations put comments and whitespace where the
// checked-in fixtures never do.

func propertyTemplates(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".tb.sql") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no SQL templates found")
	}
	return paths
}

func huntFormat(path string, source []byte, opts syntax.PrintOptions) ([]byte, error) {
	module, err := sqlbind.Parse(path, source)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	out, err := syntax.PrintModule(module, []syntax.RootPrinter{sqlbind.RootPrinter()}, opts)
	if err != nil {
		return nil, fmt.Errorf("print: %w", err)
	}
	return []byte(out), nil
}

func strip(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			switch key {
			case "pos", "errorPos", "comments":
				continue
			}
			out[key] = strip(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = strip(item)
		}
		return out
	default:
		return value
	}
}

func huntAST(path string, source []byte) (string, error) {
	tree, err := sqlbind.Parse(path, source)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, decl := range tree.Declarations {
		t, ok := decl.(*syntax.TemplateDecl)
		if !ok {
			encoded, _ := json.Marshal(decl)
			var generic any
			_ = json.Unmarshal(encoded, &generic)
			out, _ := json.Marshal(strip(generic))
			b.Write(out)
			b.WriteString("\n")
			continue
		}
		b.WriteString("== " + t.Name + "\n")
		body, _ := t.Body.([]syntax.Node)
		if body == nil {
			if bb, ok := t.Body.(sqlbind.Body); ok {
				body = bb
			}
		}
		bodyTokens(&b, body)
	}
	return b.String(), nil
}

func bodyTokens(b *strings.Builder, nodes []syntax.Node) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *sqlbind.TextNode:
			b.WriteString(strings.ReplaceAll(tokens([]byte(n.Text)), "\x00", "\n"))
		case *syntax.IfNode:
			open, _ := syntax.ControlOpen(n)
			b.WriteString("{" + open + "}\n")
			bodyTokens(b, n.Then)
			b.WriteString("{else}\n")
			bodyTokens(b, n.Else)
			b.WriteString("{/if}\n")
		case *syntax.ForNode:
			open, _ := syntax.ControlOpen(n)
			b.WriteString("{" + open + "}\n")
			bodyTokens(b, n.Body)
			b.WriteString("{/for}\n")
		default:
			encoded, _ := json.Marshal(node)
			var generic any
			_ = json.Unmarshal(encoded, &generic)
			out, _ := json.Marshal(strip(generic))
			b.Write(out)
			b.WriteString("\n")
		}
	}
}

var (
	leadingWS = regexp.MustCompile(`(?m)^[ \t]+`)
	bodyLine  = regexp.MustCompile(`(?m)^(  \S.*)$`)
)

// tokens is the rule's sql_token_identity: the body rescanned, whitespace
// dropped, literals and comments whole.
func tokens(source []byte) string {
	s := string(source)
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '-' && i+1 < len(s) && s[i+1] == '-':
			e := strings.IndexByte(s[i:], '\n')
			if e < 0 {
				e = len(s) - i
			}
			b.WriteString(strings.TrimRight(s[i:i+e], " \t") + "\x00")
			i += e
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			e := strings.Index(s[i:], "*/")
			if e < 0 {
				e = len(s) - i - 2
			}
			b.WriteString(s[i:i+e+2] + "\x00")
			i += e + 2
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for j < len(s) && s[j] != c {
				j++
			}
			if j < len(s) {
				j++
			}
			b.WriteString(s[i:j] + "\x00")
			i = j
		case c == '(' || c == ')' || c == ',' || c == ';':
			b.WriteString(string(c) + "\x00")
			i++
		default:
			j := i
			for j < len(s) && !strings.ContainsRune(" \t\n\r'\"`(),;", rune(s[j])) && !(s[j] == '-' && j+1 < len(s) && s[j+1] == '-') && !(s[j] == '/' && j+1 < len(s) && s[j+1] == '*') {
				j++
			}
			if j == i {
				j++
			}
			b.WriteString(s[i:j] + "\x00")
			i = j
		}
	}
	return b.String()
}

func TestFormatProperties(t *testing.T) {
	t.Parallel()
	report := func(kind, path, variant string, detail string) {
		t.Errorf("[%s] %s (%s)\n%s", kind, path, variant, detail)
	}
	mutations := []struct {
		name string
		fn   func([]byte) []byte
	}{
		{"as-is", func(b []byte) []byte { return b }},
		{"deindent", func(b []byte) []byte { return leadingWS.ReplaceAll(b, nil) }},
		{"line-comment-per-body-line", func(b []byte) []byte { return bodyLine.ReplaceAll(b, []byte("$1 -- c")) }},
		{"block-comment-after-comma", func(b []byte) []byte { return bytes.ReplaceAll(b, []byte(","), []byte(", /* b */")) }},
		{"line-comment-after-comma", func(b []byte) []byte { return bytes.ReplaceAll(b, []byte(",\n"), []byte(", -- c\n")) }},
		{"blank-lines", func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\n"), []byte("\n\n")) }},
		{"body-on-one-line", func(b []byte) []byte {
			return regexp.MustCompile(`(?m)\n  (\S)`).ReplaceAll(b, []byte(" $1"))
		}},
	}
	for _, path := range propertyTemplates(t) {
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range mutations {
			source := m.fn(original)
			if _, err := sqlbind.Parse(path, source); err != nil {
				continue
			}
			for _, width := range []int{30, 60, 100, 200} {
				variant := fmt.Sprintf("%s width=%d", m.name, width)
				opts := syntax.PrintOptions{Width: width}
				once, err := huntFormat(path, source, opts)
				if err != nil {
					report("format-error", path, variant, err.Error())
					continue
				}
				twice, err := huntFormat(path, once, opts)
				if err != nil {
					report("reparse-error", path, variant, err.Error()+"\n"+string(once))
					continue
				}
				if !bytes.Equal(once, twice) {
					report("not-idempotent", path, variant, firstDiff(once, twice))
				}
				before, _ := huntAST(path, source)
				after, _ := huntAST(path, once)
				if before != after {
					report("tokens-changed", path, variant, firstDiff([]byte(before), []byte(after))+"\n--- formatted\n"+string(once))
				}
			}
		}
	}
}

func firstDiff(a, b []byte) string {
	la, lb := bytes.Split(a, []byte("\n")), bytes.Split(b, []byte("\n"))
	for i := 0; i < len(la) && i < len(lb); i++ {
		if !bytes.Equal(la[i], lb[i]) {
			lo := i - 2
			if lo < 0 {
				lo = 0
			}
			hi := i + 3
			ctx := func(l [][]byte) string {
				h := hi
				if h > len(l) {
					h = len(l)
				}
				return string(bytes.Join(l[lo:h], []byte("\n")))
			}
			return fmt.Sprintf("line %d\n--- a\n%s\n--- b\n%s", i+1, ctx(la), ctx(lb))
		}
	}
	return fmt.Sprintf("lengths %d vs %d", len(la), len(lb))
}
