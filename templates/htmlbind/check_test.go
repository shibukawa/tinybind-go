package htmlbind_test

import (
	"strings"
	"testing"

	htmlbind "github.com/shibukawa/tinybind-go/templates/htmlbind"
)

const checkHead = "package pages\n\n" +
	"type Record {\n  title: string\n  summary: string\n}\n\n" +
	"external LoadData(id: string): Record\n" +
	"external async LoadSlow(id: string): Record\n" +
	"external Authorize(id: string)\n" +
	"external Norm(s: string): string\n\n"

func checkSource(body string) string {
	return checkHead + "export component Card(id: string): html {\n" + body + "\n}\n"
}

// A check is a call made for its error alone. It binds nothing, so it lowers to
// the one instruction that writes nothing and can still end the render.
func TestCheckLowersToRequire(t *testing.T) {
	generated := generateWith(t, checkSource("{check Authorize(id)}\n<h1>ok</h1>"), htmlbind.GenerateOptions{})
	want := "planCardOps.Require(func(p CardParams) error { return Authorize(p.Id) })"
	if !strings.Contains(generated, want) {
		t.Fatalf("generated code is missing %q:\n%s", want, generated)
	}
}

// The instruction runs where the directive was hoisted to, which is the top of
// the block whatever the author wrote after it. That is what leaves the response
// status free: nothing has been written when the check refuses.
func TestCheckRunsBeforeTheMarkupItGuards(t *testing.T) {
	generated := generateWith(t, checkSource("<section>\n<h1>ok</h1>\n{check Authorize(id)}\n</section>"), htmlbind.GenerateOptions{})
	require := strings.Index(generated, ".Require(")
	static := strings.Index(generated, "<section")
	if require < 0 || static < 0 {
		t.Fatalf("generated code lacks the check or the markup:\n%s", generated)
	}
	if require > static {
		t.Fatalf("the check is emitted after the markup it guards:\n%s", generated)
	}
}

// A check written after a binding lands inside that binding's subtree, so it can
// read the name. Written before it, it runs first. Source order decides, exactly
// as it does between two bindings.
func TestCheckReadsABindingWrittenBeforeIt(t *testing.T) {
	generated := generateWith(t, checkSource("{val record = LoadData(id)}\n{check Authorize(record.title)}\n<h1>{record.title}</h1>"), htmlbind.GenerateOptions{})
	want := "Require(func(p planCardOpsVal1) error { return Authorize(p.Record.Title) })"
	if !strings.Contains(generated, want) {
		t.Fatalf("generated code is missing %q:\n%s", want, generated)
	}
}

// A binding read by nothing but a check is read. Without counting the check as a
// reader the author would be told to remove the loader the check exists to
// inspect.
func TestABindingReadOnlyByACheckIsRead(t *testing.T) {
	generateWith(t, checkSource("{val record = LoadData(id)}\n{check Authorize(record.title)}\n<h1>ok</h1>"), htmlbind.GenerateOptions{})
}

// The context-carrying variant is chosen the same way every other instruction
// chooses it: from the Go signature the scan reported.
func TestCheckTakesTheRenderContext(t *testing.T) {
	generated := generateWith(t, checkSource("{check Authorize(id)}\n<h1>ok</h1>"), htmlbind.GenerateOptions{
		ContextExternals: map[string]bool{"Authorize": true},
		ErrorExternals:   map[string]bool{"Authorize": true},
	})
	want := "planCardOps.RequireCtx(func(ctx context.Context, p CardParams) error { return Authorize(ctx, p.Id) })"
	if !strings.Contains(generated, want) {
		t.Fatalf("generated code is missing %q:\n%s", want, generated)
	}
}

// A checked call that also returns a value is asked only whether it failed, so
// the value it came with is dropped at the call site.
func TestCheckDiscardsADeclaredResult(t *testing.T) {
	generated := generateWith(t, checkSource("{check LoadData(id)}\n<h1>ok</h1>"), htmlbind.GenerateOptions{
		ErrorExternals: map[string]bool{"LoadData": true},
	})
	want := "planCardOps.Require(func(p CardParams) error { _, err := LoadData(p.Id); return err })"
	if !strings.Contains(generated, want) {
		t.Fatalf("generated code is missing %q:\n%s", want, generated)
	}
}

// A declaration with no result type is not a value, and a check directive is the
// only position it has. Everywhere else wants something to render, compare, or
// pass on, and this call has nothing to give.
func TestValueLessExternalIsRefusedInEveryOtherPosition(t *testing.T) {
	for _, body := range []string{
		"<h1>{Authorize(id)}</h1>",
		"{val ok = Authorize(id)}\n<h1>{ok}</h1>",
		"<h1 data-x={Authorize(id)}>x</h1>",
		"{if Authorize(id)}\n<h1>x</h1>\n{/if}",
		"<h1>{Norm(Authorize(id))}</h1>",
	} {
		message := generateError(t, checkSource(body), htmlbind.GenerateOptions{})
		if !strings.Contains(message, "Authorize declares no result") {
			t.Fatalf("%s: want the no-result diagnostic, got %q", body, message)
		}
		if !strings.Contains(message, "{check Authorize(...)}") {
			t.Fatalf("%s: the diagnostic does not say what to write instead: %q", body, message)
		}
	}
}

