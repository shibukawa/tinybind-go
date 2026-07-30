package pages

import (
	"bytes"
	"context"
	"errors"
	"io"
	"iter"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// The generated code calls these. A live external is an ordinary Go function
// returning a sequence: the runtime pulls it, so a source that produces faster
// than the screen can use simply blocks in its own yield.
var (
	metricDeliveries []Point
	metricFailAfter  = -1
	messageBatches   [][]string
	// messageError ends the message source with a failure delivery, which is
	// how the no-recover path is reached.
	messageError error
)

func WatchMetrics(ctx context.Context, id string) iter.Seq2[Point, error] {
	return func(yield func(Point, error) bool) {
		for index, point := range metricDeliveries {
			if index == metricFailAfter {
				if !yield(Point{}, errors.New("source hiccup")) {
					return
				}
				continue
			}
			if ctx.Err() != nil {
				return
			}
			if !yield(point, nil) {
				return
			}
		}
	}
}

// LoadTitle is an ordinary settle-once async external. It sits in the same
// clause as a live source in Mixed.
func LoadTitle(id string) (string, error) {
	return "dashboard " + id, nil
}

func WatchMessages(ctx context.Context, room string) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {
		for _, batch := range messageBatches {
			if !yield(batch, nil) {
				return
			}
		}
		if messageError != nil {
			yield(nil, messageError)
		}
	}
}

func reset() {
	metricDeliveries = []Point{
		{Label: "cpu", Value: 10},
		{Label: "cpu", Value: 20},
		{Label: "cpu", Value: 30},
	}
	metricFailAfter = -1
	messageBatches = [][]string{{"hello"}, {"hello", "world"}}
	messageError = nil
}

// deliveries collects each yielded fragment, which is what a handler would
// frame for its client runtime.
func deliveries(t *testing.T, sequence iter.Seq2[htmlbind.Content, error]) ([]string, error) {
	t.Helper()
	var got []string
	for content, err := range sequence {
		if err != nil {
			return got, err
		}
		got = append(got, string(content.HTML))
	}
	return got, nil
}

func TestLiveEntryDeliversEveryUpdate(t *testing.T) {
	reset()
	var document bytes.Buffer
	got, err := deliveries(t, htmlbind.RenderLive(t.Context(), &document, Gauge(GaugeParams{Id: "7"})))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("deliveries = %d, want 3:\n%q", len(got), got)
	}
	for index, want := range []string{"cpu: 10", "cpu: 20", "cpu: 30"} {
		if !strings.Contains(got[index], want) {
			t.Errorf("delivery %d = %q, want it to carry %q", index, got[index], want)
		}
	}
	// The fallback commits first, exactly as an await boundary's does.
	if !strings.Contains(document.String(), "waiting") {
		t.Errorf("fallback was not committed:\n%s", document.String())
	}
	if !strings.Contains(document.String(), `<tb-boundary id="tb-1"`) {
		t.Errorf("no placeholder was written:\n%s", document.String())
	}
}

func TestDocumentEntryTakesOneDeliveryAndFinishes(t *testing.T) {
	reset()
	// A document response has to end. Taking the first delivery is what puts
	// real content on the first paint instead of a loading state.
	var document bytes.Buffer
	got, err := deliveries(t, htmlbind.RenderAsync(t.Context(), &document, Gauge(GaugeParams{Id: "7"})))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("deliveries = %d, want 1:\n%q", len(got), got)
	}
	if !strings.Contains(got[0], "cpu: 10") {
		t.Errorf("delivery = %q, want the first value", got[0])
	}
}

func TestTransientFailureIsFollowedByRecovery(t *testing.T) {
	reset()
	metricFailAfter = 1
	var reported []error
	got, err := deliveries(t, htmlbind.RenderLive(t.Context(), io.Discard, Gauge(GaugeParams{Id: "7"}),
		htmlbind.WithErrorReporter(func(err error) { reported = append(reported, err) })))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("deliveries = %d, want 3:\n%q", len(got), got)
	}
	// A yielded error is a delivery of a failure, not the end of the source, so
	// the boundary shows recover content and the next value replaces it.
	if !strings.Contains(got[1], htmlbind.ErrorCodeInternal) {
		t.Errorf("delivery 1 = %q, want the recover subtree", got[1])
	}
	if !strings.Contains(got[2], "cpu: 30") {
		t.Errorf("delivery 2 = %q, want primary content again after the fault", got[2])
	}
	if len(reported) != 1 {
		t.Errorf("reported %d errors, want the one failure to stay observable", len(reported))
	}
}

func TestNoRecoverClauseEndsTheSubscription(t *testing.T) {
	reset()
	messageError = errors.New("room closed")
	var document bytes.Buffer
	got, err := deliveries(t, htmlbind.RenderLive(t.Context(), &document, Chat(ChatParams{Room: "go"})))
	var unrecovered *htmlbind.UnrecoveredError
	if !errors.As(err, &unrecovered) {
		t.Fatalf("err = %v, want an UnrecoveredError", err)
	}
	if unrecovered.BoundaryID != "tb-1" {
		t.Errorf("BoundaryID = %q, want the committed placeholder", unrecovered.BoundaryID)
	}
	if len(got) != 2 {
		t.Fatalf("deliveries before the failure = %d, want 2:\n%q", len(got), got)
	}
	if !strings.Contains(got[1], "<li>world</li>") {
		t.Errorf("second delivery = %q, want the grown list", got[1])
	}
}

