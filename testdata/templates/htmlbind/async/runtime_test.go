package pages

import (
	"bytes"
	"context"
	"errors"
	"io"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// The generated code calls these; a real package would implement them against
// a database or an HTTP client.
var (
	userDelay time.Duration
	userError error
	tagsError error
)

// An async external is an ordinary blocking function. The runtime runs it in a
// goroutine and joins the results, so nothing here knows about concurrency.
func LoadUser(id string) (User, error) {
	if userDelay > 0 {
		time.Sleep(userDelay)
	}
	if userError != nil {
		return User{}, userError
	}
	return User{Name: "user-" + id, Admin: true}, nil
}

func LoadTags(id string) ([]string, error) {
	if tagsError != nil {
		return nil, tagsError
	}
	return []string{"a", "b"}, nil
}

// publicFailure supplies its own safe projection, which is the only way error
// detail reaches a recover subtree.
type publicFailure struct{ code string }

func (e publicFailure) Error() string { return "internal detail: " + e.code }
func (e publicFailure) PublicError() htmlbind.AsyncError {
	return htmlbind.AsyncError{Code: e.code, Message: "try again", Retryable: true}
}

// stream runs a render sequence to the end, writing each settled boundary. It
// is the loop a handler writes; there is deliberately no entry that hides it,
// because how many boundaries a render produces is not known up front.
//
// The runtime yields a fragment and the id of the placeholder it replaces, and
// nothing else. Framing that pair — here as an inert template plus the marker
// element a client script reacts to — belongs to whoever ships that script.
func stream(w io.Writer, sequence iter.Seq2[htmlbind.Content, error]) error {
	for content, err := range sequence {
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, `<template data-tb-boundary="`+content.BoundaryID+`">`); err != nil {
			return err
		}
		if _, err := content.WriteTo(w); err != nil {
			return err
		}
		if _, err := io.WriteString(w, `</template><tb-apply for="`+content.BoundaryID+`"></tb-apply>`); err != nil {
			return err
		}
		htmlbind.Flush(w)
	}
	return nil
}

func reset() {
	userDelay, userError, tagsError = 0, nil, nil
}

func TestSyncRenderSettlesBoundaryInPlace(t *testing.T) {
	reset()
	var output bytes.Buffer
	if err := htmlbind.Render(&output, Profile(ProfileParams{Id: "7"})); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	// The synchronous entry never writes the fallback, because the final
	// content is known before anything is emitted.
	if strings.Contains(got, "pending") || strings.Contains(got, "tb-boundary") {
		t.Fatalf("sync render leaked streaming markup:\n%s", got)
	}
	for _, want := range []string{"user-7", "<li>a</li>", "<li>b</li>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("sync render missing %q:\n%s", want, got)
		}
	}
}

// A framework owns the names its markup carries. Before this the placeholder
// and the identifier allocation were literals while the instance attribute was
// configurable, so a project setting the prefix got two naming systems in one
// document and could only choose one of them.
//
// The placeholder is a comment pair rather than an element now, so the prefix
// spells the markers instead of a tag name. An element around the fallback was
// foster-parented out of a table and left beside it.
func TestBoundaryPrefixNamesTheElementAndTheIdentifiers(t *testing.T) {
	reset()
	var output bytes.Buffer
	sequence := htmlbind.RenderAsync(context.Background(), &output,
		Profile(ProfileParams{Id: "7"}), htmlbind.WithBoundaryPrefix("pw"))
	if err := stream(&output, sequence); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, `<!--pw:pw-1-->`) {
		t.Fatalf("placeholder does not carry the configured prefix:\n%s", got)
	}
	if !strings.Contains(got, `<!--/pw:pw-1-->`) {
		t.Fatalf("placeholder is not closed with the configured name:\n%s", got)
	}
	// The completion addresses the identifier the placeholder was written
	// under, so a mismatch here would leave the fallback on screen forever.
	if !strings.Contains(got, `<template data-tb-boundary="pw-1">`) {
		t.Fatalf("completion does not address the prefixed identifier:\n%s", got)
	}
	if strings.Contains(got, "<!--tb:") {
		t.Fatalf("the default marker name survived the override:\n%s", got)
	}
}

