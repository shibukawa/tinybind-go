package htmlbind

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
	"testing"
)

// toastPayload stands in for a generated encoder: the payload appends itself,
// which is what keeps the codec at the emit site instead of at the seam.
type toastPayload struct{ Text string }

func (p toastPayload) AppendJSON(dst []byte) []byte {
	dst = append(dst, `{"text":`...)
	dst = appendJSONString(dst, p.Text)
	return append(dst, '}')
}

// appToast is an application signal type: one embed and one constructor.
type appToast struct{ Signal }

func newAppToast(text string) appToast {
	return appToast{NewSignal("app.toast", toastPayload{Text: text})}
}

// emissions is a source yielding a scripted mix of values and signals. A nil
// error is a delivery; anything else travels the error slot exactly as a source
// yields it.
type emission struct {
	value string
	err   error
}

func emits(items ...emission) func(context.Context) iter.Seq2[string, error] {
	return func(context.Context) iter.Seq2[string, error] {
		return func(yield func(string, error) bool) {
			for _, item := range items {
				if !yield(item.value, item.err) {
					return
				}
			}
		}
	}
}

func delivery(value string) emission { return emission{value: value} }
func signal(err error) emission      { return emission{err: err} }

// collectAll ranges a sequence keeping deliveries and signals apart, which is
// the loop the wire contract asks a caller to write.
func collectAll(t *testing.T, sequence iter.Seq2[Content, error]) (deliveries []Content, signals []Signal, err error) {
	t.Helper()
	for content, item := range sequence {
		if item != nil {
			if got, ok := AsSignal(item); ok {
				signals = append(signals, got)
				continue
			}
			return deliveries, signals, item
		}
		deliveries = append(deliveries, Content{BoundaryID: content.BoundaryID, HTML: bytes.Clone(content.HTML)})
	}
	return deliveries, signals, nil
}

