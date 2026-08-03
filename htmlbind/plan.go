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
	"strconv"
	"time"
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
	// write HTML, one entry per contributed tag. They merge into the shell head
	// before any body byte.
	Head []string
	// Boundary describes this component as a partial update boundary. It is
	// set only for a component that can be a chain member and renders exactly
	// one root element; a boundary only becomes an instance when the component
	// is actually rendered as a chain member.
	Boundary *Boundary[P]
	// HeadSources names the component that declared each Head entry, in the same
	// order and with the same length. It exists so a caller that has to reject a
	// contribution can say which component to change, instead of printing head
	// markup a reader then has to grep for.
	//
	// It is generated data, so reading it costs nothing and needs no reflection.
	// It is nil for a component with no contribution, which is most of them.
	HeadSources []string
	// Assets names the static files this component and everything it calls
	// require, deduplicated by identity. Generation computes it over the same
	// call graph as Head, from the same declarations, and leaves it nil for a
	// component requiring none.
	//
	// It is what Head cannot be: a head entry is markup, and a caller needs an
	// identity it can compare, refuse, or preload. See Fragment.Assets.
	Assets []Asset
	// Vary names the request properties this component's output depends on,
	// declared by whoever registered the builtin elements it reaches. It is nil
	// for a component depending on none, which is most of them. See
	// Fragment.Vary.
	Vary []string
	// HasAwaitBlock reports whether this component, or any component it calls,
	// owns an await boundary. Generation computes it over the call graph, so a
	// component that only calls an async one still reports true.
	//
	// It exists so a caller can decide before rendering whether a response needs
	// the client runtime that applies settled boundaries, instead of that
	// decision being made for it inside the render entry points.
	HasAwaitBlock bool
	// HasLiveBlock reports whether this component, or any component it calls,
	// owns a live boundary. It is computed over the call graph exactly as
	// HasAwaitBlock is.
	//
	// A live boundary is also an await boundary as far as the client runtime is
	// concerned, so HasAwaitBlock is true wherever this is. The separate flag
	// exists because a caller decides two different things: whether a response
	// needs the runtime that applies boundaries, and whether this screen has
	// anything that will keep updating after the document finishes.
	HasLiveBlock bool
	// Slots returns the fragments this component's parameters carry, in
	// declaration order. Generation emits it for a component with an html
	// parameter and leaves it nil for every other, which is most of them.
	//
	// It exists because a slot argument is a whole component the binder cannot
	// otherwise see: Bind copies this plan's own head, and a fragment arriving
	// inside a caller's parameter struct is not reachable without reflection. A
	// component library's whole shape is a component supplied through a slot, so
	// without this accessor its styles are dropped — and dropped before the guard
	// that exists to refuse an undeliverable contribution ever hears about them,
	// which makes the guard silent for exactly the case it was built for.
	//
	// An absent optional slot yields an absent Fragment, which contributes
	// nothing.
	Slots func(P) []Fragment
	// Check rejects parameters this component cannot render, before it writes
	// anything. Generation emits it for a required async parameter, whose
	// absence has to be reported while the response can still carry an error
	// status rather than a half written document.
	//
	// It is nil for a component with nothing to check, which is most of them.
	Check func(P) error
	// Ops is the instruction list executed in order.
	Ops []Op[P]
	// Cache is set for a component declared with the cache annotation. It is
	// consulted only when the caller supplied a store through WithCache, so the
	// same generated code runs cached or uncached.
	Cache *CachePolicy[P]
}

// Exec runs the plan against params.
func (p *Plan[P]) Exec(r *Renderer, params P) error {
	// Checked before the first byte of this component. A chain member is also
	// checked earlier still, when the chain is assembled, so the common page
	// and layout shape fails with nothing written at all; running the same pure
	// predicate twice is cheaper than tracking which values were already seen.
	if p.Check != nil {
		if err := p.Check(params); err != nil {
			return err
		}
	}
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
	head        []string
	headSources []string
	assets      []Asset
	vary        []string
	boundary    *boundary
	hasAwait    bool
	hasLive     bool
	validate    func() error
	render      func(*Renderer) error
}