func TestRenderAsyncWritesFallbackThenCompletion(t *testing.T) {
	reset()
	var output bytes.Buffer
	if err := stream(&output, htmlbind.RenderAsync(context.Background(), &output, Profile(ProfileParams{Id: "7"}))); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	placeholder := strings.Index(got, `<!--tb:tb-1-->`)
	completion := strings.Index(got, `<template data-tb-boundary="tb-1">`)
	if placeholder < 0 || completion < 0 {
		t.Fatalf("stream did not emit both halves of the boundary:\n%s", got)
	}
	if placeholder > completion {
		t.Fatalf("completion was written before its placeholder:\n%s", got)
	}
	if !strings.Contains(got[:completion], "loading") {
		t.Fatalf("fallback was not committed first:\n%s", got)
	}
	if !strings.Contains(got[completion:], "user-7") {
		t.Fatalf("completion does not carry the resolved subtree:\n%s", got)
	}
}

func TestRenderAsyncRecoversWithSafeErrorOnly(t *testing.T) {
	reset()
	userError = publicFailure{code: "upstream"}
	var reported error
	var output bytes.Buffer
	err := stream(&output, htmlbind.RenderAsync(context.Background(), &output, Profile(ProfileParams{Id: "7"}),
		htmlbind.WithErrorReporter(func(err error) { reported = err })))
	if err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, "upstream: try again") {
		t.Fatalf("recover subtree did not render:\n%s", got)
	}
	if strings.Contains(got, "internal detail") {
		t.Fatalf("raw error text reached the page:\n%s", got)
	}
	// The reporter is how a handled failure stays observable server-side.
	if reported == nil || !strings.Contains(reported.Error(), "internal detail") {
		t.Fatalf("reporter did not receive the original error: %v", reported)
	}
}

func TestBoundaryWithoutRecoverEndsTheSequence(t *testing.T) {
	reset()
	boom := errors.New("boom")
	userError = boom
	var reported error
	var output bytes.Buffer
	err := stream(&output, htmlbind.RenderAsync(context.Background(), &output, Silent(SilentParams{Id: "1"}),
		htmlbind.WithErrorReporter(func(err error) { reported = err })))
	// The template declared nowhere to put this failure, so it leaves the
	// boundary: a dropped failure would leave "waiting" on screen forever.
	var unrecovered *htmlbind.UnrecoveredError
	if !errors.As(err, &unrecovered) {
		t.Fatalf("err = %v, want an UnrecoveredError", err)
	}
	if unrecovered.BoundaryID != "tb-1" {
		t.Fatalf("BoundaryID = %q, want the placeholder left behind", unrecovered.BoundaryID)
	}
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the original failure underneath", err)
	}
	got := output.String()
	// What replaces the committed fallback is the caller's to write; the module
	// emits nothing more of its own.
	if !strings.Contains(got, "waiting") {
		t.Fatalf("fallback was not committed:\n%s", got)
	}
	if strings.Contains(got, "data-tb-boundary") {
		t.Fatalf("a clause without recover emitted a completion:\n%s", got)
	}
	if reported == nil {
		t.Fatal("failure without a recover clause was not reported")
	}
}

func TestSyncBoundaryWithoutRecoverFailsInsteadOfWritingFallback(t *testing.T) {
	reset()
	userError = errors.New("boom")
	var output bytes.Buffer
	err := htmlbind.Render(&output, Silent(SilentParams{Id: "1"}))
	var unrecovered *htmlbind.UnrecoveredError
	if !errors.As(err, &unrecovered) {
		t.Fatalf("err = %v, want an UnrecoveredError", err)
	}
	if unrecovered.BoundaryID != "" {
		t.Fatalf("BoundaryID = %q, want none on the path that writes no placeholder", unrecovered.BoundaryID)
	}
	// A finished document holding a loading state is worse than no document: a
	// caller rendering into a buffer can drop this one and send an error status.
	if strings.Contains(output.String(), "waiting") {
		t.Fatalf("sync render committed the fallback:\n%s", output.String())
	}
}

func TestTimeoutSurfacesAsTimeoutCode(t *testing.T) {
	reset()
	userDelay = 200 * time.Millisecond
	var output bytes.Buffer
	if err := stream(&output, htmlbind.RenderAsync(context.Background(), &output, Profile(ProfileParams{Id: "7"}),
		htmlbind.WithAsyncTimeout(10*time.Millisecond))); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); !strings.Contains(got, htmlbind.ErrorCodeTimeout) {
		t.Fatalf("timeout did not reach the recover subtree:\n%s", got)
	}
}

