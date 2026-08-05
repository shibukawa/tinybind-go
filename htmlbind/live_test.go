package htmlbind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// livePlan builds a plan holding one live boundary whose source is deliveries.
// handler is nil for a clause that declared no recover subtree.
func livePlan(deliveries func(context.Context) iter.Seq2[string, error], handler []Op[AsyncError]) *Plan[struct{}] {
	builder := Builder[struct{}]{}
	return &Plan[struct{}]{
		HasAwaitBlock: true,
		HasLiveBlock:  true,
		Ops: []Op[struct{}]{
			Live(
				func(ctx context.Context, _ struct{}) []LiveBinding[string] {
					return []LiveBinding[string]{
						func(deliver func(func(*string), error) bool) error {
							for value, err := range deliveries(ctx) {
								if !deliver(func(scope *string) { *scope = value }, err) {
									return nil
								}
							}
							return nil
						},
					}
				},
				func(struct{}) string { return "" },
				func(_ struct{}, err AsyncError) AsyncError { return err },
				[]Op[string]{Builder[string]{}.Text(func(value string) string { return value })},
				[]Op[struct{}]{builder.Static("pending")},
				handler,
			),
		},
	}
}

// recoverHandler renders the safe error code, which is what a recover subtree
// binding the error compiles to.
func recoverHandler() []Op[AsyncError] {
	return []Op[AsyncError]{Builder[AsyncError]{}.Text(func(err AsyncError) string { return err.Code })}
}

// values returns a source yielding each item in turn and then ending.
func values(items ...string) func(context.Context) iter.Seq2[string, error] {
	return func(context.Context) iter.Seq2[string, error] {
		return func(yield func(string, error) bool) {
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
		}
	}
}

// collect ranges a sequence and returns each delivery's boundary id and HTML.
func collect(t *testing.T, sequence iter.Seq2[Content, error]) ([]Content, error) {
	t.Helper()
	var got []Content
	for content, err := range sequence {
		if err != nil {
			return got, err
		}
		got = append(got, Content{BoundaryID: content.BoundaryID, HTML: bytes.Clone(content.HTML)})
	}
	return got, nil
}

