package delta

import (
	"context"
	"io"
	"iter"

	"github.com/shibukawa/tinybind-go/htmlbind"
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
	Completion *htmlbind.Content
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
func RenderDeltaStream(ctx context.Context, key []byte, known Manifest, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) iter.Seq2[DeltaRecord, error] {
	return func(yield func(DeltaRecord, error) bool) {
		collect := &collector{key: key, capture: true}
		// The boundary records go out as soon as the initial pass commits, so a
		// slow await boundary delays only its own completion. Completions only
		// matter after them anyway: a replacement addresses a placeholder
		// inside a region the client has already installed.
		rendered := func() bool {
			manifest := collect.manifest
			frames := map[string]string{}
			for _, instance := range manifest.Instances {
				frames[instance.ID] = instance.FrameValidator
			}
			sent := map[string]bool{}
			for _, operation := range operations(manifest, known, collect.contents, collect.children) {
				op := operation
				sent[op.InstanceID] = true
				if !yield(DeltaRecord{Operation: &op, Frame: frames[op.InstanceID]}, nil) {
					return false
				}
			}
			for _, instance := range manifest.Instances {
				if sent[instance.ID] {
					continue
				}
				op := Operation{InstanceID: instance.ID}
				if !yield(DeltaRecord{Operation: &op, Frame: instance.FrameValidator}, nil) {
					return false
				}
			}
			return true
		}
		for content, err := range htmlbind.CollectChainAsync(ctx, io.Discard, collect, rendered, wrappers, leaf, options...) {
			if err != nil {
				yield(DeltaRecord{}, err)
				return
			}
			completion := content
			if !yield(DeltaRecord{Completion: &completion}, nil) {
				return
			}
		}
	}
}

// DeltaStreamHead is the merged head of a chain, which a streamed delta sends
// before any region so a newly reachable component's stylesheet is installed
// before the markup that needs it.
func DeltaStreamHead(wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) ([]string, error) {
	return htmlbind.ChainHead(wrappers, leaf, options...)
}
