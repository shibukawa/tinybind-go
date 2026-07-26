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
func stream(w io.Writer, sequence iter.Seq2[htmlbind.Content, error]) error {
	for content, err := range sequence {
		if err != nil {
			return err
		}
		if _, err := content.WriteTo(w); err != nil {
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

func TestRenderAsyncWritesFallbackThenCompletion(t *testing.T) {
	reset()
	var output bytes.Buffer
	if err := stream(&output, htmlbind.RenderAsync(context.Background(), &output, Profile(ProfileParams{Id: "7"}))); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	placeholder := strings.Index(got, `<tb-boundary id="tb-1"`)
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

func TestBoundaryWithoutRecoverKeepsFallback(t *testing.T) {
	reset()
	userError = errors.New("boom")
	var reported error
	var output bytes.Buffer
	if err := stream(&output, htmlbind.RenderAsync(context.Background(), &output, Silent(SilentParams{Id: "1"}),
		htmlbind.WithErrorReporter(func(err error) { reported = err }))); err != nil {
		t.Fatal(err)
	}
	got := output.String()
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
