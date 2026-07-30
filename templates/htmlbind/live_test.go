package htmlbind

import (
	"strings"
	"testing"
)

// compileLive generates from source and returns the error, which is what every
// rule below is stated in terms of.
func compileLive(t *testing.T, source string) error {
	t.Helper()
	_, err := Generate("page.tb.html", []byte(source), GenerateOptions{})
	return err
}

func wantCompileError(t *testing.T, source, fragment string) {
	t.Helper()
	err := compileLive(t, source)
	if err == nil {
		t.Fatalf("compiled without error, want one mentioning %q", fragment)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("error = %q, want it to mention %q", err.Error(), fragment)
	}
}

func TestLiveSourceBindsInAnOrdinaryAwaitClause(t *testing.T) {
	// There is no second clause keyword. How often a value arrives is what its
	// declaration says, not what the wait site asks for.
	if err := compileLive(t, `
external live WatchMetrics(id: string): string

export component Gauge(id: string): html {
{await point = WatchMetrics(id)}
  <p>{point}</p>
{fallback}
  <p>waiting</p>
{/await}
}
`); err != nil {
		t.Fatalf("a live source did not compile in an await clause: %v", err)
	}
}

func TestOneClauseMixesSettleOnceAndLiveBindings(t *testing.T) {
	// The settle-once binding delivers once and the live one keeps delivering;
	// every render reads both. Forbidding this was the only thing a separate
	// clause keyword bought, and it was not worth buying.
	if err := compileLive(t, `
external live WatchMetrics(id: string): string
external async LoadTitle(id: string): string

export component Gauge(id: string): html {
{await title = LoadTitle(id), point = WatchMetrics(id)}
  <p>{title}{point}</p>
{fallback}
  <p>waiting</p>
{/await}
}
`); err != nil {
		t.Fatalf("a clause mixing a settle-once and a live binding did not compile: %v", err)
	}
}

func TestAwaitClauseTakesSeveralLiveBindings(t *testing.T) {
	// Each binding writes its own scope field and any of them moving re-renders
	// the subtree, so nothing has to select between the sources.
	if err := compileLive(t, `
external live WatchMetrics(id: string): string
external live WatchRoom(id: string): string

export component Gauge(id: string): html {
{await point = WatchMetrics(id), room = WatchRoom(id)}
  <p>{point}{room}</p>
{fallback}
  <p>waiting</p>
{/await}
}
`); err != nil {
		t.Fatalf("a clause with two live bindings did not compile: %v", err)
	}
}

func TestPlainExternalCannotBeAwaited(t *testing.T) {
	wantCompileError(t, `
external LoadUser(id: string): string

export component Gauge(id: string): html {
{await user = LoadUser(id)}
  <p>{user}</p>
{fallback}
  <p>waiting</p>
{/await}
}
`, "external async or external live")
}

func TestLiveExternalCannotBeCalledOutsideAnAwaitBinding(t *testing.T) {
	wantCompileError(t, `
external live WatchMetrics(id: string): string

export component Gauge(id: string): html {
<p>{WatchMetrics(id)}</p>
}
`, "can only be called in an await binding")
}

func TestAwaitClauseRejectsDuplicateBindingNames(t *testing.T) {
	wantCompileError(t, `
external live WatchMetrics(id: string): string

export component Gauge(id: string): html {
{await point = WatchMetrics(id), point = WatchMetrics(id)}
  <p>{point}</p>
{fallback}
  <p>waiting</p>
{/await}
}
`, "duplicate await binding")
}

func TestExternalCannotBeBothAsyncAndLive(t *testing.T) {
	wantCompileError(t, `
external async live WatchMetrics(id: string): string

export component Gauge(id: string): html {
<p>{id}</p>
}
`, "cannot be both async and live")
}

func TestFormControlsAreRejectedInsideALiveBoundary(t *testing.T) {
	// A delivery replaces this subtree on the server's clock, so a control here
	// loses what the user typed with no warning and no user action behind it.
	// input is a void element, so each case carries its own markup rather than
	// being built from the tag name.
	for _, markup := range []string{"<input>", "<textarea></textarea>", "<select></select>", "<form></form>"} {
		t.Run(markup, func(t *testing.T) {
			wantCompileError(t, `
external live WatchMetrics(id: string): string

export component Gauge(id: string): html {
{await point = WatchMetrics(id)}
  <div>`+markup+`</div>
{fallback}
  <p>waiting</p>
{/await}
}
`, "cannot appear in a live boundary")
		})
	}
}

func TestFormControlsAreRejectedInAMixedBoundary(t *testing.T) {
	// One live binding makes the whole boundary re-render, so the rule follows
	// the boundary rather than the individual binding.
	wantCompileError(t, `
external live WatchMetrics(id: string): string
external async LoadTitle(id: string): string

export component Gauge(id: string): html {
{await title = LoadTitle(id), point = WatchMetrics(id)}
  <form><input></form>
  <p>{title}{point}</p>
{fallback}
  <p>waiting</p>
{/await}
}
`, "cannot appear in a live boundary")
}

func TestFormControlsAreAllowedInFallbackAndRecover(t *testing.T) {
	// Neither subtree is re-rendered by a delivery, so a control in one is as
	// safe as a control outside the boundary.
	if err := compileLive(t, `
external live WatchMetrics(id: string): string

export component Gauge(id: string): html {
{await point = WatchMetrics(id)}
  <p>{point}</p>
{fallback}
  <form><input></form>
{recover err}
  <form><input></form>
{/await}
}
`); err != nil {
		t.Fatalf("a control outside the replaced subtree was rejected: %v", err)
	}
}

func TestFormControlsAreStillAllowedInASettleOnceBoundary(t *testing.T) {
	// A boundary with no live binding settles once, on a wait the page opened
	// deliberately. The rule is about repetition on the server's clock, not
	// about boundaries.
	if err := compileLive(t, `
external async LoadUser(id: string): string

export component Gauge(id: string): html {
{await user = LoadUser(id)}
  <form><input></form>
  <p>{user}</p>
{fallback}
  <p>waiting</p>
{/await}
}
`); err != nil {
		t.Fatalf("a settle-once boundary rejected a control: %v", err)
	}
}
