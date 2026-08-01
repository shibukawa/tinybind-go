package htmlbind

import "io"

// Operation is one change the browser applies to reach the server's render.
type Operation struct {
	// Kind is replace. Insert, remove, and move arrive with structural
	// boundaries; until then a structural change is expressed by replacing an
	// enclosing boundary.
	Kind string
	// InstanceID names the boundary the operation targets.
	InstanceID string
	// HTML is the boundary's complete subtree, including its root element.
	HTML string
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
func RenderDelta(key []byte, known Manifest, wrappers []Wrapper, leaf Fragment) (Delta, error) {
	collect := &collector{key: key, capture: true}
	manifest, head, err := renderChainHead(io.Discard, collect, wrappers, leaf)
	if err != nil {
		return Delta{}, err
	}
	return Delta{
		Manifest:   manifest,
		Operations: operations(manifest, known, collect.contents),
		Head:       head,
	}, nil
}

// operations selects the topmost changed boundaries. A descendant of a replaced
// boundary is already contained in that replacement, so sending it again would
// both waste bytes and apply to a node that no longer exists.
func operations(manifest, known Manifest, contents map[string]string) []Operation {
	replaced := map[string]bool{}
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
		if replaced[instance.ParentID] {
			replaced[instance.ID] = true
			continue
		}
		before, ok := known.Find(instance.ID)
		unchanged := ok && before.FrameValidator == instance.FrameValidator
		if unchanged && !(forceRoot && instance.ParentID == "") {
			continue
		}
		replaced[instance.ID] = true
		ops = append(ops, Operation{
			Kind:       OpReplace,
			InstanceID: instance.ID,
			HTML:       contents[instance.ID],
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
