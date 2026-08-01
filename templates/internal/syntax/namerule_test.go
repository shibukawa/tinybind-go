package syntax_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/internal/rawparse"
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// The three policies of decision:declaration-name-policy, exercised through the
// dummy format parser so the test says nothing about HTML or SQL.

func parseWith(rule syntax.NameRule, source string) error {
	root := rawparse.Root("template", "raw:template", "raw")
	root.Names = rule
	_, err := syntax.ParseModule("x.raw", source, []syntax.RootDeclaration{root})
	return err
}

func TestNameRuleHTMLShapeRequiresPascalCase(t *testing.T) {
	rule := syntax.NameRule{PascalCase: true, ExportedNameIsGo: true}
	if err := parseWith(rule, "export template Card(): raw {x}"); err != nil {
		t.Fatalf("PascalCase export rejected: %v", err)
	}
	// A private component keeps the uppercase name, because the name is the
	// call syntax rather than a visibility marker.
	if err := parseWith(rule, "template Card(): raw {x}"); err != nil {
		t.Fatalf("PascalCase private rejected: %v", err)
	}
	err := parseWith(rule, "template card(): raw {x}")
	if err == nil || !strings.Contains(err.Error(), "PascalCase") {
		t.Fatalf("lowercase accepted or misreported: %v", err)
	}
}

func TestNameRuleSQLShapeAllowsLowercasePrivate(t *testing.T) {
	rule := syntax.NameRule{ExportedNameIsGo: true}
	for _, source := range []string{
		"template findUser(): raw {x}",        // private, lowercase
		"template FindUser(): raw {x}",        // private, uppercase: the builder is prefixed either way
		"export template FindUser(): raw {x}", // public, uppercase
	} {
		if err := parseWith(rule, source); err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
	err := parseWith(rule, "export template findUser(): raw {x}")
	if err == nil || !strings.Contains(err.Error(), "declared export but its name is unexported") {
		t.Fatalf("exported lowercase accepted or misreported: %v", err)
	}
}

func TestNameRuleDynamoShapeTiesCaseToExport(t *testing.T) {
	rule := syntax.NameRule{ExportedNameIsGo: true, PrivateNameIsGo: true}
	if err := parseWith(rule, "template readingsAround(): raw {x}"); err != nil {
		t.Fatalf("lowercase private rejected: %v", err)
	}
	if err := parseWith(rule, "export template ReadingsSince(): raw {x}"); err != nil {
		t.Fatalf("uppercase export rejected: %v", err)
	}
	err := parseWith(rule, "export template foo(): raw {x}")
	if err == nil || !strings.Contains(err.Error(), "declared export but its name is unexported") {
		t.Fatalf("exported lowercase accepted or misreported: %v", err)
	}
	// The mirror case is what a format needs when its private identifier is the
	// name too: without the keyword the function would still be public.
	err = parseWith(rule, "template Foo(): raw {x}")
	if err == nil || !strings.Contains(err.Error(), "has an exported name") {
		t.Fatalf("private uppercase accepted or misreported: %v", err)
	}
}

func TestNameRuleZeroValueConstrainsNothing(t *testing.T) {
	for _, source := range []string{
		"template foo(): raw {x}",
		"template Foo(): raw {x}",
		"export template Foo(): raw {x}",
		"export template foo(): raw {x}",
	} {
		if err := parseWith(syntax.NameRule{}, source); err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
}