func TestCancelledRequestEmitsNoRecover(t *testing.T) {
	reset()
	userDelay = 200 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	var output bytes.Buffer
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	if err := stream(&output, htmlbind.RenderAsync(ctx, &output, Profile(ProfileParams{Id: "7"}))); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	// Cancellation bounds the wait, not the blocking external itself, so the
	// boundary simply never produces a completion.
	if strings.Contains(got, "data-tb-boundary") {
		t.Fatalf("cancellation produced a completion:\n%s", got)
	}
	if !strings.Contains(got, "loading") {
		t.Fatalf("committed fallback is missing:\n%s", got)
	}
}

func TestCachedComponentRunsOncePerKey(t *testing.T) {
	reset()
	store := htmlbind.NewMemoryCache(16)
	render := func(tone string) string {
		var output bytes.Buffer
		if err := htmlbind.Render(&output, renderBadgeFragment(tone), htmlbind.WithCache(store)); err != nil {
			t.Fatal(err)
		}
		return output.String()
	}
	first := render("solid")
	second := render("solid")
	if first != second {
		t.Fatalf("cached output differs:\n%q\n%q", first, second)
	}
	if store.Len() != 1 {
		t.Fatalf("equal inputs stored %d entries, want 1", store.Len())
	}
	// A changed parameter must not read the previous entry.
	if third := render("ghost"); third == first {
		t.Fatalf("a changed parameter reused the cached output: %q", third)
	}
	if store.Len() != 2 {
		t.Fatalf("changed input stored %d entries, want 2", store.Len())
	}
}

func TestRenderWithoutStoreIgnoresCache(t *testing.T) {
	reset()
	var withStore, without bytes.Buffer
	if err := htmlbind.Render(&withStore, renderBadgeFragment("solid"), htmlbind.WithCache(htmlbind.NewMemoryCache(4))); err != nil {
		t.Fatal(err)
	}
	if err := htmlbind.Render(&without, renderBadgeFragment("solid")); err != nil {
		t.Fatal(err)
	}
	if withStore.String() != without.String() {
		t.Fatalf("cache changed the output:\n%q\n%q", withStore.String(), without.String())
	}
}

func renderBadgeFragment(tone string) htmlbind.Fragment {
	return renderBadge(renderBadgeParams{User: User{Name: "ada", Admin: true}, Tone: tone})
}

// A caller decides before rendering whether a response will need the client
// runtime that applies settled boundaries. Asking must not render anything.
func TestHasAwaitBlockReportsOwnAndCalledBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fragment htmlbind.Fragment
		want     bool
	}{
		{"owns a boundary", Profile(ProfileParams{Id: "1"}), true},
		{"only calls a component that owns one", Page(PageParams{Id: "1"}), true},
		{"no boundary anywhere", Shell(ShellParams{}), false},
		{"private component with no boundary", renderBadgeFragment("solid"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fragment.HasAwaitBlock(); got != tc.want {
				t.Fatalf("HasAwaitBlock() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestChainHasAwaitBlockUnionsMembers(t *testing.T) {
	shell := BindShell(ShellParams{})
	if shell.HasAwaitBlock() {
		t.Fatal("a wrapper with no boundary reported one")
	}
	// The wrapper is sync and the leaf awaits, so the chain still needs the
	// runtime: the decision belongs to the chain, not to any one member.
	if !htmlbind.HasAwaitBlock([]htmlbind.Wrapper{shell}, Page(PageParams{Id: "1"})) {
		t.Fatal("chain with an awaiting leaf reported no boundary")
	}
	if htmlbind.HasAwaitBlock([]htmlbind.Wrapper{shell}, Shell(ShellParams{})) {
		t.Fatal("chain with no boundary anywhere reported one")
	}
	if !htmlbind.HasAwaitBlock(nil, Silent(SilentParams{Id: "1"})) {
		t.Fatal("wrapperless chain lost the leaf's boundary")
	}
}

func TestHasAwaitBlockDoesNotRender(t *testing.T) {
	reset()
	userDelay = time.Hour
	defer reset()
	// A boundary would block for an hour if asking rendered anything.
	if !Profile(ProfileParams{Id: "1"}).HasAwaitBlock() {
		t.Fatal("Profile reported no boundary")
	}
}