// Bind pairs a plan with parameters, producing the value a slot accepts.
func Bind[P any](plan *Plan[P], params P) Fragment {
	fragment := Fragment{
		head:        plan.Head,
		headSources: plan.HeadSources,
		assets:      plan.Assets,
		vary:        plan.Vary,
		boundary:    bindBoundary(plan.Boundary, params),
		hasAwait:    plan.HasAwaitBlock,
		hasLive:     plan.HasLiveBlock,
		render:      func(r *Renderer) error { return plan.Exec(r, params) },
	}
	if plan.Check != nil {
		fragment.validate = func() error { return plan.Check(params) }
	}
	return foldSlots(fragment, plan.Slots, params)
}

// foldSlots merges what the fragments in a parameter struct contribute into the
// value the binder returns: their head, and whether rendering will open an await
// or a live boundary.
//
// A component with no slots has a nil accessor and returns unchanged, so it
// costs nothing and needs no reflection.
func foldSlots[P any](fragment Fragment, slots func(P) []Fragment, params P) Fragment {
	if slots == nil {
		return fragment
	}
	for _, slot := range slots(params) {
		if !slot.Present() {
			continue
		}
		fragment.hasAwait = fragment.hasAwait || slot.hasAwait
		fragment.hasLive = fragment.hasLive || slot.hasLive
		fragment.head, fragment.headSources = appendHead(fragment.head, fragment.headSources, slot.head, slot.headSources)
		// Whatever is walked for head has to be walked here too, or the same
		// hole reopens one layer down: a slot-supplied component's script would
		// go unrequired for exactly the composition the head walk was added for.
		fragment.assets = appendAssets(fragment.assets, slot.assets)
		fragment.vary = appendVary(fragment.vary, slot.vary)
	}
	return fragment
}

// appendHead adds contributions to a list, dropping tags already in it.
//
// The result is a fresh slice whenever anything is added, because the list it
// starts from is the plan's own and a plan is shared by every render.
func appendHead(head, sources, addHead, addSources []string) ([]string, []string) {
	if len(addHead) == 0 {
		return head, sources
	}
	seen := make(map[string]bool, len(head))
	for _, tag := range head {
		seen[tag] = true
	}
	// HeadSources is the second view of one list, so it is only meaningful when
	// it lines up entry for entry. A plan with no sources contributes none, and
	// the pair stays either both empty or both the same length.
	grown := append([]string(nil), head...)
	grownSources := append([]string(nil), sources...)
	for len(grownSources) < len(head) {
		grownSources = append(grownSources, "")
	}
	for i, tag := range addHead {
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		grown = append(grown, tag)
		if i < len(addSources) {
			grownSources = append(grownSources, addSources[i])
		} else {
			grownSources = append(grownSources, "")
		}
	}
	if len(grown) == len(head) {
		return head, sources
	}
	return grown, grownSources
}

// Validate runs the fragment's parameter check without rendering. Chain
// assembly calls it so a chain built from unrenderable parameters fails before
// any member writes.
func (f Fragment) Validate() error {
	if f.validate == nil {
		return nil
	}
	return f.validate()
}

// Present reports whether the fragment carries content. An absent optional
// slot renders its default instead.
func (f Fragment) Present() bool { return f.render != nil }

// Head returns the fragment's head contributions, one entry per tag: its own,
// plus those of every component supplied to it through a slot parameter.
//
// The slot half matters because a component library's whole shape is a
// component handed in through a slot. Reporting only the outer component's
// contributions would drop a library's stylesheet, and drop it before a caller
// that refuses an undeliverable contribution could ever see it.
func (f Fragment) Head() []string { return f.head }

// HeadSources names the component that declared each Head entry, in the same
// order and with the same length. Head and HeadSources are two views of one
// list, so index i of either describes the same contributed tag.
//
// A caller that cannot deliver a head contribution uses it to report which
// component to change. The merged head returned by MergeHead has no matching
// source list, because deduplication drops entries: ask a member for its own.
func (f Fragment) HeadSources() []string { return f.headSources }

// HasAwaitBlock reports whether rendering this fragment can open an await
// boundary, so a caller knows whether a response will need the client runtime
// that applies settled boundaries. Reading it renders nothing.
//
// A fragment passed in through a slot parameter is counted, because generation
// emits the accessor that reaches it. A fragment a caller holds and has not yet
// bound in is its own to union, which HasAwaitBlock over a chain does for the
// ordinary document, layout, and page shape.
func (f Fragment) HasAwaitBlock() bool { return f.hasAwait }

