package htmlbind

import (
	"context"
	"io"
	"iter"
)

// DeltaRecord is one item of a streamed delta: either a boundary the browser
// must install, or an await boundary that settled after the initial pass.
//
// The two travel on one sequence because they are the same event to a client: a
// region of the page is ready. Splitting them in the protocol would mean two
// consumers applying markup to one document.
type DeltaRecord struct {
	// Operation names a boundary the comparison produced. Its HTML is empty for
	// an unchanged boundary, which still carries its validator so the client can
	// rebuild the whole manifest from what it received.
	Operation *Operation
	// Frame is the validator of the boundary Operation names.
	Frame string
	// Completion is an await boundary that settled, addressed by the
	// placeholder written during the initial pass rather than by an instance id.
	Completion *Content
}

// RenderDeltaStream renders the chain and yields the boundaries that changed,
// then each await boundary as it settles.
//
// It is the streaming counterpart of RenderDelta, and the difference is worth
// stating: RenderDelta blocks until every await settles and compares finished
// regions, while this yields each region with its fallback in place and follows
// with the replacements. A slow dependency delays only its own region.
//
// The document markup outside every boundary is discarded, because a delta
// reuses the browser's existing document shell.
func RenderDeltaStream(ctx context.Context, key []byte, known Manifest, wrappers []Wrapper, leaf Fragment, options ...Option) iter.Seq2[DeltaRecord, error] {
	return func(yield func(DeltaRecord, error) bool) {
		collect := &collector{key: key, capture: true}
		coordinator, err := initialPass(ctx, collect, wrappers, leaf, options)
		if err != nil {
			yield(DeltaRecord{}, err)
			return
		}
		defer coordinator.stop()

		manifest := collect.manifest
		frames := map[string]string{}
		for _, instance := range manifest.Instances {
			frames[instance.ID] = instance.FrameValidator
		}
		sent := map[string]bool{}
		for _, operation := range operations(manifest, known, collect.contents) {
			op := operation
			sent[op.InstanceID] = true
			if !yield(DeltaRecord{Operation: &op, Frame: frames[op.InstanceID]}, nil) {
				return
			}
		}
		for _, instance := range manifest.Instances {
			if sent[instance.ID] {
				continue
			}
			op := Operation{InstanceID: instance.ID}
			if !yield(DeltaRecord{Operation: &op, Frame: instance.FrameValidator}, nil) {
				return
			}
		}

		// Only now do completions matter: a replacement addresses a placeholder
		// inside a region the client has already installed.
		coordinator.wait()
		for {
			select {
			case result, open := <-coordinator.results:
				if !open {
					return
				}
				if result.err != nil {
					yield(DeltaRecord{}, result.err)
					return
				}
				completion := result.content
				if !yield(DeltaRecord{Completion: &completion}, nil) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}

// initialPass renders the chain with a collector and an async coordinator
// attached, so one render produces both the boundary comparison and the
// pending await boundaries.
//
// Output goes nowhere: a delta reuses the browser's document, so only the
// captured boundary subtrees and the completions are wanted.
func initialPass(ctx context.Context, collect *collector, wrappers []Wrapper, leaf Fragment, options []Option) (*asyncCoordinator, error) {
	if err := validateChain(wrappers, leaf); err != nil {
		return nil, err
	}
	coordinator := newAsyncCoordinator(ctx, newRenderOptions(options))
	head, err := mergeCallerHead(MergeHead(wrappers, leaf), coordinator.opts.head)
	if err != nil {
		coordinator.stop()
		return nil, err
	}
	next := memberFragment(leaf, leaf.boundary, len(wrappers))
	for i := len(wrappers) - 1; i >= 0; i-- {
		wrapper, inner, index := wrappers[i], next, i
		child := Fragment{render: func(r *Renderer) error { return wrapper.render(r, inner) }}
		next = memberFragment(child, wrapper.boundary, index)
	}
	renderer := &Renderer{w: io.Discard, head: head, opts: coordinator.opts, async: coordinator, collect: collect}
	if err := next.render(renderer); err != nil {
		coordinator.stop()
		return nil, err
	}
	return coordinator, nil
}

// DeltaStreamHead is the merged head of a chain, which a streamed delta sends
// before any region so a newly reachable component's stylesheet is installed
// before the markup that needs it.
func DeltaStreamHead(wrappers []Wrapper, leaf Fragment, options ...Option) ([]string, error) {
	return mergeCallerHead(MergeHead(wrappers, leaf), newRenderOptions(options).head)
}
