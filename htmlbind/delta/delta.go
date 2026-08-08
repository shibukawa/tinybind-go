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

// OpReplace names the only operation kind this milestone produces.
const OpReplace = "replace"

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
		unchanged := ok && before.FrameValidator == instance.FrameValidator
		// An unchanged boundary is never sent, including one whose parent is
		// being replaced: its hole in that replacement is what the client moves
		// its live node into. A boundary the client does not hold is not
		// unchanged by this test, since it is absent from the known manifest,
		// so a newly appearing region is always sent.
		if unchanged && !(forceRoot && instance.ParentID == "") {
			continue
		}
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
// longer produces.
func disappeared(manifest, known Manifest) bool {
	for _, before := range known.Instances {
		if _, ok := manifest.Find(before.ID); !ok {
			return true
		}
	}
	return false
}
