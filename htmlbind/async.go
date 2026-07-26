package htmlbind

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
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

// normalizeAsyncError maps a Go error to the value a recover clause sees.
func normalizeAsyncError(err error) AsyncError {
	var public PublicError
	if errors.As(err, &public) {
		return public.PublicError()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return AsyncError{Code: ErrorCodeTimeout, Retryable: true, Timeout: true}
	}
	return AsyncError{Code: ErrorCodeInternal}
}

// panicError carries a recovered panic so it travels the same path as a
// returned error.
type panicError struct{ value any }

func (e *panicError) Error() string {
	return fmt.Sprintf("htmlbind: panic in async external: %v", e.value)
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

// WriteTo writes the content as an inert template element followed by the
// marker that commits it. It never emits script, so a completion needs no CSP
// nonce.
//
// The trailing marker is what makes the swap safe. An HTML parser inserts an
// element when it reads the start tag, so a runtime that reacted to the
// template's insertion could read a template whose content had not arrived yet
// and replace the placeholder with nothing. The marker comes after the closing
// tag in the byte stream, so by the time it exists the template is complete,
// however the bytes were chunked.
func (c Content) WriteTo(w io.Writer) (int64, error) {
	counter := &countingWriter{w: w}
	_, err := io.WriteString(counter, `<template data-tb-boundary="`+c.BoundaryID+`">`)
	if err == nil {
		_, err = counter.Write(c.HTML)
	}
	if err == nil {
		_, err = io.WriteString(counter, `</template><tb-apply for="`+c.BoundaryID+`"></tb-apply>`)
	}
	return counter.n, err
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	written, err := c.w.Write(p)
	c.n += int64(written)
	return written, err
}

// boundaryRuntime is the fixed update script the async render entries prepend to
// the merged head. It is trusted runtime code, not generated per component, so a
// completion never has to carry inline script of its own and one head script
// covers a whole document.
//
// It is driven by the tb-apply marker Content.WriteTo emits rather than by
// watching for templates, so a swap can only happen once the template it reads
// is closed. The marker's connected callback runs while the document is still
// parsing, so the swap is as prompt as an inline script would be.
const boundaryRuntime = `<script>(function(){` +
	`customElements.define("tb-apply",class extends HTMLElement{connectedCallback(){` +
	`var id=this.getAttribute("for");this.remove();` +
	`var t=document.querySelector('template[data-tb-boundary="'+id+'"]');if(!t)return;` +
	`var h=document.getElementById(id);if(h){h.replaceWith(t.content);}t.remove();` +
	`}});})();</script>`

// asyncCoordinator owns every boundary opened during one render. Boundary work
// runs in its own goroutine and renders into its own buffer; only the ranging
// caller and the initial pass ever write the response.
type asyncCoordinator struct {
	ctx    context.Context
	cancel context.CancelFunc
	opts   *renderOptions

	mu      sync.Mutex
	counter int

	wg      sync.WaitGroup
	results chan boundaryResult
	// sem bounds simultaneously running boundary work when a limit is set.
	sem chan struct{}
}

// boundaryResult is one settled boundary, or a render failure that ends the
// sequence. A boundary whose clause omits recover reports neither.
type boundaryResult struct {
	content Content
	present bool
	err     error
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

// nextID returns a placeholder identifier unique within this render. Boundary
// IDs from every chain member come from this one namespace, so a layout
// boundary and a page boundary can never collide.
func (c *asyncCoordinator) nextID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counter++
	return "tb-" + strconv.Itoa(c.counter)
}

// start launches one boundary. run settles the boundary and renders its
// replacement; it reports present=false when the clause omitted recover and the
// bindings failed, which leaves the committed fallback in place.
func (c *asyncCoordinator) start(run func(ctx context.Context) (Content, bool, error)) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if c.sem != nil {
			select {
			case c.sem <- struct{}{}:
				defer func() { <-c.sem }()
			case <-c.ctx.Done():
				return
			}
		}
		ctx := c.ctx
		if c.opts.timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.opts.timeout)
			defer cancel()
		}
		content, present, err := run(ctx)
		if err == nil && !present {
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
