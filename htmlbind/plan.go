// Package htmlbind is the rendering runtime for generated HTML templates.
//
// Generation produces one immutable Plan per component: an ordered instruction
// list typed by that component's parameter struct. A shared coordinator walks
// the plan and writes HTML, so composition concerns such as slots, chain
// nesting, and document head merging live here rather than in generated code.
//
// The package depends on the standard library only, and never on net/http, so
// generated code stays usable on TinyGo and WebAssembly targets. Response
// concerns such as content type and content encoding belong to the caller.
package htmlbind

import (
	"bytes"
	"context"
	"io"
)

// Op is one instruction of a render plan. P is the parameter struct of the
// component the instruction belongs to, so every step stays statically typed
// and no reflection is needed.
type Op[P any] interface {
	// Exec writes this instruction's output. Implementations are immutable and
	// safe to share across goroutines.
	Exec(r *Renderer, params P) error
}

// Plan is a component compiled to instructions. A plan is built once at
// package initialization and shared by every render.
type Plan[P any] struct {
	// Head holds this component's document head contributions as ready to
	// write HTML. They merge into the shell head before any body byte.
	Head []string
	// Ops is the instruction list executed in order.
	Ops []Op[P]
	// Cache is set for a component declared with the cache annotation. It is
	// consulted only when the caller supplied a store through WithCache, so the
	// same generated code runs cached or uncached.
	Cache *CachePolicy[P]
}

// Exec runs the plan against params.
func (p *Plan[P]) Exec(r *Renderer, params P) error {
	if p.Cache == nil || r.opts == nil || r.opts.cache == nil {
		return execOps(r, p.Ops, params)
	}
	return p.execCached(r, params)
}

// execCached writes a stored rendering when one is current, and otherwise
// renders into an isolated buffer and publishes it. Publishing after the whole
// subtree renders is what keeps a failed render from storing partial output.
func (p *Plan[P]) execCached(r *Renderer, params P) error {
	key := p.Cache.cacheKey(params)
	ctx := r.context()
	if cached, ok := r.opts.cache.Get(ctx, key); ok {
		_, err := r.w.Write(cached)
		return err
	}
	var buffer bytes.Buffer
	// A cached subtree renders without the boundary coordinator. Generation
	// rejects an await boundary inside a cached component, so this only makes
	// the runtime's behavior match that rule instead of storing a placeholder.
	sub := &Renderer{w: &buffer, head: r.head, opts: r.opts}
	if err := execOps(sub, p.Ops, params); err != nil {
		return err
	}
	rendered := buffer.Bytes()
	r.opts.cache.Set(ctx, key, rendered, p.Cache.TTL)
	_, err := r.w.Write(rendered)
	return err
}

func execOps[P any](r *Renderer, ops []Op[P], params P) error {
	for _, op := range ops {
		if err := op.Exec(r, params); err != nil {
			return err
		}
	}
	return nil
}

// Fragment is a component with its parameters already bound. It is the runtime
// value behind the html template type, so a slot argument can be passed
// between components and across template files without either side naming the
// other's parameter struct.
//
// The zero Fragment is absent, which is how an optional slot with no argument
// is represented.
type Fragment struct {
	head   []string
	render func(*Renderer) error
}

// Bind pairs a plan with parameters, producing the value a slot accepts.
func Bind[P any](plan *Plan[P], params P) Fragment {
	return Fragment{
		head:   plan.Head,
		render: func(r *Renderer) error { return plan.Exec(r, params) },
	}
}

// Present reports whether the fragment carries content. An absent optional
// slot renders its default instead.
func (f Fragment) Present() bool { return f.render != nil }

// Head returns the fragment's own head contributions.
func (f Fragment) Head() []string { return f.head }

// Renderer is the coordinator walking plans. It owns the output stream and the
// merged head, so instructions never touch either directly.
type Renderer struct {
	w    io.Writer
	head []string
	// opts holds the caller-supplied render options. It is never nil.
	opts *renderOptions
	// async is set only by the streaming render entries. When it is nil an
	// await boundary blocks and renders its settled subtree in place.
	async *asyncCoordinator
}

// context returns the context this render runs under. The async entries take
// one directly; the synchronous entries accept one through WithContext so a
// shared cache store and a blocking await still see request cancellation.
func (r *Renderer) context() context.Context {
	if r.async != nil {
		return r.async.ctx
	}
	if r.opts != nil && r.opts.ctx != nil {
		return r.opts.ctx
	}
	return context.Background()
}

// buffered returns a renderer writing into w and sharing this render's merged
// head, options, and boundary coordinator. A boundary opened while rendering a
// completed boundary's subtree therefore streams like any other.
func (r *Renderer) buffered(w io.Writer) *Renderer {
	return &Renderer{w: w, head: r.head, opts: r.opts, async: r.async}
}

// reportError hands a failure to the caller's reporter. It is how a normalized
// data:async-render-error stays observable even when a recover subtree renders.
func (r *Renderer) reportError(err error) {
	if r.opts != nil && r.opts.report != nil {
		r.opts.report(err)
	}
}

// Write emits raw bytes. Instructions call it after applying their own
// context-appropriate escaping.
func (r *Renderer) Write(value string) error {
	_, err := io.WriteString(r.w, value)
	return err
}

// MergedHead returns the head contributions collected for this render.
func (r *Renderer) MergedHead() []string { return r.head }
