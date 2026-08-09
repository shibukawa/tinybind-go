package htmlbind

import (
	"context"
	"io"
	"iter"
	"strconv"
)

// Collector observes one chain render: every byte an instruction writes, and
// each chain member's boundary opening and closing around its own output. It
// is the seam the update machinery hangs from — validators and captured
// subtrees are derived by the implementation, so a render that collects
// nothing links none of the hashing that derivation needs.
//
// A collector is driven by the one goroutine walking the plan, so an
// implementation needs no locking of its own.
type Collector interface {
	// Begin starts one render, carrying the validator tag the render options
	// resolved. It is called once, before anything else.
	//
	// It used to carry the placeholder element name too, so a decomposing
	// observer could write the same shape a progressive render writes. The two
	// shapes are no longer the same: a hole has no content and is written as a
	// template, while an await boundary brackets a visible fallback and is
	// written as a comment pair. Neither is nameable by a prefix, so nothing is
	// left to pass.
	Begin(validatorTag string)
	// Write observes one instruction's output, after escaping.
	Write(value string)
	// Open enters the boundary of one chain member: its instance ID, the
	// component's declaration identity, the instance attribute its root
	// element will carry, the canonical encoding of its declared inputs, and
	// the address of the static half its values are walked against.
	Open(id, componentID, attr, input, sequence string)
	// Close leaves the innermost open boundary.
	Close()
	// TakePending consumes the boundary whose root element has not yet written
	// its instance attribute, returning that attribute and the instance ID.
	// Only the boundary's own root consumes it, so an ordinary component
	// nested inside cannot claim its parent's ID.
	TakePending() (attr, id string, ok bool)
	// Slot brackets one instruction's output, so what it wrote can be separated
	// from the static text around it. begin opens and end closes.
	Slot(begin bool)
	// Choice records what the value stream needs in order to walk the sequence
	// tree: which branch a conditional took, how many times a loop ran, and
	// whether a called component opened a boundary or rendered inline.
	Choice(value string)
}

// CollectChain renders like RenderChain with collect observing the render, and
// returns the merged head. Every chain member declaring a boundary opens one
// around its own output, so the observer sees the layout chain rather than
// every component call.
//
// Collecting emits the instance attribute on each boundary's root element, so
// the output differs from RenderChain by exactly those attributes.
func CollectChain(w io.Writer, collect Collector, wrappers []Wrapper, leaf Fragment, options ...Option) ([]string, error) {
	opts := newRenderOptions(options)
	if collect != nil {
		collect.Begin(opts.validatorTag)
	}
	composed, head, err := assemble(collect, wrappers, leaf)
	if err != nil {
		return nil, err
	}
	head, err = mergeCallerHead(head, opts.head)
	if err != nil {
		return nil, err
	}
	if err := composed(&Renderer{w: w, sw: stringWriterOf(w), head: head, opts: opts, collect: collect}); err != nil {
		return nil, err
	}
	return head, nil
}

// CollectChainAsync is CollectChain for the streaming path: the initial pass
// renders with every await boundary's fallback in place, and the sequence then
// yields each boundary as it settles, exactly as RenderChainAsync does.
//
// rendered runs after the initial pass commits and before the first
// completion, which is the moment the observer's state describes the whole
// document. Returning false ends the sequence without waiting for the
// outstanding boundaries. A nil rendered is allowed and skips the call.
func CollectChainAsync(ctx context.Context, w io.Writer, collect Collector, rendered func() bool, wrappers []Wrapper, leaf Fragment, options ...Option) iter.Seq2[Content, error] {
	return streamChain(ctx, w, collect, rendered, wrappers, leaf, options)
}

// memberFragment wraps one chain member so its boundary opens before its first
// byte and closes after its last, which is what makes a frame validator cover
// the member's own markup only.
func memberFragment(member Fragment, decl *boundary, index int) Fragment {
	if decl == nil {
		return member
	}
	// A boundary that names its own instance keeps that name wherever it
	// renders. Only a component with no name of its own is numbered by its chain
	// position, which is the case the numbering exists for: a layout is the same
	// member of the same chain whatever its parameters say.
	id := decl.instance
	if id == "" {
		id = "c" + strconv.Itoa(index)
	}
	return Fragment{
		head:          member.head,
		hasAwait:      member.hasAwait,
		hasLive:       member.hasLive,
		sequence:      member.sequence,
		opensBoundary: true,
		render: func(r *Renderer) error {
			if r.collect == nil {
				return member.render(r)
			}
			r.collect.Open(id, decl.componentID, decl.attr, decl.input(),
				member.sequenceAddress())
			if err := member.render(r); err != nil {
				return err
			}
			r.collect.Close()
			return nil
		},
	}
}
