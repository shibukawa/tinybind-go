package htmlbind

import (
	"context"
	"errors"
	"io"
	"sync"
)

// Error codes carried by AsyncError. Generated templates compare against them
// through the built-in error type's code field.
const (
	// ErrorCodeInternal is the code for any failure that supplied no public
	// projection of its own. Raw Go errors never reach a recover subtree.
	ErrorCodeInternal = "internal"
	// ErrorCodeTimeout is the code for a boundary that exceeded its deadline.
	ErrorCodeTimeout = "timeout"
)

// AsyncError is the presentation-safe failure value an await boundary's recover
// clause receives. It carries only fields a template may render; the original
// Go error stays server-side and reaches the caller through WithErrorReporter.
type AsyncError struct {
	// Code is a stable classification, either an application code or one of the
	// ErrorCode constants above.
	Code string
	// Message is optional presentation-safe text. It is empty unless the failing
	// error supplied one.
	Message string
	// Retryable reports whether the UI may offer a retry.
	Retryable bool
	// Timeout reports whether the configured async deadline expired.
	Timeout bool
}

// PublicError is implemented by an error that supplies its own safe projection.
// Any other error is exposed to a recover clause as ErrorCodeInternal with no
// message, so error text cannot leak into a page by accident.
type PublicError interface {
	error
	PublicError() AsyncError
}

// UnrecoveredError reports an await boundary whose bindings failed in a clause
// that declared no recover subtree. The template said nothing about what to
// show, so the failure leaves the boundary instead of stopping there: the
// synchronous entries return it, and the streaming sequence yields it and ends.
//
// It carries the original Go error rather than the safe AsyncError projection,
// because it reaches the caller's Go code and never a template. What a caller
// puts on the page in response is its own to write, and must not be this text.
type UnrecoveredError struct {
	// BoundaryID is the placeholder whose fallback is committed to the response.
	// It is empty on the synchronous path, which writes no placeholder.
	BoundaryID string
	// Err is the failure the bindings reported.
	Err error
}

func (e *UnrecoveredError) Error() string {
	where := "await boundary"
	if e.BoundaryID != "" {
		where += " " + e.BoundaryID
	}
	return "htmlbind: " + where + " failed with no recover clause: " + e.Err.Error()
}

func (e *UnrecoveredError) Unwrap() error { return e.Err }

// normalizeAsyncError maps a Go error to the value a recover clause sees.
func normalizeAsyncError(err error) AsyncError {
	if public, ok := findPublicError(err); ok {
		return public.PublicError()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return AsyncError{Code: ErrorCodeTimeout, Retryable: true, Timeout: true}
	}
	return AsyncError{Code: ErrorCodeInternal}
}

// findPublicError walks the wrap chain looking for the one interface this
// package projects. It is errors.As for a fixed interface target, written out
// so the runtime compiled into an application never links reflect for it.
func findPublicError(err error) (PublicError, bool) {
	for err != nil {
		if public, ok := err.(PublicError); ok {
			return public, true
		}
		switch wrapper := err.(type) {
		case interface{ Unwrap() error }:
			err = wrapper.Unwrap()
		case interface{ Unwrap() []error }:
			for _, wrapped := range wrapper.Unwrap() {
				if public, ok := findPublicError(wrapped); ok {
					return public, true
				}
			}
			return nil, false
		default:
			return nil, false
		}
	}
	return nil, false
}

// panicError carries a recovered panic so it travels the same path as a
// returned error.
type panicError struct{ value any }

func (e *panicError) Error() string {
	const prefix = "htmlbind: panic in async external"
	// The usual panic values are covered by hand rather than handed to fmt,
	// whose %v formatter would be the only reflection this package links.
	switch value := e.value.(type) {
	case error:
		return prefix + ": " + value.Error()
	case string:
		return prefix + ": " + value
	case interface{ String() string }:
		return prefix + ": " + value.String()
	default:
		return prefix
	}
}