func TestSignalTravelsBesideDeliveriesWithoutDisturbingThem(t *testing.T) {
	deliveries, signals, err := collectAll(t, RenderLive(t.Context(), &bytes.Buffer{},
		Bind(livePlan(emits(delivery("1"), signal(newAppToast("saved")), delivery("2")), recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(deliveries) != 2 {
		t.Fatalf("deliveries = %d, want 2: a signal renders nothing and suppresses nothing", len(deliveries))
	}
	for index, want := range []string{"1", "2"} {
		if string(deliveries[index].HTML) != want {
			t.Errorf("delivery %d = %q, want %q", index, deliveries[index].HTML, want)
		}
	}
	if len(signals) != 1 {
		t.Fatalf("signals = %d, want 1", len(signals))
	}
	if signals[0].Name() != "app.toast" {
		t.Errorf("name = %q, want %q", signals[0].Name(), "app.toast")
	}
	if got := string(signals[0].Payload()); got != `{"text":"saved"}` {
		t.Errorf("payload = %s, want the encoded struct", got)
	}
}

func TestSignalKeepsItsPlaceInTheSourcesOwnOrder(t *testing.T) {
	// Highlight what just arrived is only expressible if a signal stays ordered
	// against the deliveries around it.
	var order []string
	for content, item := range RenderLive(t.Context(), &bytes.Buffer{},
		Bind(livePlan(emits(
			delivery("a"), signal(NamedSignal("one")), delivery("b"), signal(NamedSignal("two")),
		), recoverHandler()), struct{}{})) {
		if item != nil {
			got, ok := AsSignal(item)
			if !ok {
				t.Fatalf("unexpected failure: %v", item)
			}
			order = append(order, "signal:"+got.Name())
			continue
		}
		order = append(order, "delivery:"+string(content.HTML))
	}
	want := "delivery:a signal:one delivery:b signal:two"
	if got := strings.Join(order, " "); got != want {
		t.Errorf("order = %q, want %q", got, want)
	}
}

func TestSignalWithNoRecoverClauseDoesNotEndTheSubscription(t *testing.T) {
	// A clause declaring no recover subtree ends on a failure. A signal is not
	// one, so this is the case that proves classification happens ahead of the
	// omitted-recover rule rather than beside it.
	deliveries, signals, err := collectAll(t, RenderLive(t.Context(), &bytes.Buffer{},
		Bind(livePlan(emits(signal(newAppToast("hi")), delivery("1")), nil), struct{}{})))
	if err != nil {
		t.Fatalf("render: %v, want the signal to pass through a clause with no recover subtree", err)
	}
	if len(signals) != 1 || len(deliveries) != 1 {
		t.Fatalf("signals = %d, deliveries = %d, want 1 and 1", len(signals), len(deliveries))
	}
}

func TestSignalRendersNoRecoverSubtree(t *testing.T) {
	deliveries, _, err := collectAll(t, RenderLive(t.Context(), &bytes.Buffer{},
		Bind(livePlan(emits(signal(NamedSignal("app.ping"))), recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(deliveries) != 0 {
		t.Errorf("deliveries = %d (%q), want none: a signal is not a failure and renders nothing",
			len(deliveries), deliveries[0].HTML)
	}
}

func TestSignalIsNotReportedToTheErrorHook(t *testing.T) {
	var reported []error
	_, _, err := collectAll(t, RenderLive(t.Context(), &bytes.Buffer{},
		Bind(livePlan(emits(signal(newAppToast("x")), delivery("1")), recoverHandler()), struct{}{}),
		WithErrorReporter(func(err error) { reported = append(reported, err) })))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(reported) != 0 {
		t.Errorf("reported = %v, want nothing: a signal is not an observation of a fault", reported)
	}
}

func TestSignalDoesNotSupersedeNestedBoundaries(t *testing.T) {
	// A delivery supersedes the previous one and cancels the nested work it
	// opened. A signal renders nothing, so it must leave that work alone: if it
	// took the delivery path the inner boundary would be cancelled and its
	// placeholder would keep a fallback nothing replaces.
	deliveries, signals, err := collectAll(t, RenderLive(t.Context(), io.Discard,
		Bind(nestedPlan(emits(delivery("1"), signal(NamedSignal("app.ping"))),
			func(value string) (string, error) { return "inner-" + value, nil }), struct{}{})))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("signals = %d, want 1", len(signals))
	}
	var inner string
	for _, content := range deliveries {
		if content.BoundaryID == "tb-1-1" {
			inner = string(content.HTML)
		}
	}
	if inner != "inner-1" {
		t.Errorf("nested boundary = %q, want %q: a signal must not cancel the work behind the content on screen", inner, "inner-1")
	}
}

func TestSignalOnTheDocumentEntryIsDroppedAndIsNotADelivery(t *testing.T) {
	// The document entry takes the first delivery and unsubscribes. A signal
	// must not consume that one shot, or a source emitting one first would leave
	// the fallback on a page that had real content to show.
	var document bytes.Buffer
	deliveries, signals, err := collectAll(t, RenderAsync(t.Context(), &document,
		Bind(livePlan(emits(signal(newAppToast("early")), delivery("1"), delivery("2")), recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("signals = %d, want none on the document entry", len(signals))
	}
	if len(deliveries) != 1 || string(deliveries[0].HTML) != "1" {
		t.Fatalf("deliveries = %v, want exactly the first value", deliveries)
	}
}

func TestSignalOnTheSyncEntryIsDropped(t *testing.T) {
	var page bytes.Buffer
	err := Render(&page, Bind(livePlan(emits(signal(newAppToast("x")), delivery("1")), recoverHandler()), struct{}{}))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := page.String(); got != "1" {
		t.Errorf("page = %q, want %q: a signal has no client here and must not decide the boundary", got, "1")
	}
}

func TestSyncEntryKeepsItsFallbackWhenOnlySignalsArrive(t *testing.T) {
	// Not marking a signal as delivered is what leaves the fallback available.
	var page bytes.Buffer
	err := Render(&page, Bind(livePlan(emits(signal(NamedSignal("app.ping"))), recoverHandler()), struct{}{}))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := page.String(); got != "pending" {
		t.Errorf("page = %q, want the fallback: nothing renderable arrived", got)
	}
}

func TestSignalIsRecognizedThroughWrapAndJoin(t *testing.T) {
	toast := newAppToast("saved")
	for name, err := range map[string]error{
		"bare":    toast,
		"wrapped": fmt.Errorf("carrying: %w", toast),
		"joined":  errors.Join(errors.New("other"), toast),
	} {
		got, ok := AsSignal(err)
		if !ok {
			t.Errorf("%s: AsSignal did not recognize it", name)
			continue
		}
		if got.Name() != "app.toast" {
			t.Errorf("%s: name = %q, want %q", name, got.Name(), "app.toast")
		}
		if !errors.Is(err, ErrSignal) {
			t.Errorf("%s: errors.Is did not match ErrSignal", name)
		}
	}
}

func TestOnlyAnEmbeddingTypeIsASignal(t *testing.T) {
	// The accessor is unexported, so nothing outside this package can satisfy
	// the interface without embedding. A type that merely looks like one is an
	// ordinary error.
	if _, ok := AsSignal(errors.New("app.toast")); ok {
		t.Error("a plain error was recognized as a signal")
	}
	if _, ok := AsSignal(&UnrecoveredError{Err: errors.New("boom")}); ok {
		t.Error("an UnrecoveredError was recognized as a signal")
	}
}

func TestSignalErrorTextNamesItWithoutThePayload(t *testing.T) {
	text := newAppToast("secret-value").Error()
	if !strings.Contains(text, "app.toast") {
		t.Errorf("Error() = %q, want the name", text)
	}
	if strings.Contains(text, "secret-value") {
		t.Errorf("Error() = %q, want no payload: an unclassified signal is logged by code that has no idea what it is", text)
	}
}

func TestSignalAppendJSON(t *testing.T) {
	for name, tc := range map[string]struct {
		signal Signal
		want   string
	}{
		"with payload": {NewSignal("app.toast", toastPayload{Text: "hi"}), `{"name":"app.toast","data":{"text":"hi"}}`},
		"no payload":   {NamedSignal("app.ping"), `{"name":"app.ping"}`},
		"raw payload":  {NewRawSignal("app.raw", []byte(`[1,2]`)), `{"name":"app.raw","data":[1,2]}`},
	} {
		if got := string(tc.signal.AppendJSON(nil)); got != tc.want {
			t.Errorf("%s: AppendJSON = %s, want %s", name, got, tc.want)
		}
	}
}

func TestSignalNameIsValidatedAtConstruction(t *testing.T) {
	for name, bad := range map[string]string{
		"empty":         "",
		"reserved":      "tb.delivery_applied",
		"leading dot":   ".toast",
		"leading digit": "1toast",
		"space":         "app toast",
		"slash":         "app/toast",
		"too long":      strings.Repeat("a", maxSignalNameLen+1),
	} {
		if fault := NamedSignal(bad).fault(); fault == nil {
			t.Errorf("%s: %q was accepted, want a construction fault", name, bad)
		}
	}
	for _, good := range []string{"toast", "app.toast", "app_toast", "app-toast", "a", "App.Toast2"} {
		if fault := NamedSignal(good).fault(); fault != nil {
			t.Errorf("%q was rejected: %v", good, fault)
		}
	}
}

func TestInvalidSignalTakesTheFailurePath(t *testing.T) {
	// A name the client could not have dispatched, or an embed that was never
	// constructed, is a fault in the source. It is loud where it happens rather
	// than dropped on the way out.
	for name, emitted := range map[string]error{
		"reserved name": NamedSignal("tb.delivery_applied"),
		"never built":   appToast{},
	} {
		_, signals, err := collectAll(t, RenderLive(t.Context(), &bytes.Buffer{},
			Bind(livePlan(emits(signal(emitted), delivery("1")), nil), struct{}{})))
		if err == nil {
			t.Errorf("%s: render succeeded, want the fault to end the clause with no recover subtree", name)
		}
		if len(signals) != 0 {
			t.Errorf("%s: signals = %d, want none to reach the caller", name, len(signals))
		}
	}
}

func TestInvalidSignalRendersRecoverWhenTheClauseHasOne(t *testing.T) {
	deliveries, _, err := collectAll(t, RenderLive(t.Context(), &bytes.Buffer{},
		Bind(livePlan(emits(signal(NamedSignal(""))), recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(deliveries) != 1 || string(deliveries[0].HTML) != ErrorCodeInternal {
		t.Errorf("deliveries = %v, want the recover subtree: a malformed signal is an ordinary fault", deliveries)
	}
}
