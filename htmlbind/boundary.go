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
}

// boundary is the type-erased form stored on a bound fragment, so a chain can
// open a boundary without naming the component's parameter struct.
type boundary struct {
	componentID string
	attr        string
	input       func() string
}

func bindBoundary[P any](decl *Boundary[P], params P) *boundary {
	if decl == nil {
		return nil
	}
	return &boundary{
		componentID: decl.ComponentID,
		attr:        decl.Attr,
		input:       func() string { return decl.Input(params) },
	}
}