// Concurrent runs every task in its own goroutine and reports the first failure
// in task order. Generated await clauses call it with one task per binding.
//
// This is the whole reason an async external stays an ordinary blocking Go
// function: the function knows how to fetch its value, and the runtime owns
// running it off the render path and joining the results. Each task assigns its
// own field of the boundary scope, so the tasks share no memory and need no
// lock.
//
// ctx bounds the wait, not the work. When it is cancelled Concurrent returns
// straight away and the still-running tasks are abandoned: their results are
// discarded and the caller must not read the scope they were writing. A task
// that wants to stop early has to take a context of its own.
func Concurrent(ctx context.Context, tasks ...func() error) error {
	errs := make([]error, len(tasks))
	var wg sync.WaitGroup
	for index, task := range tasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[index] = runTask(task)
		}()
	}
	joined := make(chan struct{})
	go func() {
		wg.Wait()
		close(joined)
	}()
	select {
	case <-joined:
	case <-ctx.Done():
		// Returning without touching errs is what keeps an abandoned task from
		// racing this goroutine.
		return ctx.Err()
	}
	// Report in declaration order rather than completion order, so a template
	// with two failing bindings fails the same way on every run.
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// runTask executes one binding, turning a panic into an ordinary error.
func runTask(task func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &panicError{value: recovered}
		}
	}()
	return task()
}

// Content is one settled await boundary: the placeholder it replaces and the
// HTML that replaces it. The runtime yields these after the initial document
// write, and the ranging caller is the only code that writes them.
type Content struct {
	// BoundaryID matches the placeholder written during the initial pass.
	BoundaryID string
	// HTML is the already escaped and context-checked replacement fragment.
	// Consumers must not escape it again.
	HTML []byte
}

// AppendJSON appends this delivery as a JSON object with an id and an html
// field, and returns the extended slice.
//
// It exists because past the initial document there is no parser to feed. The
// template-and-marker framing requirement:suspense-html-streaming defines is for
// bytes the HTML parser is consuming; a client reading a fetch stream is not
// parsing markup, so a record is the natural form and JSON is the ordinary
// record. A caller streaming completions into a live document wants this; a
// caller writing into the document response still writes markup.
//
// The fragment is escaped for a script context as well as a JSON one, using the
// same rules as the generated encoders, so the result stays safe to embed in an
// inline script element as well as to send as a body. Framing around the record
// — newline-delimited, an event stream, a length prefix — is still the caller's
// to choose, because it has to match the client that reads it.
func (c Content) AppendJSON(dst []byte) []byte {
	dst = append(dst, `{"id":`...)
	dst = appendJSONString(dst, c.BoundaryID)
	dst = append(dst, `,"html":`...)
	dst = appendJSONString(dst, string(c.HTML))
	return append(dst, '}')
}

// WriteTo writes the settled fragment and nothing else: no wrapper element, no
// marker, no script.
//
// htmlbind does not pick a wire format for completions. The framing that carries
// a fragment and the client code that acts on it are one design, and it belongs
// to whoever ships the runtime — a framework built on htmlbind, or the handler
// itself. BoundaryID is what ties this fragment back to its placeholder, so the
// caller has both halves and writes the framing around this call.
func (c Content) WriteTo(w io.Writer) (int64, error) {
	written, err := w.Write(c.HTML)
	return int64(written), err
}

// asyncCoordinator owns every boundary opened during one render. Boundary work
// runs in its own goroutine and renders into its own buffer; only the ranging
// caller and the initial pass ever write the response.
type asyncCoordinator struct {
	ctx    context.Context
	cancel context.CancelFunc
	opts   *renderOptions

	wg      sync.WaitGroup
	results chan boundaryResult
	// sem bounds simultaneously running boundary work when a limit is set.
	sem chan struct{}
}

// boundaryResult is one of three things: a settled boundary, a failure that
// ends the sequence — a subtree that would not render, or bindings that failed
// in a clause with no recover subtree — or a signal a live source emitted,
// which reaches the caller in the sequence's error position and ends nothing.
type boundaryResult struct {
	content Content
	present bool
	err     error
	// signal is set when this result is a forwarded signal. It is a pointer so
	// the three states stay distinguishable without a second flag.
	signal *Signal
}

