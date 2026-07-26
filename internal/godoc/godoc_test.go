package godoc_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/shibukawa/tinybind-go/internal/godoc"
)

func TestText_FirstNonEmptyGroupWins(t *testing.T) {
	src := `package p

// Doc on the declaration.
type T struct{}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	decl := file.Decls[0].(*ast.GenDecl)
	spec := decl.Specs[0].(*ast.TypeSpec)
	// An ungrouped declaration carries the doc on the GenDecl, not the spec.
	if got := godoc.Text(spec.Doc, decl.Doc); got != "Doc on the declaration." {
		t.Fatalf("Text = %q", got)
	}
	if got := godoc.Text(nil, nil); got != "" {
		t.Fatalf("Text of no comments = %q", got)
	}
}

func TestSplit(t *testing.T) {
	cases := []struct {
		name        string
		doc         string
		summary     string
		description string
	}{
		{name: "empty"},
		{
			name:    "single sentence",
			doc:     "createUserHandler creates a user.",
			summary: "createUserHandler creates a user.",
		},
		{
			name:    "no punctuation",
			doc:     "reports service liveness",
			summary: "reports service liveness",
		},
		{
			name:        "sentence then paragraph",
			doc:         "searchHandler searches users. Only exact matches count.\n\nPaging starts at 1.",
			summary:     "searchHandler searches users.",
			description: "Only exact matches count.\n\nPaging starts at 1.",
		},
		{
			name:        "wrapped first line without period",
			doc:         "handles chat\n\nMore detail here.",
			summary:     "handles chat",
			description: "More detail here.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary, description := godoc.Split(tc.doc)
			if summary != tc.summary || description != tc.description {
				t.Fatalf("Split(%q) = (%q, %q), want (%q, %q)",
					tc.doc, summary, description, tc.summary, tc.description)
			}
		})
	}
}

func TestDeprecated(t *testing.T) {
	if !godoc.Deprecated("oldHandler serves v1.\n\nDeprecated: use newHandler.") {
		t.Fatal("deprecation paragraph not detected")
	}
	if godoc.Deprecated("newHandler serves v2.") {
		t.Fatal("unexpected deprecation")
	}
	// Only a paragraph-leading marker counts, matching godoc.
	if godoc.Deprecated("mentions Deprecated: inside a sentence") {
		t.Fatal("mid-sentence marker must not count")
	}
}
