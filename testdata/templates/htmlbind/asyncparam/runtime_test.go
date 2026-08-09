package pages

import (
	"bytes"
	"context"
	"errors"
	"io"
	"iter"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// LoadBadge is the async external the mixed clause calls beside a caller
// started value, so one boundary settles both kinds of source together.
func LoadBadge(name string) (string, error) { return "badge-" + name, nil }

// stream runs a render sequence to the end, writing each settled boundary. The
// runtime yields the fragment and its boundary id; the framing that carries them
// to the browser belongs to the layer that ships the client script, which here
// is this helper.
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

// startOrders is the handler side of the feature: the work begins here, before
// anything renders, and the template only waits for it.
func startOrders(ctx context.Context, calls *atomic.Int32, delay time.Duration) htmlbind.Pending[[]Order] {
	return htmlbind.Go(ctx, func(ctx context.Context) ([]Order, error) {
		if calls != nil {
			calls.Add(1)
		}
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return []Order{{Id: "a1", Total: 10}, {Id: "b2", Total: 20}}, nil
	})
}

func TestCallerStartedValueStreamsFallbackThenCompletion(t *testing.T) {
	ctx := context.Background()
	customer := Customer{Name: "ada", Orders: startOrders(ctx, nil, 20*time.Millisecond)}
	var output bytes.Buffer
	if err := stream(&output, htmlbind.RenderAsync(ctx, &output, Profile(ProfileParams{Customer: customer}))); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	completion := strings.Index(got, `<template data-tb-boundary="tb-1">`)
	if completion < 0 {
		t.Fatalf("no completion was emitted:\n%s", got)
	}
	// The record's settled field renders in the fallback, which is the point of
	// allowing async on a field rather than only on a whole parameter.
	if !strings.Contains(got[:completion], "loading ada") {
		t.Fatalf("fallback did not commit first with the settled field:\n%s", got)
	}
	for _, want := range []string{"a1: 10", "b2: 20", "badge-ada"} {
		if !strings.Contains(got[completion:], want) {
			t.Fatalf("completion missing %q:\n%s", want, got)
		}
	}
}

func TestOneValueAwaitedByLayoutAndPageRunsTheWorkOnce(t *testing.T) {
	ctx := context.Background()
	var calls atomic.Int32
	customer := Customer{Name: "ada", Orders: startOrders(ctx, &calls, 0)}
	var output bytes.Buffer
	sequence := htmlbind.RenderChainAsync(ctx, &output,
		[]htmlbind.Wrapper{BindLayout(LayoutParams{Customer: customer})},
		Profile(ProfileParams{Customer: customer}))
	if err := stream(&output, sequence); err != nil {
		t.Fatal(err)
	}
	// A channel would have delivered the orders to whichever boundary received
	// first and left the other waiting; a settled handle is read by both.
	if got := calls.Load(); got != 1 {
		t.Fatalf("work ran %d times, want 1", got)
	}
	got := output.String()
	if strings.Count(got, "a1") < 2 {
		t.Fatalf("both boundaries did not read the same value:\n%s", got)
	}
}

func TestUnsetOptionalSettlesAbsentWithoutPanicking(t *testing.T) {
	ctx := context.Background()
	// Headline is left at its zero value: the caller had nothing to pass.
	customer := Customer{Name: "ada", Orders: htmlbind.Resolved([]Order{{Id: "a1", Total: 10}})}
	var output bytes.Buffer
	if err := stream(&output, htmlbind.RenderAsync(ctx, &output, Profile(ProfileParams{Customer: customer}))); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, `<p class="note">`) {
		t.Fatalf("absent optional did not render its primary subtree:\n%s", got)
	}
	// An absent optional attribute is omitted, not written empty, and absence
	// never reaches the recover subtree because it is data rather than failure.
	if strings.Contains(got, "title=") {
		t.Fatalf("absent value was rendered as an attribute:\n%s", got)
	}
}

