package rawparse_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/internal/rawparse"
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

func TestSharedParserFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "templates", "parser")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var cases []string
	for _, entry := range entries {
		if entry.IsDir() {
			cases = append(cases, entry.Name())
		}
	}
	sort.Strings(cases)
	if len(cases) == 0 {
		t.Fatal("no shared parser fixtures found")
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			inputPath := filepath.Join(dir, "input.txt")
			astPath := filepath.Join(dir, "ast.json")
			input, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatal(err)
			}
			module, err := rawparse.Parse(inputPath, input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.MarshalIndent(module, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(astPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(astPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("AST mismatch\n--- want ---\n%s--- got ---\n%s", want, got)
			}
		})
	}
}

func TestFormatSpecificRootRegistrationUsesDummyParser(t *testing.T) {
	module, err := syntax.ParseModule("sql.txt", `export statement Find(id: int): sql.one<Row> { SELECT {id} }`, []syntax.RootDeclaration{
		rawparse.Root("statement", "sql:statement", "sql"),
	})
	if err != nil {
		t.Fatal(err)
	}
	decl := module.Declarations[0].(*syntax.TemplateDecl)
	if decl.Kind != "sql:statement" || decl.Name != "Find" || !decl.Exported {
		t.Fatalf("declaration = %+v", decl)
	}
	body := decl.Body.([]syntax.Node)
	if body[0].NodeType() != "raw:text" || body[1].NodeType() != "template:expression" {
		t.Fatalf("body node types = %q, %q", body[0].NodeType(), body[1].NodeType())
	}
	expr := body[1].(*syntax.ExpressionNode)
	if expr.Context != "raw:text" {
		t.Fatalf("context = %q", expr.Context)
	}
}

func TestDummyParserDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"output mismatch", `template Bad(): html {text}`, "template declaration requires raw output"},
		{"missing if end", `template Bad(ok: bool): raw {{if ok}yes}`, "expected {else} or {/if}"},
		{"missing for end", `template Bad(items: string[]): raw {{for item in items}yes}`, "expected {/for}"},
		{"unknown root", `component Bad(): html {text}`, "unknown root declaration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rawparse.Parse("invalid.txt", []byte(tt.source))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
			var parseErr *syntax.ParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("error type = %T, want *syntax.ParseError", err)
			}
		})
	}
}
