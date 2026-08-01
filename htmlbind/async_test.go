package htmlbind

import (
	"bytes"
	"context"
	"errors"
	"io"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"
)

// consume runs a render sequence to the end, writing and flushing each settled
// boundary. A real handler frames each fragment for the client runtime it
// ships; these tests look at what the module itself produces, so they write the
// fragments bare.
func consume(w io.Writer, sequence iter.Seq2[Content, error]) error {
	for content, err := range sequence {
		if err != nil {
			return err
		}
		if _, err := content.WriteTo(w); err != nil {
			return err
		}
		Flush(w)
	}
	return nil
}

// staticPlan builds a plan writing literal markup, standing in for generated
// code in the tests below.
func staticPlan(parts ...string) *Plan[struct{}] {
	builder := Builder[struct{}]{}
	ops := make([]Op[struct{}], 0, len(parts))
	for _, part := range parts {
		ops = append(ops, builder.Static(part))
	}
	return &Plan[struct{}]{Ops: ops}
}

// awaitPlan builds a plan holding one boundary whose binding calls load.
func awaitPlan(load func() (string, error)) *Plan[struct{}] {
	return awaitPlanWith(load, []Op[AsyncError]{
		Builder[AsyncError]{}.Text(func(err AsyncError) string { return err.Code }),
	})
}

// silentAwaitPlan builds the same boundary with no recover subtree, which is
// what a clause that declared none compiles to.
func silentAwaitPlan(load func() (string, error)) *Plan[struct{}] {
	return awaitPlanWith(load, nil)
}

func awaitPlanWith(load func() (string, error), handler []Op[AsyncError]) *Plan[struct{}] {
	builder := Builder[struct{}]{}
	return &Plan[struct{}]{Ops: []Op[struct{}]{
		Await(
			func(ctx context.Context, _ struct{}) (string, error) {
				var value string
				if err := Concurrent(ctx, func() error {
					result, err := load()
					value = result
					return err
				}); err != nil {
					return "", err
				}
				return value, nil
			},
			func(_ struct{}, err AsyncError) AsyncError { return err },
			[]Op[string]{Builder[string]{}.Text(func(value string) string { return value })},
			[]Op[struct{}]{builder.Static("pending")},
			handler,
		),
	}}
}

func TestConcurrentReportsFirstBindingInDeclarationOrder(t *testing.T) {
	slow := errors.New("slow")
	fast := errors.New("fast")
	err := Concurrent(context.Background(),
		func() error { time.Sleep(20 * time.Millisecond); return slow },
		func() error { return fast },
	)
	// Reporting by declaration order rather than completion order keeps a
	// template with two failing bindings failing the same way every run.
	if !errors.Is(err, slow) {
		t.Fatalf("err = %v, want the first declared failure", err)
	}
}

func TestConcurrentRunsBindingsInParallel(t *testing.T) {
	// Two blocking externals of the same duration must overlap; running them in
	// sequence is the thing the goroutine wrapping exists to avoid.
	const pause = 50 * time.Millisecond
	start := time.Now()
	err := Concurrent(context.Background(),
		func() error { time.Sleep(pause); return nil },
		func() error { time.Sleep(pause); return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 2*pause {
		t.Fatalf("bindings took %s, want roughly one pause", elapsed)
	}
}

func TestConcurrentStopsWaitingWhenCancelled(t *testing.T) {
	// ctx bounds the wait, not the work: a blocking external cannot be
	// interrupted, so the runtime abandons it and reports the cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	err := Concurrent(ctx, func() error {
		defer close(finished)
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	<-finished
}

func TestConcurrentTurnsPanicIntoError(t *testing.T) {
	if !panicRecovery {
		// Logged rather than skipped: TinyGo's testing package implements
		// neither SkipNow nor the Goexit it needs, so calling Skip there fails
		// the run. The return is what keeps the panic away from a runtime that
		// cannot recover it.
		t.Log("recover does not run on this target, so a panicking external ends the program instead")
		return
	}
	err := Concurrent(context.Background(), func() error { panic("nope") })
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v, want the recovered panic", err)
	}
}

func TestNormalizeHidesErrorTextByDefault(t *testing.T) {
	normalized := normalizeAsyncError(errors.New("connection to db-7 refused"))
	if normalized.Code != ErrorCodeInternal || normalized.Message != "" {
		t.Fatalf("normalized = %+v, want an opaque internal failure", normalized)
	}
	deadline := normalizeAsyncError(context.DeadlineExceeded)
	if deadline.Code != ErrorCodeTimeout || !deadline.Timeout {
		t.Fatalf("normalized = %+v, want a timeout", deadline)
	}
}

// flushRecorder counts flushes so the streaming contract can be checked without
// a real HTTP connection. It is read while the render writes, so it guards its
// own state.
type flushRecorder struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	flushes int
}

