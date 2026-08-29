package delta_test

import (
	"context"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

type wrap struct {
	ID    string
	Title string
	N     int
}

var wrapOps = htmlbind.Builder[wrap]{}

// wrapPlan has a boundary body containing a Val with a multi-op body, a Require,
// and a ForCtx loop — the three shapes the decomposition and the render used to
// disagree on.
var wrapPlan = &htmlbind.Plan[wrap]{
	Boundary: &htmlbind.Boundary[wrap]{
		ComponentID: "pages.wrap.Wrap",
		Attr:        "data-tb-id",
		Instance:    func(p wrap) string { return p.ID },
		Input:       func(p wrap) string { return delta.CanonJoin(delta.CanonString(p.Title)) },
	},
	Ops: []htmlbind.Op[wrap]{
		wrapOps.Static("<section"), wrapOps.BoundaryAttr(), wrapOps.Static(">"),
		wrapOps.Require(func(p wrap) error { return nil }),
		htmlbind.Val(
			func(p wrap) string { return p.Title },
			func(p wrap, v string) string { return v },
			[]htmlbind.Op[string]{
				htmlbind.Builder[string]{}.Static("<h1>"),
				htmlbind.Builder[string]{}.Text(func(s string) string { return s }),
				htmlbind.Builder[string]{}.Static("</h1>"),
			}),
		wrapOps.Static("<ul>"),
		htmlbind.ForCtx(
			func(_ context.Context, p wrap) []int { return make([]int, p.N) },
			func(_ wrap, _ int, i int) int { return i },
			[]htmlbind.Op[int]{
				htmlbind.Builder[int]{}.Static("<li>x</li>"),
			}),
		wrapOps.Static("</ul></section>"),
	},
}

func TestValRequireForCtxRoundTrip(t *testing.T) {
	result, err := delta.RenderDelta([]byte("k"), delta.Manifest{}, nil,
		htmlbind.Bind(wrapPlan, wrap{ID: "w", Title: "Hi", N: 2}))
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
