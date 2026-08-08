package htmlbind

// Boundary marks a component as an automatic partial update boundary. Chain
// members carry one; an ordinary component call does not become an instance,
// so a manifest stays the size of the layout chain rather than the size of the
// document.
//
// A boundary is declared by generated code and never by hand, because its
// identity must change with the generated component version.
type Boundary[P any] struct {
	// ComponentID is the stable declaration identity of this component,
	// including its generated version. A template edit changes it, which
	// invalidates every validator derived from it.
	ComponentID string
	// Attr is the data attribute carrying the instance ID on the boundary's
	// root element. The generator writes the configured prefix into it.
	Attr string
	// Input canonically encodes the declared parameters, excluding slot
	// arguments, which belong to the child boundary rather than this frame.
	Input func(P) string
	// Instance returns the id this invocation is addressed by, for a component
	// that names its own — a reloadable one, whose id an author writes at the
	// call site. It is nil for a chain member, whose id comes from its position
	// in the chain instead.
	//
	// It is also what decides whether a component call opens a boundary at all.
	// A reloadable component is an update boundary wherever it renders, so a
	// delta can compare the region a redraw can replace; every other component
	// call stays out of the manifest, which is what keeps a manifest the size
	// of the regions that update rather than the size of the document.
	Instance func(P) string
}

// boundary is the type-erased form stored on a bound fragment, so a chain can
// open a boundary without naming the component's parameter struct.
type boundary struct {
	componentID string
	attr        string
	// instance is the author-written id of a component that names its own, and
	// empty for a chain member, which is numbered by its position instead.
	instance string
	input    func() string
}

func bindBoundary[P any](decl *Boundary[P], params P) *boundary {
	if decl == nil {
		return nil
	}
	bound := &boundary{
		componentID: decl.ComponentID,
		attr:        decl.Attr,
		input:       func() string { return decl.Input(params) },
	}
	if decl.Instance != nil {
		bound.instance = decl.Instance(params)
	}
	return bound
}
