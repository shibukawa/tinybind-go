package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

func holeOptions() htmlbind.GenerateOptions {
	options := messageOptions()
	options.Messages["about.agree"] = htmlbind.MessageSymbol{
		Package: "example.com/app/messages", Name: "AboutAgree",
	}
	options.Messages["about.links"] = htmlbind.MessageSymbol{
		Package: "example.com/app/messages", Name: "AboutLinks",
	}
	return options
}

// TestHoleNameComesFromTheTag is the chosen spelling: a translation writes
// <a>…</a> and a template writes <a href="/start"></a>, and the two line up
// with nothing in between to explain.
//
// See .knowledge requirement:message-hole-binding.
func TestHoleNameComesFromTheTag(t *testing.T) {
	source := "messages about\n\ncomponent Page(): html {<p>{t agree}<a href=\"/start\"></a>{/t}</p>}"
	out, err := htmlbind.Generate("h.txt", []byte(source), holeOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	rendered := string(out)
	for _, want := range []string{
		`{Name: "a", Ops:`,
		`planPageOps.MessageInner()`,
		`messages.AboutAgree(framework.Locale(ctx))`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("output lacks %q:\n%s", want, rendered)
		}
	}
}

// TestTwoHolesSharingATagNeedTheAttribute is the one case the tag-name rule
// cannot answer, and the escape hatch it falls back to.
func TestTwoHolesSharingATagNeedTheAttribute(t *testing.T) {
	clashing := "messages about\n\ncomponent Page(): html {<p>{t links}<a href=\"/x\"></a><a href=\"/y\"></a>{/t}</p>}"
	if _, err := htmlbind.Generate("h.txt", []byte(clashing), holeOptions()); err == nil {
		t.Fatal("two holes sharing a tag were accepted with no way to tell them apart")
	} else if !strings.Contains(err.Error(), "duplicate message hole a") {
		t.Fatalf("error = %v, want the clash named", err)
	}

	disambiguated := "messages about\n\ncomponent Page(): html {<p>{t links}<a href=\"/x\" hole=\"first\"></a><a href=\"/y\" hole=\"second\"></a>{/t}</p>}"
	out, err := htmlbind.Generate("h.txt", []byte(disambiguated), holeOptions())
	if err != nil {
		t.Fatalf("the attribute did not resolve the clash: %v", err)
	}
	for _, want := range []string{`{Name: "first", Ops:`, `{Name: "second", Ops:`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("output lacks %q:\n%s", want, out)
		}
	}
}

// TestTextInsideABlockBelongsToTheTranslation keeps the boundary: a sentence
// written in the template would be a second copy of what the catalog holds.
func TestTextInsideABlockBelongsToTheTranslation(t *testing.T) {
	source := "messages about\n\ncomponent Page(): html {<p>{t agree}please <a href=\"/x\"></a>{/t}</p>}"
	_, err := htmlbind.Generate("h.txt", []byte(source), holeOptions())
	if err == nil {
		t.Fatal("literal text inside a message block was accepted")
	}
	if !strings.Contains(err.Error(), "belongs to the translation") {
		t.Fatalf("error = %v, want the ownership stated", err)
	}
}

// TestABlockBindingNoHoleIsAnError points an author at the plain form.
func TestABlockBindingNoHoleIsAnError(t *testing.T) {
	source := "messages about\n\ncomponent Page(): html {<p>{t agree}{/t}</p>}"
	_, err := htmlbind.Generate("h.txt", []byte(source), holeOptions())
	if err == nil {
		t.Fatal("a block binding no hole was accepted")
	}
	if !strings.Contains(err.Error(), "binds no hole") {
		t.Fatalf("error = %v, want it to point at the plain form", err)
	}
}

// TestACloserWithNoReferenceIsAnError covers the discovered-at-the-closer rule
// from the other side.
func TestACloserWithNoReferenceIsAnError(t *testing.T) {
	_, err := htmlbind.Parse("h.txt", []byte("component Page(): html {<p>x{/t}</p>}"))
	if err == nil {
		t.Fatal("a stray closer parsed")
	}
	if !strings.Contains(err.Error(), "no {t ...} reference opens one") {
		t.Fatalf("error = %v, want the missing opener named", err)
	}
}

// TestThePlainFormIsUnaffected keeps the two forms separate: a reference with
// no block is still a string expression.
func TestThePlainFormIsUnaffected(t *testing.T) {
	source := "messages about\n\ncomponent Page(): html {<p>{t title}</p>}"
	out, err := htmlbind.Generate("h.txt", []byte(source), holeOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if strings.Contains(string(out), "MessageHoleOps") {
		t.Fatalf("the plain form emitted hole machinery:\n%s", out)
	}
}

// TestAMessageBlockPrintsBackAsWritten keeps requirement:template-source-formatting
// honest for the block form. Without this the formatter refuses a template
// carrying rich text, which is a failure an author meets on their first save
// rather than at generation.
func TestAMessageBlockPrintsBackAsWritten(t *testing.T) {
	sources := []string{
		"messages terms\n\ncomponent Page(): html {\n  <p>{t agree}<a href=\"/start\"></a>{/t}</p>\n}\n",
		"messages terms\n\ncomponent Page(): html {\n  <p>{t links}<a href=\"/x\" hole=\"first\"></a><a href=\"/y\" hole=\"second\"></a>{/t}</p>\n}\n",
	}
	for _, source := range sources {
		module, err := htmlbind.Parse("p.txt", []byte(source))
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		printed, err := syntax.PrintModule(module, []syntax.RootPrinter{htmlbind.RootPrinter()}, syntax.PrintOptions{})
		if err != nil {
			t.Fatalf("the formatter refused a message block: %v", err)
		}
		if printed != source {
			t.Fatalf("printing changed the source:\nwant %q\ngot  %q", source, printed)
		}
		// The printed form has to parse back to the same shape, or a save would
		// quietly rewrite what the author meant.
		if _, err := htmlbind.Parse("p.txt", []byte(printed)); err != nil {
			t.Fatalf("printed output does not parse: %v", err)
		}
	}
}