// One call per directive. A comma list buys several names on one line, and a
// check binds no name to share the line with.
func TestCheckTakesOneCall(t *testing.T) {
	message := generateError(t, checkSource("{check Authorize(id), Authorize(id)}\n<h1>x</h1>"), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "check takes one call") {
		t.Fatalf("want the one-call diagnostic, got %q", message)
	}
	if !strings.Contains(message, "write a second {check}") {
		t.Fatalf("the diagnostic does not say what to do instead: %q", message)
	}
}

// The position wants a call whatever the callee turns out to be: a name or a
// field path has no error to check.
func TestCheckWantsACall(t *testing.T) {
	message := generateError(t, checkSource("{check id}\n<h1>x</h1>"), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "check syntax is {check Name(...)}") {
		t.Fatalf("want the call-shape diagnostic, got %q", message)
	}
}

// An attribute value has no later siblings and no block, so a check is refused
// there as the block it is rather than read as a bare value.
func TestCheckIsRefusedInAnAttribute(t *testing.T) {
	message := generateError(t, checkSource(`<h1 data-x="{check Authorize(id)}">x</h1>`), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "control blocks are forbidden in attributes") {
		t.Fatalf("want the attribute diagnostic a val binding gets, got %q", message)
	}
}

// An async external stays await-only. A check runs before anything is written,
// which is precisely what a boundary cannot promise.
func TestCheckRefusesAnAsyncExternal(t *testing.T) {
	message := generateError(t, checkSource("{check LoadSlow(id)}\n<h1>x</h1>"), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "LoadSlow is async") {
		t.Fatalf("want the async diagnostic, got %q", message)
	}
}

// A call with no result and no error has no outcome anything can observe. It
// would emit Go that does not compile, so it is refused where the template line
// can be named.
func TestCheckRefusesACallThatCannotFail(t *testing.T) {
	message := generateError(t, checkSource("{check Authorize(id)}\n<h1>x</h1>"), htmlbind.GenerateOptions{
		ErrorExternals: map[string]bool{"LoadData": true},
	})
	if !strings.Contains(message, "returns nothing at all") {
		t.Fatalf("want the no-outcome diagnostic, got %q", message)
	}
}

// The other way to arrive at a call that cannot fail: one that answers a value
// and only a value. That is a binding, and the diagnostic says so rather than
// leaving a discarded result to the Go compiler.
func TestCheckRefusesAValueThatCannotFail(t *testing.T) {
	message := generateError(t, checkSource("{check LoadData(id)}\n<h1>x</h1>"), htmlbind.GenerateOptions{
		ErrorExternals: map[string]bool{},
	})
	if !strings.Contains(message, "returns a value and no error") {
		t.Fatalf("want the total-call diagnostic, got %q", message)
	}
	if !strings.Contains(message, "{val name = LoadData(...)}") {
		t.Fatalf("the diagnostic does not say what to write instead: %q", message)
	}
}

// A result type is what an async external's boundary hands to its subtree, so
// there is nothing for the value-less form to mean there.
func TestAsyncExternalMustDeclareAResult(t *testing.T) {
	source := "package pages\n\nexternal async Authorize(id: string)\n\n" +
		"export component Card(id: string): html {\n<h1>x</h1>\n}\n"
	message := generateError(t, source, htmlbind.GenerateOptions{})
	if !strings.Contains(message, "must declare a result type") {
		t.Fatalf("want the async declaration diagnostic, got %q", message)
	}
}

// A check hoists in front of the block's markup, which puts it where the root
// element used to be. The boundary scan has to see through it for the same
// reason it sees through a value binding: a component that guards itself must
// not silently stop being an update boundary.
func TestACheckLeavesTheComponentItsBoundaryRoot(t *testing.T) {
	plain := generateWith(t, checkSource("<section><h1>{id}</h1></section>"), htmlbind.GenerateOptions{})
	if !strings.Contains(plain, "BoundaryAttr()") {
		t.Fatalf("the component is not a boundary before a check is added, so this test proves nothing:\n%s", plain)
	}
	guarded := generateWith(t, checkSource("<section>{check Authorize(id)}<h1>{id}</h1></section>"), htmlbind.GenerateOptions{})
	if !strings.Contains(guarded, "BoundaryAttr()") {
		t.Fatalf("the check cost the component its boundary root:\n%s", guarded)
	}
	// Transparent, not skippable: a second root element beside the check is
	// still two roots.
	two := generateWith(t, checkSource("{check Authorize(id)}<section></section><aside></aside>"), htmlbind.GenerateOptions{})
	if strings.Contains(two, "BoundaryAttr()") {
		t.Fatalf("two root elements became a boundary:\n%s", two)
	}
}

// A template writing no check generates exactly what it generated before the
// directive existed.
func TestUnusedIsFree(t *testing.T) {
	generated := generateWith(t, checkSource("<h1>{LoadData(id).title}</h1>"), htmlbind.GenerateOptions{})
	if strings.Contains(generated, "Require") {
		t.Fatalf("a template with no check emitted one:\n%s", generated)
	}
}