func TestUnsetRequiredValueFailsBeforeAnyByteCommits(t *testing.T) {
	ctx := context.Background()
	// Orders is required, and this caller forgot it.
	customer := Customer{Name: "ada"}
	var output bytes.Buffer
	err := stream(&output, htmlbind.RenderAsync(ctx, &output, Profile(ProfileParams{Customer: customer})))
	if err == nil {
		t.Fatal("unset required value rendered without an error")
	}
	var unset *htmlbind.UnsetPendingError
	if !errors.As(err, &unset) || unset.Path != "customer.orders" {
		t.Fatalf("error did not name the unset value: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("bytes were committed before the check ran:\n%s", output.String())
	}
	// The synchronous entry reports the same thing, so a caller without
	// progressive delivery is not left with a silently empty section.
	var sync bytes.Buffer
	if err := htmlbind.Render(&sync, Profile(ProfileParams{Customer: customer})); err == nil {
		t.Fatal("sync render accepted an unset required value")
	}
}

func TestFailedValueRoutesToRecover(t *testing.T) {
	ctx := context.Background()
	customer := Customer{Name: "ada", Orders: htmlbind.Failed[[]Order](errors.New("upstream is down"))}
	var reported error
	var output bytes.Buffer
	if err := stream(&output, htmlbind.RenderAsync(ctx, &output, Profile(ProfileParams{Customer: customer}),
		htmlbind.WithErrorReporter(func(err error) { reported = err }))); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, htmlbind.ErrorCodeInternal) {
		t.Fatalf("recover subtree did not render:\n%s", got)
	}
	if strings.Contains(got, "upstream is down") {
		t.Fatalf("raw error text reached the page:\n%s", got)
	}
	if reported == nil || !strings.Contains(reported.Error(), "upstream is down") {
		t.Fatalf("reporter did not receive the original error: %v", reported)
	}
}

func TestWorkPanicBecomesTheBoundaryError(t *testing.T) {
	ctx := context.Background()
	customer := Customer{Name: "ada", Orders: htmlbind.Go(ctx, func(context.Context) ([]Order, error) {
		panic("index out of range")
	})}
	var output bytes.Buffer
	if err := stream(&output, htmlbind.RenderAsync(ctx, &output, Profile(ProfileParams{Customer: customer}))); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); !strings.Contains(got, htmlbind.ErrorCodeInternal) {
		t.Fatalf("panic did not settle the handle as a failure:\n%s", got)
	}
}

func TestForBodyAwaitsItsOwnRow(t *testing.T) {
	ctx := context.Background()
	rows := []Row{
		{Label: "first", Count: htmlbind.Resolved(1)},
		{Label: "second", Count: htmlbind.Go(ctx, func(context.Context) (int, error) { return 2, nil })},
	}
	var output bytes.Buffer
	if err := stream(&output, htmlbind.RenderAsync(ctx, &output, Rows(RowsParams{Rows: rows}))); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	// One boundary per iteration, per the await clause inside a for body.
	if !strings.Contains(got, `<!--tb:tb-1-->`) || !strings.Contains(got, `<!--tb:tb-2-->`) {
		t.Fatalf("each row did not open its own boundary:\n%s", got)
	}
	for _, want := range []string{"first: 1", "second: 2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("completion missing %q:\n%s", want, got)
		}
	}
}

func TestSyncEntryBlocksOnTheValue(t *testing.T) {
	ctx := context.Background()
	customer := Customer{Name: "ada", Orders: startOrders(ctx, nil, 5*time.Millisecond)}
	var output bytes.Buffer
	if err := htmlbind.Render(&output, Profile(ProfileParams{Customer: customer,
		Headline: htmlbind.Resolved(ptr("hello"))}), htmlbind.WithContext(ctx)); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	// The same template renders settled in place, which is what serves a client
	// without JavaScript.
	if strings.Contains(got, "<!--tb:") || strings.Contains(got, "loading") {
		t.Fatalf("sync render leaked streaming markup:\n%s", got)
	}
	for _, want := range []string{"a1: 10", `title="hello"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("sync render missing %q:\n%s", want, got)
		}
	}
}

func ptr[T any](value T) *T { return &value }