func TestSyncEntryRendersFirstDeliveryInPlace(t *testing.T) {
	reset()
	var page bytes.Buffer
	if err := htmlbind.Render(&page, Gauge(GaugeParams{Id: "7"})); err != nil {
		t.Fatal(err)
	}
	got := page.String()
	// One template serves a live client and a client that will never ask for
	// updates, including one with no JavaScript.
	if !strings.Contains(got, "cpu: 10") {
		t.Errorf("sync render did not settle the boundary:\n%s", got)
	}
	if strings.Contains(got, "tb-boundary") || strings.Contains(got, "waiting") {
		t.Errorf("sync render leaked streaming markup:\n%s", got)
	}
}

func TestBoundaryIDsRepeatAcrossExecutions(t *testing.T) {
	reset()
	var first, second bytes.Buffer
	if _, err := deliveries(t, htmlbind.RenderAsync(t.Context(), &first, Gauge(GaugeParams{Id: "7"}))); err != nil {
		t.Fatal(err)
	}
	if _, err := deliveries(t, htmlbind.RenderAsync(t.Context(), &second, Gauge(GaugeParams{Id: "7"}))); err != nil {
		t.Fatal(err)
	}
	// Re-executing the same page has to address the placeholders already on
	// screen, which is what lets a reconnect carry no state of its own.
	if !strings.Contains(first.String(), `id="tb-1"`) || !strings.Contains(second.String(), `id="tb-1"`) {
		t.Errorf("boundary ids are not stable across executions:\n%s\n%s", first.String(), second.String())
	}
}

func TestSeveralBindingsRenderEveryCurrentValue(t *testing.T) {
	reset()
	// Nothing selects between the sources: whichever one moves, the subtree is
	// rendered again reading all of them.
	got, err := deliveries(t, htmlbind.RenderLive(t.Context(), io.Discard, Dashboard(DashboardParams{Id: "7"})))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("deliveries = %d, want one per value from either source:\n%q", len(got), got)
	}
	for index, fragment := range got {
		// Every render carries both bindings, including the one that did not
		// move for this delivery.
		if !strings.Contains(fragment, "cpu:") {
			t.Errorf("delivery %d = %q, want the metric binding", index, fragment)
		}
		if !strings.Contains(fragment, "<li>hello</li>") {
			t.Errorf("delivery %d = %q, want the message binding", index, fragment)
		}
	}
	// The last render sees the last value of each source.
	last := got[len(got)-1]
	if !strings.Contains(last, "cpu: 30") || !strings.Contains(last, "<li>world</li>") {
		t.Errorf("last delivery = %q, want the latest value of both bindings", last)
	}
}

func TestFirstRenderWaitsForEveryBinding(t *testing.T) {
	reset()
	// A binding that never delivers keeps the boundary on its fallback: the
	// subtree reads every binding, so rendering early would show a zero value
	// for the one that has not arrived.
	messageBatches = nil
	var document bytes.Buffer
	got, err := deliveries(t, htmlbind.RenderAsync(t.Context(), &document, Dashboard(DashboardParams{Id: "7"})))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("deliveries = %d, want none until every binding has a value:\n%q", len(got), got)
	}
	if !strings.Contains(document.String(), "waiting") {
		t.Errorf("fallback was not committed:\n%s", document.String())
	}
}

func TestOneClauseMixesSettleOnceAndLiveBindings(t *testing.T) {
	reset()
	// The clause says which values the subtree waits for; the declarations say
	// how often each arrives. A settle-once binding delivers once and a live one
	// keeps delivering, and every render reads both.
	got, err := deliveries(t, htmlbind.RenderLive(t.Context(), io.Discard, Mixed(MixedParams{Id: "7"})))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("no deliveries")
	}
	for index, fragment := range got {
		// Nothing renders before every binding has a value, and the settled one
		// is on every later render rather than only the first.
		if !strings.Contains(fragment, "dashboard 7") {
			t.Errorf("delivery %d = %q, want the settle-once binding on every render", index, fragment)
		}
		if !strings.Contains(fragment, "cpu:") {
			t.Errorf("delivery %d = %q, want the live binding", index, fragment)
		}
	}
	// How many renders there are is timing: metric values produced while the
	// settle-once binding is still pending are coalesced, because the newest one
	// is sufficient by construction. What is guaranteed is that the last render
	// carries the source's last value.
	if last := got[len(got)-1]; !strings.Contains(last, "cpu: 30") {
		t.Errorf("last delivery = %q, want the live source's final value", last)
	}
}

func TestCapabilityFlagsSeparateLiveFromAwait(t *testing.T) {
	reset()
	gauge := Gauge(GaugeParams{Id: "1"})
	static := Static(StaticParams{Id: "1"})
	if !gauge.HasLiveBlock() {
		t.Error("a component owning a live boundary reports none")
	}
	if !gauge.HasAwaitBlock() {
		t.Error("a live boundary must also report an await block: the client needs the same runtime")
	}
	if static.HasLiveBlock() || static.HasAwaitBlock() {
		t.Error("a component with no boundary reports one")
	}
	if !htmlbind.HasLiveBlock(nil, gauge) {
		t.Error("chain with a live leaf reports no live block")
	}
	if htmlbind.HasLiveBlock(nil, static) {
		t.Error("chain with no live boundary reports one")
	}
}
