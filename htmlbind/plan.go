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
	"strings"
	"sync"
	"time"
	"unicode/utf8"
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
	// DeclaresPrivate reports whether this component, or any component it calls,
	// declared its output per-reader. Generation computes it over the call graph
	// as HasAwaitBlock is, because a private component's bytes end up inside
	// whatever renders it.
	//
	// It exists so a caller can put a cache policy on the wire before the first
	// body byte. A private component four levels down renders long after the
	// header is committed, so anything computed during a render would be
	// available only on the buffered branch — and a response's cache policy would
	// then depend on whether streaming was on.
	DeclaresPrivate bool
	// DeclaresPublic reports whether this component declared its output shared.
	//
	// Unlike DeclaresPrivate it does not fold over the call graph, because the
	// declaration is an assertion about this component and what it renders rather
	// than a property it inherits from a caller. Generation refuses the assertion
	// when the call graph beneath it reaches a declared private component, so a
	// plan carrying this bit has already been checked.
	DeclaresPublic bool
	// PrivateSource names the component whose declaration set DeclaresPrivate, so
	// a caller that has to explain a private response can say which component to
	// change. It is empty when nothing declared it.
	//
	// It is the courtesy HeadSources provides for a head contribution, for the
	// same reason: the answer is useless without the position.
	PrivateSource string
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

	// sequenceOnce and sequenceMemo hold the derived static half. Derivation
	// walks the instruction list and evaluates nothing, so one plan yields one
	// tree however many times it renders — and, since the hole a nested boundary
	// leaves no longer spells the boundary prefix, one tree however that prefix
	// is named.
	sequenceOnce sync.Once
	sequenceMemo *Sequence
}

// Exec runs the plan against params.
func (p *Plan[P]) Exec(r *Renderer, params P) error { return p.exec(r, params, p.Ops) }

// exec runs one instruction list against params. A chain member hands in the
// list its assembly prepared, whose leading value bindings are already computed;
// every other caller hands in the plan's own.
//
// Unlike Check, a prepared binding is not recomputed here. Check is a pure
// predicate, so running it twice is cheaper than tracking what already ran; a
// loader is neither pure nor free, and running it twice is the duplicate fetch
// requirement:template-value-binding exists to remove.
func (p *Plan[P]) exec(r *Renderer, params P, ops []Op[P]) error {
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
		return execOps(r, ops, params)
	}
	return p.execCached(r, params, ops)
}

// execCached writes a stored rendering when one is current, and otherwise
// renders into an isolated buffer and publishes it. Publishing after the whole
// subtree renders is what keeps a failed render from storing partial output.
func (p *Plan[P]) execCached(r *Renderer, params P, ops []Op[P]) error {
	// A private component with no scope value stores nothing. An entry written
	// under an empty scope would be a shared entry wearing a private label, and
	// this is the one case where rendering normally is the whole answer.
	if p.Cache.Scoped && r.opts.cacheScope == "" {
		return execOps(r, ops, params)
	}
	key := p.Cache.cacheKey(r.opts.cacheScope, params)
	ctx := r.context()
	if cached, ok := r.opts.cache.Get(ctx, key); ok {
		_, err := r.w.Write(cached)
		return err
	}
	var buffer bytes.Buffer
	// A cached subtree renders without the boundary coordinator. Generation
	// rejects an await boundary inside a cached component, so this only makes
	// the runtime's behavior match that rule instead of storing a placeholder.
	sub := &Renderer{w: &buffer, sw: &buffer, head: r.head, opts: r.opts}
	if err := execOps(sub, ops, params); err != nil {
		return err
	}
	rendered := buffer.Bytes()
	r.opts.cache.Set(ctx, key, rendered, p.Cache.TTL)
	_, err := r.w.Write(rendered)
	return err
}

