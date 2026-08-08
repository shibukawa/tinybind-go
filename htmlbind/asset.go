package htmlbind

// Asset is one static file a component requires: a stylesheet or a script the
// generator extracted and named after the hash of its own bytes.
//
// It is not derivable from Head. A head contribution is a ready-to-write tag, so
// a caller reading Head gets markup and has to parse it back out to learn what
// the component needs; and a fragment produced while rendering contributes no
// head at all. This is the same fact expressed as an identity a caller can act
// on: decide whether the document already carries it, decide whether a response
// with no document shell can deliver it, decide whether to preload it.
//
// The division is the one the module keeps everywhere else. It decides what is
// required and what its identity is; where the bytes are served is the caller's.
type Asset struct {
	// ID identifies the file by its content. Two components requiring one asset
	// carry one ID, and an edited file is a different one, which is what lets an
	// immutable cache header stay honest.
	ID string
	// Type is the media type: text/css for a stylesheet, text/javascript for a
	// script.
	Type string
	// URL is where the reference tag points, which is the generation-time public
	// URL base joined to the file name.
	URL string
}

// Asset media types. They are the two kinds requirement:static-asset-extraction
// produces, and the module writes no other.
const (
	AssetTypeStyle  = "text/css"
	AssetTypeScript = "text/javascript"
)

// Assets returns every static file this fragment requires, its own and those of
// every component it can reach, including one supplied through a slot.
//
// It is readable before rendering starts, because it is bound to the value
// rather than produced by walking one. That timing is the point: a live delivery
// or a fragment swap can insert a component whose script was not in the first
// render, and a set known up front lets the document carry every asset a later
// swap might need. Nothing is then fetched mid-swap, and no client-side loading
// design has to enter the module.
//
// A member below a slot that never renders still counts. The set is what this
// value could require, not what one render happened to reach — the same
// direction HasAwaitBlock already takes, and the conservative one for a caller
// deciding what to put in a document shell.
func (f Fragment) Assets() []Asset { return f.assets }

// Assets returns every static file this wrapper requires. It is the Wrapper form
// of the accessor documented on Fragment.
func (w Wrapper) Assets() []Asset { return w.assets }

// MergeAssets is the required set of a whole chain, deduplicated by identity, in
// composition order outermost first.
//
// It takes the same argument pair as MergeHead because it answers the same
// question one layer down: MergeHead says what tags this document writes, and
// this says what files those tags stand for.
func MergeAssets(wrappers []Wrapper, leaf Fragment) []Asset {
	// One contributor has nothing to deduplicate against, so its slice is the
	// answer; merged results are treated as immutable everywhere they travel.
	single, contributors := leaf.assets, 0
	if len(leaf.assets) > 0 {
		contributors = 1
	}
	for _, wrapper := range wrappers {
		if len(wrapper.assets) > 0 {
			single = wrapper.assets
			contributors++
		}
	}
	if contributors == 0 {
		return nil
	}
	if contributors == 1 {
		return single
	}
	var merged []Asset
	seen := map[string]bool{}
	add := func(assets []Asset) {
		for _, asset := range assets {
			if asset.ID == "" || seen[asset.ID] {
				continue
			}
			seen[asset.ID] = true
			merged = append(merged, asset)
		}
	}
	for _, wrapper := range wrappers {
		add(wrapper.assets)
	}
	add(leaf.assets)
	return merged
}

// appendAssets adds requirements to a set, dropping ones already in it.
//
// The result is a fresh slice whenever anything is added, because the set it
// starts from is the plan's own and a plan is shared by every render.
func appendAssets(assets, add []Asset) []Asset {
	if len(add) == 0 {
		return assets
	}
	seen := make(map[string]bool, len(assets))
	for _, asset := range assets {
		seen[asset.ID] = true
	}
	grown := append([]Asset(nil), assets...)
	for _, asset := range add {
		if asset.ID == "" || seen[asset.ID] {
			continue
		}
		seen[asset.ID] = true
		grown = append(grown, asset)
	}
	if len(grown) == len(assets) {
		return assets
	}
	return grown
}
