package delta_test

import (
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

// innerPlan opens no boundary of its own, so a Component call to it is inlined:
// its whole output is one opaque value in the parent's sequence. Its interior
// If must therefore contribute nothing to the parent's value stream — the tree
// gives the inline exactly one slot.
var innerPlan = &htmlbind.Plan[bool]{
	Ops: []htmlbind.Op[bool]{
		htmlbind.Builder[bool]{}.Static("<b>"),
		htmlbind.Builder[bool]{}.If(
			func(b bool) bool { return b },
			[]htmlbind.Op[bool]{htmlbind.Builder[bool]{}.Static("yes")},
			[]htmlbind.Op[bool]{htmlbind.Builder[bool]{}.Static("no")}),
		htmlbind.Builder[bool]{}.Static("</b>"),
	},
}

type outer struct{ ID string }

var outerOps = htmlbind.Builder[outer]{}

var outerPlan = &htmlbind.Plan[outer]{
	Boundary: &htmlbind.Boundary[outer]{
		ComponentID: "pages.outer.Outer",
		Attr:        "data-tb-id",
		Instance:    func(p outer) string { return p.ID },
		Input:       func(p outer) string { return delta.CanonJoin() },
	},
	Ops: []htmlbind.Op[outer]{
		outerOps.Static("<div"), outerOps.BoundaryAttr(), outerOps.Static(">"),
		outerOps.Component(func(p outer) htmlbind.Fragment { return htmlbind.Bind(innerPlan, true) }),
		outerOps.Static("</div>"),
	},
}

func TestInlineComponentInteriorChoiceDoesNotLeak(t *testing.T) {
	result, err := delta.RenderDelta([]byte("k"), delta.Manifest{}, nil,
		htmlbind.Bind(outerPlan, outer{ID: "o"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range result.Operations {
		if op.Sequence == "" {
			continue
		}
		seq, ok := htmlbind.LookupSequence(op.Sequence)
		if !ok {
			t.Fatalf("no sequence for %s", op.InstanceID)
		}
		rebuilt, err := seq.Reassemble(op.Values)
		if err != nil {
			t.Fatalf("%s reassemble: %v\n values=%q", op.InstanceID, err, op.Values)
		}
		if rebuilt != op.HTML {
			t.Fatalf("%s roundtrip:\n got %q\nwant %q\nvalues %q", op.InstanceID, rebuilt, op.HTML, op.Values)
		}
	}
}