func execOps[P any](r *Renderer, ops []Op[P], params P) error {
	if r.collect == nil {
		for _, op := range ops {
			if err := op.Exec(r, params); err != nil {
				return err
			}
		}
		return nil
	}
	// A collecting render brackets every instruction whose output varies, so the
	// literal text between them is separable from the values. Control flow is not
	// bracketed: a conditional and a loop are structure the sequence tree carries,
	// and their inner instructions are bracketed on their own.
	for _, op := range ops {
		switch op.(type) {
		case staticOp[P], ifOp[P], ifCtxOp[P], componentOp[P], componentCtxOp[P], slotOp[P], slotCtxOp[P]:
			if err := op.Exec(r, params); err != nil {
				return err
			}
			continue
		}
		if _, repeats := op.(interface{ sequenceBody() []SeqNode }); repeats {
			if err := op.Exec(r, params); err != nil {
				return err
			}
			continue
		}
		r.collect.Slot(true)
		err := op.Exec(r, params)
		r.collect.Slot(false)
		if err != nil {
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
	// declaresPrivate and declaresPublic carry the two cache scope declarations.
	// They are separate bits rather than one tri-state because they fold
	// differently: private unions from everything this fragment contains, and
	// public stays where it was written.
	declaresPrivate bool
	declaresPublic  bool
	privateSource   string
	validate        func() error
	// prepare computes the leading value bindings of this fragment's plan
	// before anything is written, and returns the render to run in place of the
	// unprepared one. It is what lets a loader's failure still choose the
	// response status, per decision:value-binding-hoisting.
	//
	// It is nil for a fragment that is not a chain member, because only a chain
	// member is assembled; such a fragment computes its bindings where they run.
	prepare func(context.Context) (func(*Renderer) error, error)
	render  func(*Renderer) error
	// opensBoundary marks a fragment that opens its own boundary when it renders,
	// which a chain member does. Without it a slot holding one would be recorded
	// as an inlined component while a boundary opened inside it, and the values
	// would carry both the hole and the subtree it stands for.
	opensBoundary bool
	// sequence derives this component's static half. It closes over the plan
	// rather than holding the tree, so a fragment costs nothing until something
	// asks for the address.
	sequence func() *Sequence
}

// Bind pairs a plan with parameters, producing the value a slot accepts.
//
// Deprecated: use the Bind method on Plan. It carries no type parameter beyond
// the receiver's own, so the method form was always available; this function
// remains so no generated or hand-written caller is forced to move.
func Bind[P any](plan *Plan[P], params P) Fragment {
	return plan.Bind(params)
}

// Bind pairs a plan with parameters, producing the value a slot accepts.
func (plan *Plan[P]) Bind(params P) Fragment {
	fragment := Fragment{
		head:            plan.Head,
		headSources:     plan.HeadSources,
		assets:          plan.Assets,
		vary:            plan.Vary,
		boundary:        bindBoundary(plan.Boundary, params),
		hasAwait:        plan.HasAwaitBlock,
		hasLive:         plan.HasLiveBlock,
		declaresPrivate: plan.DeclaresPrivate,
		declaresPublic:  plan.DeclaresPublic,
		privateSource:   plan.PrivateSource,
		render:          func(r *Renderer) error { return plan.Exec(r, params) },
		sequence:        plan.Sequence,
	}
	if plan.Check != nil {
		fragment.validate = func() error { return plan.Check(params) }
	}
	fragment.prepare = func(ctx context.Context) (func(*Renderer) error, error) {
		prepared, err := prepareOps(ctx, params, plan.Ops)
		if err != nil {
			return nil, err
		}
		return func(r *Renderer) error { return plan.exec(r, params, prepared) }, nil
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
		// Private unions and public does not. A component handed in through a
		// slot renders inside this one, so its per-reader output makes this one
		// per-reader too; its assertion that it is shared says nothing about the
		// markup wrapped around it.
		if slot.declaresPrivate && !fragment.declaresPrivate {
			fragment.declaresPrivate, fragment.privateSource = true, slot.privateSource
		}
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

// sequenceAddress names this fragment's static half, or empty when it has none.
func (f Fragment) sequenceAddress() string {
	if f.sequence == nil {
		return ""
	}
	return f.sequence().Address
}

// InstanceID returns the update-boundary instance this fragment renders as, and
// empty when it renders as no addressable boundary.
//
// It is set for a component that names its own instance — a reloadable one,
// whose id an author writes at the call site. A chain member is numbered by its
// position instead, and reports nothing here because that number is decided by
// the chain rather than by the fragment.
//
// A redraw reads it to check that the component it just bound is addressable at
// the id the request asked for, which generated code guarantees and a
// hand-assembled registration can get wrong.
func (f Fragment) InstanceID() string {
	if f.boundary == nil {
		return ""
	}
	return f.boundary.instance
}

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
	w io.Writer
	// sw is w's io.StringWriter face, resolved once at construction so Write
	// does not repeat the assertion per instruction. It is nil when w cannot
	// take a string directly.
	sw   io.StringWriter
	head []string
	// collect is nil for an ordinary render, which is what keeps the bytes of
	// an unchanged template identical to before update support existed.
	collect Collector
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
	// scratch is reused for the string-to-bytes conversion Write needs when the
	// writer has no WriteString. Each renderer owns its own, because boundary
	// subtrees render in their own goroutines.
	scratch []byte
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

// boundaryPrefix names the await markers and the root identifier namespace. A
// nested boundary inherits its parent's id instead, so this is consulted once
// per render tree.
func (r *Renderer) boundaryPrefix() string {
	if r.opts != nil && r.opts.boundaryPrefix != "" {
		return r.opts.boundaryPrefix
	}
	return DefaultBoundaryPrefix
}

// awaitFenceOpen and awaitFenceClose bracket an await boundary's fallback, so a
// completion replaces the range between them rather than one element.
//
// They are comments rather than the wrapper element this used to write, because
// the fallback has to be visible and has to stay where it was written, and no
// element is both:
//
//   - An unknown element in table context is foster-parented. The parser moves
//     it out to just before the table and leaves the fallback rows inside, so a
//     client replacing the placeholder writes the settled row outside the table
//     and the fallback stays in the list forever. This is the tree construction
//     algorithm, not a browser quirk, and no caller markup avoids it.
//   - A template is kept where it was written, but a template does not render
//     its content, so the fallback would be invisible until it settled — and a
//     visible fallback with no JavaScript is what this whole path is for.
//
// A comment is kept wherever it appears and renders nothing itself, which is the
// pair of properties needed. The cost is that a client walks siblings between
// two markers instead of replacing one node; see the hole placeholder in
// htmlbind/delta, which stays an element because it has no content to keep.
func awaitFenceOpen(prefix, id string) string { return "<!--" + prefix + ":" + id + "-->" }

func awaitFenceClose(prefix, id string) string { return "<!--/" + prefix + ":" + id + "-->" }

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

// subtree returns a renderer for one boundary's contents: it writes into w and
// opens a fresh identifier namespace under that boundary's id, so the same
// subtree rendered again produces the same placeholders.
//
// ctx bounds whatever that subtree starts. A live boundary passes a new one per
// delivery so the previous delivery's nested boundaries stop before their
// identifiers are handed out again; a settle-once boundary passes its own.
func (r *Renderer) subtree(w io.Writer, id string, ctx context.Context) *Renderer {
	count := 0
	return &Renderer{w: w, sw: stringWriterOf(w), head: r.head, opts: r.opts, async: r.async,
		idPrefix: id, idCount: &count, boundaryCtx: ctx}
}

// stringWriterOf resolves a writer's io.StringWriter face for the field every
// renderer constructor fills, or nil for a writer without one.
func stringWriterOf(w io.Writer) io.StringWriter {
	if sw, ok := w.(io.StringWriter); ok {
		return sw
	}
	return nil
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
		r.collect.Write(value)
	}
	if r.sw != nil {
		_, err := r.sw.WriteString(value)
		return err
	}
	// A writer without WriteString — a compressing middleware's, for one —
	// would otherwise cost one []byte copy per instruction, so the conversion
	// goes through a buffer this render keeps.
	r.scratch = append(r.scratch[:0], value...)
	_, err := r.w.Write(r.scratch)
	return err
}

// WriteEscaped emits value under Escape's rules, writing clean runs and
// entities separately so a value that needs escaping never builds an
// intermediate string.
func (r *Renderer) WriteEscaped(value string) error {
	if !strings.ContainsAny(value, `&<>"'`) {
		return r.Write(value)
	}
	start := 0
	for i, c := range value {
		var entity string
		switch c {
		case '&':
			entity = "&amp;"
		case '<':
			entity = "&lt;"
		case '>':
			entity = "&gt;"
		case '"':
			entity = "&#34;"
		case '\'':
			entity = "&#39;"
		case utf8.RuneError:
			// Escape's rune loop turns each invalid byte into the replacement
			// character. A genuine U+FFFD decodes to its own three bytes and
			// passes through inside the clean run.
			if _, width := utf8.DecodeRuneInString(value[i:]); width != 1 {
				continue
			}
			entity = "�"
		default:
			continue
		}
		if start < i {
			if err := r.Write(value[start:i]); err != nil {
				return err
			}
		}
		if err := r.Write(entity); err != nil {
			return err
		}
		start = i + 1
	}
	if start < len(value) {
		return r.Write(value[start:])
	}
	return nil
}

// writeAttr emits one attribute from its precomputed ` name="` prefix and its
// already escaped value, in pieces, so a per-render value costs no
// concatenation.
func (r *Renderer) writeAttr(prefix, value string) error {
	if err := r.Write(prefix); err != nil {
		return err
	}
	if err := r.Write(value); err != nil {
		return err
	}
	return r.Write(`"`)
}

// writeAttrEscaped is writeAttr for a value still carrying its raw characters,
// which is what the URL instructions hold after the scheme check.
func (r *Renderer) writeAttrEscaped(prefix, value string) error {
	if err := r.Write(prefix); err != nil {
		return err
	}
	if err := r.WriteEscaped(value); err != nil {
		return err
	}
	return r.Write(`"`)
}

// MergedHead returns the head contributions collected for this render.
func (r *Renderer) MergedHead() []string { return r.head }

// prepared runs this fragment's prologue and returns the fragment to render in
// its place. A fragment with no prologue, or one already prepared, is returned
// unchanged, so calling it twice is safe and costs nothing the second time.
func (f Fragment) prepared(ctx context.Context) (Fragment, error) {
	if f.prepare == nil || !f.Present() {
		return f, nil
	}
	render, err := f.prepare(ctx)
	if err != nil {
		return Fragment{}, err
	}
	f.render = render
	f.prepare = nil
	return f, nil
}
