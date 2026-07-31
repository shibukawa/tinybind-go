package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// contextExternalSource exercises every expression position an external call can
// occupy, so each one is checked for the instruction that carries the context.
const contextExternalSource = `package pages

external Token(): string
external Field(): html
external Enabled(): bool
external Tags(): string[]

component Row(label: string): html {
<li>{label}</li>
}

export component Page(): html {
<form data-token="{Token()}" hidden="{Enabled()}">
{Field()}
<span>{Token()}</span>
{if Enabled()}<b>on</b>{/if}
<ul>
{for tag in Tags()}
  <Row label={Token()} />
{/for}
</ul>
</form>
}
`

func TestSyncExternalReceivesTheRenderContext(t *testing.T) {
	generated, err := htmlbind.Generate("ctx.pw.html", []byte(contextExternalSource), htmlbind.GenerateOptions{
		ContextExternals: map[string]bool{"Token": true, "Field": true, "Enabled": true, "Tags": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(generated)
	// One instruction per position, each taking the context and passing it on.
	for _, want := range []string{
		`"context"`,
		`AttrCtx("data-token", func(ctx context.Context, p PageParams) (string, bool)`,
		`BoolAttrCtx("hidden", func(ctx context.Context, p PageParams) bool { return Enabled(ctx) })`,
		`SlotCtx(func(ctx context.Context, p PageParams) htmlbind.Fragment { return Field(ctx) }, nil)`,
		`TextCtx(func(ctx context.Context, p PageParams) string { return Token(ctx) })`,
		`IfCtx(func(ctx context.Context, p PageParams) bool { return Enabled(ctx) }`,
		`htmlbind.ForCtx(`,
		`func(ctx context.Context, p PageParams) []string { return Tags(ctx) }`,
		`ComponentCtx(func(ctx context.Context, p`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated code is missing %q\n%s", want, got)
		}
	}
}

// TestExternalsWithoutContextAreUnchanged is the unused-is-free rule: the same
// template whose implementations take no context generates the instructions it
// generated before the context forms existed.
func TestExternalsWithoutContextAreUnchanged(t *testing.T) {
	generated, err := htmlbind.Generate("plain.pw.html", []byte(contextExternalSource), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := string(generated)
	if strings.Contains(got, "Ctx(") {
		t.Errorf("a template with no context-taking external emitted a context instruction\n%s", got)
	}
	if strings.Contains(got, `"context"`) {
		t.Errorf("a template with no context-taking external imported context\n%s", got)
	}
	for _, want := range []string{
		`Text(func(p PageParams) string { return Token() })`,
		`Slot(func(p PageParams) htmlbind.Fragment { return Field() }, nil)`,
		`htmlbind.For(`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated code is missing %q\n%s", want, got)
		}
	}
}

// TestPartialContextExternals covers the per-function choice: one external takes
// the context and the other does not, in the same template.
func TestPartialContextExternals(t *testing.T) {
	source := `package pages

external Token(): string
external Label(): string

export component Page(): html {
<span>{Token()}</span><span>{Label()}</span>
}
`
	generated, err := htmlbind.Generate("mixed.pw.html", []byte(source), htmlbind.GenerateOptions{
		ContextExternals: map[string]bool{"Token": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(generated)
	if !strings.Contains(got, `TextCtx(func(ctx context.Context, p PageParams) string { return Token(ctx) })`) {
		t.Errorf("the context-taking external did not receive it\n%s", got)
	}
	if !strings.Contains(got, `Text(func(p PageParams) string { return Label() })`) {
		t.Errorf("the plain external was changed\n%s", got)
	}
}

// TestAwaitBindingArgumentTakesContext covers an external called as an argument
// of an await binding, where the generated closure already holds the context.
func TestAwaitBindingArgumentTakesContext(t *testing.T) {
	source := `package pages

external Token(): string
external async LoadUser(id: string): string

export component Page(): html {
{await user = LoadUser(Token())}
<span>{user}</span>
{fallback}
<span>…</span>
{/await}
}
`
	generated, err := htmlbind.Generate("await.pw.html", []byte(source), htmlbind.GenerateOptions{
		ContextExternals: map[string]bool{"Token": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "LoadUser(Token(ctx))") {
		t.Errorf("an await binding argument did not receive the context\n%s", generated)
	}
}

// TestPendingBindingRejectsAContextExternal covers the one position with no
// context-carrying instruction: awaiting a value the caller started emits an
// unset check that runs before anything is written, so it may not call out.
func TestPendingBindingRejectsAContextExternal(t *testing.T) {
	source := `package pages

type Row {
  count: async int
}

external Rows(): Row[]
external Which(): int

export component Page(): html {
{await value = Rows()[Which()].count}
<span>{value}</span>
{fallback}
<span>…</span>
{/await}
}
`
	options := htmlbind.GenerateOptions{ContextExternals: map[string]bool{"Which": true}}
	_, err := htmlbind.Generate("pending.pw.html", []byte(source), options)
	if err == nil {
		t.Fatal("a context-taking external in a caller-supplied await binding was accepted")
	}
	if !strings.Contains(err.Error(), "render context") {
		t.Errorf("the diagnostic does not name the cause: %v", err)
	}
	// The same template compiles when the implementation takes no context, so
	// the rejection is about the context and not about the shape.
	if _, err := htmlbind.Generate("pending.pw.html", []byte(source), htmlbind.GenerateOptions{}); err != nil {
		t.Fatalf("the same binding without a context-taking external failed: %v", err)
	}
}
