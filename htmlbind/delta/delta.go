// Package delta compares two renders of one chain and expresses the
// difference as operations a browser applies, so a screen already showing the
// document reaches the server's fresh render without a full page load.
//
// It is the update half of htmlbind. The render half exposes only the
// observation seam — htmlbind.Collector — and everything derived from it lives
// here: manifests, keyed validators, canonical input encoding, and deltas. The
// split is what it costs, or rather does not cost: an application that only
// renders documents links none of the hashing and encoding this package needs
// to authenticate validators.
package delta

import (
	"io"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Operation is one change the browser applies to reach the server's render.
type Operation struct {
	// Kind is replace. Insert, remove, and move arrive with structural
	// boundaries; until then a structural change is expressed by replacing an
	// enclosing boundary.
	Kind string
	// InstanceID names the boundary the operation targets.
	InstanceID string
	// HTML is the boundary's own markup, including its root element, with an
	// inert placeholder where each nested boundary sits rather than that
	// boundary's bytes.
	HTML string
	// Boundaries names the nested boundaries appearing as holes in HTML.
	//
	// It is what tells a hole to fill from one to retain: an id also carrying an
	// operation in this response is replaced, and one that does not is a
	// boundary the client already holds and moves its live node into. Nothing in
	// the markup distinguishes the two, and without the list a missing fragment
	// would be indistinguishable from a truncated response.
	Boundaries []string
}

const (
	// OpReplace swaps a boundary's own markup, holes and all.
	OpReplace = "replace"
	// OpChildren says a boundary's own markup is unchanged and its nested
	// boundaries are now these, in this order.
	//
	// It carries no HTML. The client reconciles what it already holds against
	// the list: an id it holds and the list keeps stays, moving if the order
	// moved; an id the list drops is removed; an id it does not hold arrives as
	// its own operation in the same response.
	//
	// It exists because appending one row to a list is the ordinary event on a
	// live screen, and expressing it by replacing the parent costs the whole
	// list of holes — measured at 7,383 bytes to add one 76-byte row to a
	// hundred, where the list of ids costs a few hundred.
	OpChildren = "children"
)

// Delta is the result of comparing a fresh render against what the browser
// already holds.
type Delta struct {
	// Manifest is the full state after the operations apply. Unchanged
	// boundaries appear here without an operation, which is the entire point.
	Manifest Manifest
	// Operations are ordered outermost first, so a target always exists by the
	// time a later operation addresses something inside it.
	Operations []Operation
	// Head is the merged head of the new composition. A delta reuses the live
	// document shell, so a component appearing for the first time has no link
	// tag installed; the client diffs this against the head it already has and
	// waits for new stylesheets before applying content.
	Head []string
}

// RenderDelta renders the chain and returns only the boundaries whose markup
// differs from known.
//
// The render always runs: a component may read anything, so equal inputs do not
// prove equal output. Only transmission is skipped, never execution. An empty
// known manifest is not an error and yields every boundary, which is how a
// freshly loaded page gets its first comparable state.
//
// The document markup outside every boundary is discarded, because a delta
// reuses the browser's existing document shell.
func RenderDelta(key []byte, known Manifest, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) (Delta, error) {
	collect := &collector{key: key, capture: true}
	head, err := htmlbind.CollectChain(io.Discard, collect, wrappers, leaf, options...)
	if err != nil {
		return Delta{}, err
	}
	return Delta{
		Manifest:   collect.manifest,
		Operations: operations(collect.manifest, known, collect.contents, collect.children),
		Head:       head,
	}, nil
}

// operations sends every changed boundary as its own fragment.
//
// A parent's fragment holds a placeholder where each child sits rather than the
// child's bytes, so a descendant is no longer contained in its ancestor's
// replacement and has to be sent when it changed. The gain is the other
// direction: an unchanged child of a changed parent is sent by nobody, and the
// client moves the node it already holds into the hole — keeping the focus, the
// form values, and the media state that recreating it inside the parent would
// have destroyed.
func operations(manifest, known Manifest, contents map[string]string, children map[string][]string) []Operation {
	// A boundary the browser holds that this render did not produce has to be
	// taken off the screen, and the delta has no way to say where it was: the
	// hints carry ids and validators, not structure. Replacing the outermost
	// boundary removes it along with everything else that moved, which is the
	// documented fallback when no granular operation is available.
	//
	// Without this a shorter chain leaves the old innermost region on screen
	// whenever the boundary above it happened to render identical markup.
	forceRoot := disappeared(manifest, known)
	var ops []Operation
	for _, instance := range manifest.Instances {
		before, ok := known.Find(instance.ID)
		root := forceRoot && instance.ParentID == ""
		if ok && before.FrameValidator == instance.FrameValidator && !root {
			// The component's own markup is unchanged, so its DOM stays. If its
			// nested boundaries moved, the client is told the new order and
			// reconciles what it holds; a parent replacement would have cost
			// every hole in the list to express one insertion.
			if before.ChildrenValidator != instance.ChildrenValidator {
				ops = append(ops, Operation{
					Kind:       OpChildren,
					InstanceID: instance.ID,
					Boundaries: children[instance.ID],
				})
			}
			// An unchanged boundary is never sent, including one whose parent is
			// being replaced: its hole in that replacement is what the client
			// moves its live node into.
			continue
		}
		// A boundary absent from the known manifest is not unchanged by the test
		// above, so a newly appearing region is always sent.
		ops = append(ops, Operation{
			Kind:       OpReplace,
			InstanceID: instance.ID,
			HTML:       contents[instance.ID],
			Boundaries: children[instance.ID],
		})
	}
	return ops
}

// disappeared reports whether the browser holds a boundary this render no
// longer produces and nothing in the response can say so.
//
// A removal is covered when its parent survives with its own markup unchanged:
// that parent's child set shrank, so it reports the survivors and the client
// drops what the list no longer names. The test is the parent's frame rather
// than its mere presence, because a chain member is numbered by position — a
// shorter chain renumbers, so an id surviving can mean a different component
// wearing the same number, whose operation says nothing about the region that
// went.
//
// Everything else falls back to replacing the outermost boundary, which takes
// the region off the screen along with everything else that moved.
func disappeared(manifest, known Manifest) bool {
	for _, before := range known.Instances {
		if _, ok := manifest.Find(before.ID); ok {
			continue
		}
		if before.ParentID == "" {
			return true
		}
		parent, ok := manifest.Find(before.ParentID)
		if !ok || parent.FrameValidator != knownFrame(known, before.ParentID) {
			return true
		}
	}
	return false
}

// knownFrame is the frame the client holds for an instance, or empty when it
// holds none — which never compares equal to a rendered one.
func knownFrame(known Manifest, id string) string {
	instance, ok := known.Find(id)
	if !ok {
		return ""
	}
	return instance.FrameValidator
}
