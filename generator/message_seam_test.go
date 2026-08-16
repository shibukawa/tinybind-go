package generator_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// TestTemplateSeamsReachTheGeneratorPath is a coverage check rather than a
// behavior one. The same shape of seam once reached the templates path and not
// the route path, which shipped a feature that was simply absent on filesystem
// routes and was found downstream rather than here.
//
// See .knowledge requirement:route-package-context-externals.
func TestTemplateSeamsReachTheGeneratorPath(t *testing.T) {
	dir := t.TempDir()
	source := `package fixture
messages about

export component Hello(): html {<h1>{t title}</h1><p>{lang}</p>}`
	if err := os.WriteFile(filepath.Join(dir, "page.tb.html"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	options := generator.DefaultOptions()
	options.ImplicitBindings = []htmlbind.ImplicitBinding{{
		Name:     "lang",
		Provider: htmlbind.BindingProvider{Package: "example.com/app/framework", Name: "Lang"},
	}}
	options.Messages = map[string]htmlbind.MessageSymbol{
		"about.title": {Package: "example.com/app/messages", Name: "AboutTitle"},
	}
	options.MessageContextBinding = "lang"
	path, err := generator.New(options).GenerateTemplates(dir, dir, "")
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"messages.AboutTitle(framework.Lang(ctx))",
		"framework.Lang(ctx)",
		`"example.com/app/messages"`,
		`"example.com/app/framework"`,
	} {
		if !bytes.Contains(generated, []byte(want)) {
			t.Fatalf("the generator path did not carry the seam; output lacks %q:\n%s", want, generated)
		}
	}
}

// TestATemplateUsingAMessageWithNoSeamFails is the other half: without the
// table the reference is refused rather than emitted as an empty string, so a
// caller that forgot to fill the option learns at generation.
func TestATemplateUsingAMessageWithNoSeamFails(t *testing.T) {
	dir := t.TempDir()
	source := `package fixture
messages about

export component Hello(): html {<h1>{t title}</h1>}`
	if err := os.WriteFile(filepath.Join(dir, "page.tb.html"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := generator.New(generator.DefaultOptions()).GenerateTemplates(dir, dir, ""); err == nil {
		t.Fatal("a template naming an unresolved message generated without error")
	}
}