func newAsyncCoordinator(ctx context.Context, opts *renderOptions) *asyncCoordinator {
	ctx, cancel := context.WithCancel(ctx)
	coordinator := &asyncCoordinator{
		ctx:     ctx,
		cancel:  cancel,
		opts:    opts,
		results: make(chan boundaryResult),
	}
	if opts.concurrency > 0 {
		coordinator.sem = make(chan struct{}, opts.concurrency)
	}
	return coordinator
}

// start launches one boundary. run settles the boundary and renders its
// replacement; it reports present=false without an error only for a cancelled
// request, where the committed fallback is the final content and nobody is left
// to read anything else.
// boundaryCtx bounds this boundary's work. It is the render's context for an
// ordinary boundary, and a per-delivery one for a boundary nested inside a live
// subtree, so a superseded delivery's work stops rather than racing the
// replacement that reuses its placeholder.
func (c *asyncCoordinator) start(boundaryCtx context.Context, run func(ctx context.Context) (Content, bool, error)) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if c.sem != nil {
			select {
			case c.sem <- struct{}{}:
				defer func() { <-c.sem }()
			case <-boundaryCtx.Done():
				return
			}
		}
		ctx := boundaryCtx
		if c.opts.timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.opts.timeout)
			defer cancel()
		}
		content, present, err := run(ctx)
		if err == nil && !present {
			return
		}
		if boundaryCtx.Err() != nil {
			// Superseded or cancelled while rendering. Its placeholder either
			// no longer exists or now belongs to a later delivery, so sending
			// this would overwrite fresher content with staler content.
			return
		}
		select {
		case c.results <- boundaryResult{content: content, present: present, err: err}:
		case <-c.ctx.Done():
		}
	}()
}

// wait closes the result channel once every boundary, including boundaries
// nested inside a completed one, has settled. A nested boundary registers
// before its parent finishes, so the counter cannot reach zero early.
func (c *asyncCoordinator) wait() {
	go func() {
		c.wg.Wait()
		close(c.results)
	}()
}

// stop cancels remaining request-owned work. It does not wait: boundary
// goroutines write only their own buffers, and an external that ignores its
// context must not be able to block the handler's return.
func (c *asyncCoordinator) stop() { c.cancel() }

// startStream launches a live subscription. Unlike start, run may emit many
// times for one boundary: it calls emit once per delivery and keeps going until
// its source ends, emit reports that nobody is reading, or ctx is cancelled.
//
// Two things start does are deliberately skipped here. The boundary timeout does
// not apply, because a live source is allowed to be quiet for as long as its
// data is quiet, and a deadline would end a healthy subscription. The
// concurrency limit does not apply either: it bounds work that finishes, and a
// subscription that holds a slot for the life of the screen would starve every
// await boundary behind it.
// emitSignal is the second exit: a signal reaches the caller without being a
// delivery and without ending anything, so it needs a path of its own rather
// than a field on Content.
func (c *asyncCoordinator) startStream(run func(ctx context.Context, emit func(Content) bool, emitSignal func(Signal) bool) error) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		emit := func(content Content) bool {
			select {
			case c.results <- boundaryResult{content: content, present: true}:
				return true
			case <-c.ctx.Done():
				return false
			}
		}
		emitSignal := func(signal Signal) bool {
			select {
			case c.results <- boundaryResult{signal: &signal}:
				return true
			case <-c.ctx.Done():
				return false
			}
		}
		if err := run(c.ctx, emit, emitSignal); err != nil {
			select {
			case c.results <- boundaryResult{err: err}:
			case <-c.ctx.Done():
			}
		}
	}()
}

// live reports whether this render keeps live subscriptions open. The document
// entries leave it false, so a live boundary contributes its first delivery like
// a settled await boundary and then stops watching; that is what lets a document
// response finish instead of staying open for the life of the screen.
func (c *asyncCoordinator) liveMode() bool { return c.opts.live }