func (f *flushRecorder) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buffer.Write(p)
}

func (f *flushRecorder) Flush() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.flushes++
}

func (f *flushRecorder) snapshot() (string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buffer.String(), f.flushes
}

func TestRenderAsyncFlushesAfterTheInitialPass(t *testing.T) {
	output := &flushRecorder{}
	release := make(chan struct{})
	plan := awaitPlan(func() (string, error) {
		<-release
		return "done", nil
	})
	done := make(chan error, 1)
	go func() { done <- consume(output, RenderAsync(context.Background(), output, Bind(plan, struct{}{}))) }()
	// The fallback has to reach the client before the binding settles, which is
	// the whole point of streaming.
	deadline := time.After(2 * time.Second)
	for {
		written, flushes := output.snapshot()
		if flushes > 0 {
			if !strings.Contains(written, "pending") {
				t.Fatalf("fallback was not written before the binding settled: %q", written)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("initial pass was not flushed while the binding was pending")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	written, flushes := output.snapshot()
	if flushes < 2 {
		t.Fatalf("flushed %d times, want one per chunk", flushes)
	}
	if !strings.Contains(written, "done") {
		t.Fatalf("completion was not written: %q", written)
	}
}

func TestAsyncRenderInjectsNoRuntimeOfItsOwn(t *testing.T) {
	shell := &Plan[Fragment]{Ops: []Op[Fragment]{
		Builder[Fragment]{}.Static("<head>"),
		Builder[Fragment]{}.MergedHead(),
		Builder[Fragment]{}.Static("</head>"),
		Builder[Fragment]{}.Slot(func(child Fragment) Fragment { return child }, nil),
	}}
	page := &Plan[struct{}]{Head: []string{`<title>t</title>`}, Ops: []Op[struct{}]{
		Builder[struct{}]{}.Static("body"),
	}}
	wrappers := []Wrapper{BindWrapper(shell, Fragment{}, func(target *Fragment, children Fragment) { *target = children })}

	var streamed bytes.Buffer
	if err := consume(&streamed, RenderChainAsync(context.Background(), &streamed, wrappers, Bind(page, struct{}{}))); err != nil {
		t.Fatal(err)
	}
	// Applying a completion is the framework's job, so the render contributes no
	// script of its own on either path.
	if strings.Contains(streamed.String(), "<script") {
		t.Fatalf("streaming render injected a client runtime: %q", streamed.String())
	}
	if !strings.Contains(streamed.String(), "<title>t</title>") {
		t.Fatalf("component head contributions were dropped: %q", streamed.String())
	}

	var settled bytes.Buffer
	if err := RenderChain(&settled, wrappers, Bind(page, struct{}{})); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(settled.String(), "<script") {
		t.Fatalf("synchronous render injected a client runtime: %q", settled.String())
	}
}

func TestContentWriteToEmitsTheFragmentAlone(t *testing.T) {
	var output bytes.Buffer
	written, err := Content{BoundaryID: "tb-4", HTML: []byte("<p>hi</p>")}.WriteTo(&output)
	if err != nil {
		t.Fatal(err)
	}
	// No template, no marker, no script: the caller frames the fragment to match
	// whatever client runtime it ships.
	want := `<p>hi</p>`
	if output.String() != want {
		t.Fatalf("content = %q, want %q", output.String(), want)
	}
	if written != int64(len(want)) {
		t.Fatalf("reported %d bytes, want %d", written, len(want))
	}
}

func TestEarlyStopDoesNotWaitForPendingWork(t *testing.T) {
	// One boundary settles at once and a second blocks, so the consumer can stop
	// the range while work is still outstanding.
	blocked := make(chan struct{})
	defer close(blocked)
	settled := awaitPlan(func() (string, error) { return "first", nil })
	pending := awaitPlan(func() (string, error) {
		<-blocked
		return "second", nil
	})
	page := &Plan[struct{}]{Ops: []Op[struct{}]{
		Builder[struct{}]{}.Component(func(struct{}) Fragment { return Bind(settled, struct{}{}) }),
		Builder[struct{}]{}.Component(func(struct{}) Fragment { return Bind(pending, struct{}{}) }),
	}}
	var output bytes.Buffer
	returned := make(chan int, 1)
	go func() {
		seen := 0
		for content, err := range RenderAsync(context.Background(), &output, Bind(page, struct{}{})) {
			if err != nil || content.BoundaryID == "" {
				t.Errorf("unexpected item %+v, err %v", content, err)
			}
			seen++
			break
		}
		returned <- seen
	}()
	select {
	case seen := <-returned:
		if seen != 1 {
			t.Fatalf("consumed %d items, want the one settled boundary", seen)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stopping the range blocked on the pending boundary")
	}
}

func TestUnrecoveredBoundaryEndsTheSequence(t *testing.T) {
	boom := errors.New("boom")
	plan := silentAwaitPlan(func() (string, error) { return "", boom })
	var output bytes.Buffer
	err := consume(&output, RenderAsync(context.Background(), &output, Bind(plan, struct{}{})))
	var unrecovered *UnrecoveredError
	if !errors.As(err, &unrecovered) {
		t.Fatalf("err = %v, want an UnrecoveredError", err)
	}
	// The id is what ties the failure back to the placeholder still on screen,
	// which is the one thing a caller needs to replace it.
	if unrecovered.BoundaryID != "tb-1" {
		t.Fatalf("BoundaryID = %q, want the placeholder left behind", unrecovered.BoundaryID)
	}
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the original failure underneath", err)
	}
	if !strings.Contains(output.String(), "pending") {
		t.Fatalf("committed fallback is missing: %q", output.String())
	}
}

func TestUnrecoveredBoundaryFailsTheSynchronousRender(t *testing.T) {
	boom := errors.New("boom")
	plan := silentAwaitPlan(func() (string, error) { return "", boom })
	var output bytes.Buffer
	err := Render(&output, Bind(plan, struct{}{}))
	var unrecovered *UnrecoveredError
	if !errors.As(err, &unrecovered) {
		t.Fatalf("err = %v, want an UnrecoveredError", err)
	}
	if unrecovered.BoundaryID != "" {
		t.Fatalf("BoundaryID = %q, want none where no placeholder is written", unrecovered.BoundaryID)
	}
	// Writing the fallback here would finish a document promising content that
	// will never arrive, and this path has committed no status to take back.
	if strings.Contains(output.String(), "pending") {
		t.Fatalf("synchronous render committed the fallback: %q", output.String())
	}
}

func TestChainValidationFailsBeforeWriting(t *testing.T) {
	var output bytes.Buffer
	if err := RenderChain(&output, nil, Fragment{}); !errors.Is(err, ErrNoLeaf) {
		t.Fatalf("err = %v, want ErrNoLeaf", err)
	}
	if output.Len() != 0 {
		t.Fatalf("a rejected chain wrote %q", output.String())
	}
	var streamed bytes.Buffer
	var failure error
	for _, err := range RenderChainAsync(context.Background(), &streamed, []Wrapper{{}}, Bind(staticPlan("x"), struct{}{})) {
		failure = err
	}
	if !errors.Is(failure, ErrNilWrapper) {
		t.Fatalf("err = %v, want ErrNilWrapper", failure)
	}
	if streamed.Len() != 0 {
		t.Fatalf("a rejected chain wrote %q", streamed.String())
	}
}