// HasLiveBlock reports whether rendering this fragment can open a live
// boundary: a region the server keeps re-rendering after the document has
// finished. It follows the same rules as HasAwaitBlock, including how a
// fragment arriving through a slot parameter is counted.
//
// A caller reads it to decide whether this screen is worth a live request at
// all. A document whose render owns no live boundary will never produce another
// delivery, so asking for one costs a page execution and returns nothing.
func (f Fragment) HasLiveBlock() bool { return f.hasLive }

// Renderer is the coordinator walking plans. It owns the output stream and the
// merged head, so instructions never touch either directly.
type Renderer struct {
	w    io.Writer
	head []string
	// collect is nil for an ordinary render, which is what keeps the bytes of
	// an unchanged template identical to before update support existed.
	collect *collector
	// opts holds the caller-supplied render options. It is never nil.
	opts *renderOptions
	// async is set only by the streaming render entries. When it is nil an
	// await boundary blocks and renders its settled subtree in place.
	async *asyncCoordinator
	// idPrefix and idCount name boundary placeholders by their position in the
	// render tree rather than by when they happened to be allocated.
	//
	// Each boundary's subtree is its own namespace, so rendering that subtree
	// again — which is what a live boundary does on every delivery — hands out
	// the same identifiers instead of new ones. Without that a long-lived
	// subscription would mint a placeholder per delivery forever, and a client
	// would accumulate ones nothing will ever replace.
	//
	// It also makes a nested boundary's id independent of the order sibling
	// boundaries happen to settle in, which is what lets the same chain,
	// executed again for a reconnect, address the placeholders already on
	// screen.
	//
	// The counter is only ever touched by the one goroutine rendering its
	// subtree, so it needs no lock.
	idPrefix string
	idCount  *int
	// boundaryCtx bounds work started while rendering this subtree. A live
	// boundary gives each delivery its own, so the previous delivery's nested
	// boundaries are cancelled before their placeholders are reused.
	boundaryCtx context.Context
}

// nextBoundaryID allocates the next placeholder identifier in this subtree's
// namespace.
func (r *Renderer) nextBoundaryID() string {
	if r.idCount == nil {
		count := 0
		r.idCount = &count
	}
	*r.idCount++
	prefix := r.idPrefix
	if prefix == "" {
		prefix = r.boundaryPrefix()
	}
	return prefix + "-" + strconv.Itoa(*r.idCount)
}

// boundaryPrefix names the placeholder element and the root identifier
// namespace. A nested boundary inherits its parent's id instead, so this is
// consulted once per render tree.
func (r *Renderer) boundaryPrefix() string {
	if r.opts != nil && r.opts.boundaryPrefix != "" {
		return r.opts.boundaryPrefix
	}
	return DefaultBoundaryPrefix
}

// boundaryElement is the placeholder tag name, derived from the same prefix as
// everything else the protocol puts in the document.
func (r *Renderer) boundaryElement() string {
	return r.boundaryPrefix() + "-boundary"
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
// head, options, boundary coordinator, and identifier namespace.
func (r *Renderer) buffered(w io.Writer) *Renderer {
	return &Renderer{w: w, head: r.head, opts: r.opts, async: r.async,
		idPrefix: r.idPrefix, idCount: r.idCount, boundaryCtx: r.boundaryCtx}
}

// subtree returns a renderer for one boundary's contents: it writes into w and
// opens a fresh identifier namespace under that boundary's id, so the same
// subtree rendered again produces the same placeholders.
//
// ctx bounds whatever that subtree starts. A live boundary passes a new one per
// delivery so the previous delivery's nested boundaries stop before their
// identifiers are handed out again; a settle-once boundary passes its own.
func (r *Renderer) subtree(w io.Writer, id string, ctx context.Context) *Renderer {
	count := 0
	return &Renderer{w: w, head: r.head, opts: r.opts, async: r.async,
		idPrefix: id, idCount: &count, boundaryCtx: ctx}
}

// boundaryContext returns the context bounding work this subtree starts.
func (r *Renderer) boundaryContext() context.Context {
	if r.boundaryCtx != nil {
		return r.boundaryCtx
	}
	return r.context()
}

// boundaryTimeout returns the configured per-boundary deadline, or zero when
// the caller set none.
func (r *Renderer) boundaryTimeout() time.Duration {
	if r.opts == nil {
		return 0
	}
	return r.opts.timeout
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
	if r.collect != nil {
		r.collect.write(value)
	}
	_, err := io.WriteString(r.w, value)
	return err
}

// MergedHead returns the head contributions collected for this render.
func (r *Renderer) MergedHead() []string { return r.head }