func TestLiveYieldsEveryDelivery(t *testing.T) {
	var document bytes.Buffer
	got, err := collect(t, RenderLive(t.Context(), &document, Bind(livePlan(values("1", "2", "3"), recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("deliveries = %d, want 3", len(got))
	}
	for index, want := range []string{"1", "2", "3"} {
		if string(got[index].HTML) != want {
			t.Errorf("delivery %d = %q, want %q", index, got[index].HTML, want)
		}
		if got[index].BoundaryID != got[0].BoundaryID {
			t.Errorf("delivery %d boundary = %q, want %q: every delivery replaces the same placeholder",
				index, got[index].BoundaryID, got[0].BoundaryID)
		}
	}
	if !strings.Contains(document.String(), "pending") {
		t.Errorf("document = %q, want the fallback committed before any delivery", document.String())
	}
}

func TestLiveDocumentEntryTakesOnlyTheFirstDelivery(t *testing.T) {
	// The document response has to finish. A live boundary on it behaves like a
	// settled await boundary: real content on the first paint, then no more.
	var document bytes.Buffer
	got, err := collect(t, RenderAsync(t.Context(), &document, Bind(livePlan(values("1", "2", "3"), recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(got))
	}
	if string(got[0].HTML) != "1" {
		t.Errorf("delivery = %q, want %q", got[0].HTML, "1")
	}
}

func TestLiveUnsubscribeStopsTheSource(t *testing.T) {
	// Stopping the range is the unsubscribe, and the source observes it as its
	// own yield reporting false. Nothing else tells it to stop.
	produced := 0
	source := func(context.Context) iter.Seq2[string, error] {
		return func(yield func(string, error) bool) {
			for {
				produced++
				if !yield("tick", nil) {
					return
				}
			}
		}
	}
	if _, err := collect(t, RenderAsync(t.Context(), io.Discard, Bind(livePlan(source, recoverHandler()), struct{}{}))); err != nil {
		t.Fatalf("render: %v", err)
	}
	if produced != 1 {
		t.Errorf("produced = %d, want 1: the document entry takes one delivery and unsubscribes", produced)
	}
}

func TestLiveFailureDeliveryIsNotTerminal(t *testing.T) {
	// A yielded error is a delivery of a failure. The sequence ending is the
	// only terminal signal, so a transient fault shows recover content and the
	// next value replaces it.
	source := func(context.Context) iter.Seq2[string, error] {
		return func(yield func(string, error) bool) {
			if !yield("first", nil) {
				return
			}
			if !yield("", errors.New("source hiccup")) {
				return
			}
			yield("recovered", nil)
		}
	}
	got, err := collect(t, RenderLive(t.Context(), io.Discard, Bind(livePlan(source, recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := []string{"first", ErrorCodeInternal, "recovered"}
	if len(got) != len(want) {
		t.Fatalf("deliveries = %d, want %d", len(got), len(want))
	}
	for index, expected := range want {
		if string(got[index].HTML) != expected {
			t.Errorf("delivery %d = %q, want %q", index, got[index].HTML, expected)
		}
	}
}

func TestLiveErrorReporterRunsOffTheBoundaryLock(t *testing.T) {
	// The delivery lock serializes renders, not reports. A reporter is the
	// caller's code and may block — a full pipe, a synchronous exporter — and a
	// failing source is exactly when it is most likely to. Holding the lock
	// across it would freeze the boundary that is failing, which is the one
	// whose remaining sources most need to keep delivering.
	// Named for this test rather than plainly `scope`: see the note on
	// awaitScope in context_op_test.go for why a function-local parameter type
	// needs a package-unique name under TinyGo.
	type reporterScope struct{ A, B string }

	entered := make(chan struct{})
	release := make(chan struct{})
	aDelivered := make(chan struct{})
	firstRender := make(chan struct{})

	failing := func(deliver func(func(*reporterScope), error) bool) error {
		deliver(func(s *reporterScope) { s.A = "a" }, nil)
		close(aDelivered)
		<-firstRender
		// Renders recover, and then blocks in the reporter for as long as this
		// test holds it there.
		deliver(nil, errors.New("source failed"))
		return nil
	}
	healthy := func(deliver func(func(*reporterScope), error) bool) error {
		<-aDelivered
		deliver(func(s *reporterScope) { s.B = "1" }, nil)
		close(firstRender)
		<-entered
		// The reporter is blocked right now. This delivery can only be rendered
		// if the lock was released before the report was made.
		deliver(func(s *reporterScope) { s.B = "2" }, nil)
		return nil
	}

	plan := &Plan[struct{}]{
		HasAwaitBlock: true,
		HasLiveBlock:  true,
		Ops: []Op[struct{}]{
			Live(
				func(context.Context, struct{}) []LiveBinding[reporterScope] {
					return []LiveBinding[reporterScope]{failing, healthy}
				},
				func(struct{}) reporterScope { return reporterScope{} },
				func(_ struct{}, err AsyncError) AsyncError { return err },
				[]Op[reporterScope]{Builder[reporterScope]{}.Text(func(s reporterScope) string { return s.A + s.B })},
				[]Op[struct{}]{Builder[struct{}]{}.Static("pending")},
				recoverHandler(),
			),
		},
	}

	var reports atomic.Int64
	var once sync.Once
	deliveries := make(chan string, 4)
	finished := make(chan error, 1)
	go func() {
		var failure error
		// Each delivery is forwarded as it arrives rather than collected, so the
		// test can observe the third one while the reporter is still blocked.
		for content, err := range RenderLive(t.Context(), io.Discard, Bind(plan, struct{}{}),
			WithErrorReporter(func(error) {
				reports.Add(1)
				once.Do(func() { close(entered) })
				<-release
			})) {
			if err != nil {
				failure = err
				break
			}
			deliveries <- string(content.HTML)
		}
		finished <- failure
	}()

	next := func(what string) string {
		t.Helper()
		select {
		case html := <-deliveries:
			return html
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %s", what)
			return ""
		}
	}

	if html := next("the first render"); html != "a1" {
		t.Errorf("first delivery = %q, want %q", html, "a1")
	}
	if html := next("the recover render"); html != ErrorCodeInternal {
		t.Errorf("failure delivery = %q, want %q", html, ErrorCodeInternal)
	}
	// This is the assertion: the healthy binding delivers while the reporter is
	// still inside its call. Under the previous code it blocked on the lock the
	// reporter was holding and this timed out.
	if html := next("a delivery made while the reporter is blocked"); html != "a2" {
		t.Errorf("delivery during the report = %q, want %q", html, "a2")
	}
	close(release)

	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("render: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the subscription to end")
	}
	if reports.Load() != 1 {
		t.Errorf("reports = %d, want 1: the failure is reported exactly once", reports.Load())
	}
}

func TestLiveFailureWithNoRecoverEndsTheSubscription(t *testing.T) {
	source := func(context.Context) iter.Seq2[string, error] {
		return func(yield func(string, error) bool) {
			if !yield("first", nil) {
				return
			}
			yield("", errors.New("gone"))
		}
	}
	got, err := collect(t, RenderLive(t.Context(), io.Discard, Bind(livePlan(source, nil), struct{}{})))
	var unrecovered *UnrecoveredError
	if !errors.As(err, &unrecovered) {
		t.Fatalf("err = %v, want UnrecoveredError", err)
	}
	if unrecovered.BoundaryID == "" {
		t.Error("UnrecoveredError.BoundaryID is empty, want the committed placeholder's id")
	}
	if len(got) != 1 {
		t.Errorf("deliveries before the failure = %d, want 1", len(got))
	}
}

func TestLiveCancellationEndsQuietly(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	source := func(ctx context.Context) iter.Seq2[string, error] {
		return func(yield func(string, error) bool) {
			if !yield("first", nil) {
				return
			}
			cancel()
			<-ctx.Done()
			yield("", ctx.Err())
		}
	}
	got, err := collect(t, RenderLive(ctx, io.Discard, Bind(livePlan(source, recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("err = %v, want no error: expected cancellation produces no recover content", err)
	}
	if len(got) > 1 {
		t.Errorf("deliveries = %d, want at most the one before cancellation", len(got))
	}
}

func TestLiveSyncEntryRendersFirstDeliveryInPlace(t *testing.T) {
	var page bytes.Buffer
	if err := Render(&page, Bind(livePlan(values("now", "later"), recoverHandler()), struct{}{})); err != nil {
		t.Fatalf("render: %v", err)
	}
	if page.String() != "now" {
		t.Errorf("page = %q, want %q: the sync entry renders the first delivery and stops watching",
			page.String(), "now")
	}
}

func TestLiveSyncEntryKeepsFallbackWhenNothingIsDelivered(t *testing.T) {
	var page bytes.Buffer
	if err := Render(&page, Bind(livePlan(values(), recoverHandler()), struct{}{})); err != nil {
		t.Fatalf("render: %v", err)
	}
	if page.String() != "pending" {
		t.Errorf("page = %q, want the fallback: a source that delivered nothing has nothing to replace it with",
			page.String())
	}
}

func TestLiveBoundaryIDsRepeatAcrossRenders(t *testing.T) {
	// Re-executing the same chain has to produce the same placeholder ids, which
	// is what lets a reconnect address the boundaries already on screen without
	// the client sending anything to align them.
	first, err := collect(t, RenderAsync(t.Context(), io.Discard, Bind(livePlan(values("a"), recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("first render: %v", err)
	}
	second, err := collect(t, RenderAsync(t.Context(), io.Discard, Bind(livePlan(values("b"), recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("second render: %v", err)
	}
	if first[0].BoundaryID != second[0].BoundaryID {
		t.Errorf("boundary ids %q and %q differ across two renders of the same chain",
			first[0].BoundaryID, second[0].BoundaryID)
	}
}

func TestHasLiveBlockReportsTheChain(t *testing.T) {
	live := Bind(livePlan(values("a"), recoverHandler()), struct{}{})
	static := Bind(staticPlan("plain"), struct{}{})
	if !live.HasLiveBlock() {
		t.Error("live fragment reports no live block")
	}
	if static.HasLiveBlock() {
		t.Error("static fragment reports a live block")
	}
	if !HasLiveBlock(nil, live) {
		t.Error("chain holding a live leaf reports no live block")
	}
	if HasLiveBlock(nil, static) {
		t.Error("chain with no live boundary reports one")
	}
	if !live.HasAwaitBlock() {
		t.Error("a live boundary must also report an await block: the client needs the same runtime")
	}
}

func TestContentAppendJSONEscapesForScriptAndJSON(t *testing.T) {
	// Past the initial document there is no parser to feed, so a delivery is a
	// record rather than markup. The fragment has to survive both a JSON string
	// and an inline script element.
	content := Content{BoundaryID: `tb-1"`, HTML: []byte(`<p class="x">a & b</p></script>`)}
	got := string(content.AppendJSON(nil))
	for _, unsafe := range []string{"</script>", "\n"} {
		if strings.Contains(got, unsafe) {
			t.Errorf("encoded delivery contains %q unescaped: %s", unsafe, got)
		}
	}
	// Escaping is only correct if it round-trips: the client has to get the
	// exact fragment the server rendered.
	var decoded struct {
		ID   string `json:"id"`
		HTML string `json:"html"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("encoded delivery is not valid JSON: %v\n%s", err, got)
	}
	if decoded.ID != content.BoundaryID {
		t.Errorf("id round-tripped to %q, want %q", decoded.ID, content.BoundaryID)
	}
	if decoded.HTML != string(content.HTML) {
		t.Errorf("html round-tripped to %q, want %q", decoded.HTML, content.HTML)
	}
	// Appending is what lets a caller build a framed record without a second
	// buffer.
	if prefixed := string(content.AppendJSON([]byte("data: "))); !strings.HasPrefix(prefixed, "data: {") {
		t.Errorf("AppendJSON did not append to the given slice: %s", prefixed)
	}
}

// quiet is a source that never delivers and returns only when its context ends,
// which is how a live source behaves while its data is simply not changing.
func quiet(ctx context.Context) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) { <-ctx.Done() }
}

func TestQuietSourceDoesNotHangTheDocumentEntry(t *testing.T) {
	// The document response has to finish. A source with nothing to say yet must
	// not be able to hold it open.
	var document bytes.Buffer
	got, err := collect(t, RenderAsync(t.Context(), &document, Bind(livePlan(quiet, recoverHandler()), struct{}{}),
		WithAsyncTimeout(50*time.Millisecond)))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("deliveries = %d, want none: nothing was ever delivered", len(got))
	}
	// Running out of time is not a failure of the source, so the fallback stays
	// rather than being replaced by recover content.
	if !strings.Contains(document.String(), "pending") {
		t.Errorf("document = %q, want the committed fallback", document.String())
	}
	if strings.Contains(document.String(), ErrorCodeTimeout) {
		t.Errorf("document = %q, want no recover content for a quiet source", document.String())
	}
}

func TestQuietSourceFallsBackOnTheSyncEntry(t *testing.T) {
	// The non-JavaScript path has to answer too, and a fallback is the only
	// honest thing to answer with when no value arrived.
	var page bytes.Buffer
	if err := Render(&page, Bind(livePlan(quiet, recoverHandler()), struct{}{}),
		WithAsyncTimeout(50*time.Millisecond)); err != nil {
		t.Fatalf("render: %v", err)
	}
	if page.String() != "pending" {
		t.Errorf("page = %q, want the fallback", page.String())
	}
}

func TestLiveEntryLetsASourceStayQuiet(t *testing.T) {
	// The same deadline must not apply here: a live source is allowed to be
	// quiet for as long as its data is quiet.
	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := collect(t, RenderLive(ctx, io.Discard, Bind(livePlan(quiet, recoverHandler()), struct{}{}),
		WithAsyncTimeout(20*time.Millisecond))); err != nil {
		t.Fatalf("render: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("live subscription ended after %v, want it to outlive the boundary deadline", elapsed)
	}
}

// nestedPlan is a live boundary whose primary subtree opens an await boundary,
// which is the shape that used to mint a placeholder per delivery.
func nestedPlan(deliveries func(context.Context) iter.Seq2[string, error], inner func(string) (string, error)) *Plan[struct{}] {
	return &Plan[struct{}]{HasAwaitBlock: true, HasLiveBlock: true, Ops: []Op[struct{}]{
		Live(
			func(ctx context.Context, _ struct{}) []LiveBinding[string] {
				return []LiveBinding[string]{func(deliver func(func(*string), error) bool) error {
					for value, err := range deliveries(ctx) {
						if !deliver(func(scope *string) { *scope = value }, err) {
							return nil
						}
					}
					return nil
				}}
			},
			func(struct{}) string { return "" },
			func(_ struct{}, err AsyncError) AsyncError { return err },
			[]Op[string]{
				Await(
					func(_ context.Context, value string) (string, error) { return inner(value) },
					func(_ string, err AsyncError) AsyncError { return err },
					[]Op[string]{Builder[string]{}.Text(func(value string) string { return value })},
					[]Op[string]{Builder[string]{}.Static("inner-pending")},
					nil,
				),
			},
			[]Op[struct{}]{Builder[struct{}]{}.Static("pending")},
			recoverHandler(),
		),
	}}
}

func TestNestedBoundaryIDsAreReusedAcrossDeliveries(t *testing.T) {
	// A live boundary re-renders its subtree on the server's clock. If every
	// delivery minted new placeholder ids, a long-lived subscription would grow
	// them without bound and leave the client holding ones nothing replaces.
	ids := map[string]bool{}
	for content, err := range RenderLive(t.Context(), io.Discard,
		Bind(nestedPlan(values("1", "2", "3", "4", "5"), func(v string) (string, error) { return v, nil }), struct{}{})) {
		if err != nil {
			t.Fatal(err)
		}
		ids[content.BoundaryID] = true
	}
	// One for the live boundary and one for the await boundary inside it, however
	// many deliveries there were.
	if len(ids) != 2 {
		t.Errorf("distinct boundary ids = %d, want 2:\n%v", len(ids), ids)
	}
	if !ids["tb-1"] || !ids["tb-1-1"] {
		t.Errorf("ids = %v, want the boundary and its nested one named by position", ids)
	}
}

func TestSupersededDeliveryCannotLandOnAReusedPlaceholder(t *testing.T) {
	// Reusing ids means a nested boundary left over from an earlier delivery
	// would otherwise settle into the replacement's placeholder. The earlier
	// delivery's work is cancelled, so it never reports.
	release := make(chan struct{})
	inner := func(value string) (string, error) {
		if value == "1" {
			// The first delivery's nested boundary is still working when the
			// second delivery replaces it. Selecting by value matters: the two
			// nested boundaries run in their own goroutines, so which one calls
			// inner first is the scheduler's choice, and blocking the second
			// delivery's would block the delivery that is allowed to settle.
			<-release
			return "stale", nil
		}
		return "fresh-" + value, nil
	}
	source := func(context.Context) iter.Seq2[string, error] {
		return func(yield func(string, error) bool) {
			if !yield("1", nil) {
				return
			}
			if !yield("2", nil) {
				return
			}
			close(release)
		}
	}
	var nested []string
	for content, err := range RenderLive(t.Context(), io.Discard, Bind(nestedPlan(source, inner), struct{}{})) {
		if err != nil {
			t.Fatal(err)
		}
		if content.BoundaryID == "tb-1-1" {
			nested = append(nested, string(content.HTML))
		}
	}
	for _, fragment := range nested {
		if fragment == "stale" {
			t.Errorf("a superseded delivery's nested boundary reported into the reused placeholder: %v", nested)
		}
	}
}

func TestPanicInASourceBecomesARecoverableFailure(t *testing.T) {
	if !panicRecovery {
		t.Log("recover does not run on this target, so a panicking source ends the program instead")
		return
	}
	source := func(context.Context) iter.Seq2[string, error] {
		return func(yield func(string, error) bool) { panic("source blew up") }
	}
	got, err := collect(t, RenderLive(t.Context(), io.Discard, Bind(livePlan(source, recoverHandler()), struct{}{})))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(got) != 1 || string(got[0].HTML) != ErrorCodeInternal {
		t.Fatalf("deliveries = %q, want one recover render: a panic travels the same path as a returned error", got)
	}
}
