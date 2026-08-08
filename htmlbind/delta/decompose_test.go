package delta_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

// A delta used to send the topmost changed boundary carrying its whole subtree,
// so a changed panel recreated every row inside it — discarding the focus, the
// form values, and the playing media those rows held. A fragment now stops at
// its children: each carries a hole where a nested boundary sits, an unchanged
// child is sent by nobody, and the client moves the node it already holds into
// the hole.

type row struct {
	ID   string
	Text string
}

var rowOps = htmlbind.Builder[row]{}

var rowPlan = &htmlbind.Plan[row]{
	Boundary: &htmlbind.Boundary[row]{
		ComponentID: "pages.panel.Row",
		Attr:        "data-tb-id",
		Instance:    func(p row) string { return p.ID },
		Input:       func(p row) string { return delta.CanonJoin(delta.CanonString(p.Text)) },
	},
	Ops: []htmlbind.Op[row]{
		rowOps.Static("<li"), rowOps.BoundaryAttr(), rowOps.Static(">"),
		rowOps.Text(func(p row) string { return p.Text }),
		rowOps.Static("</li>"),
	},
}

type panel struct {
	ID    string
	Title string
	Rows  []row
}

var panelOps = htmlbind.Builder[panel]{}

var panelPlan = &htmlbind.Plan[panel]{
	Boundary: &htmlbind.Boundary[panel]{
		ComponentID: "pages.panel.Panel",
		Attr:        "data-tb-id",
		Instance:    func(p panel) string { return p.ID },
		Input:       func(p panel) string { return delta.CanonJoin(delta.CanonString(p.Title)) },
	},
	Ops: []htmlbind.Op[panel]{
		panelOps.Static("<section"), panelOps.BoundaryAttr(), panelOps.Static("><h1>"),
		panelOps.Text(func(p panel) string { return p.Title }),
		panelOps.Static("</h1><ul>"),
		htmlbind.For(
			func(p panel) []row { return p.Rows },
			func(_ panel, item row, _ int) row { return item },
			[]htmlbind.Op[row]{
				rowOps.Component(func(p row) htmlbind.Fragment { return htmlbind.Bind(rowPlan, p) }),
			}),
		panelOps.Static("</ul></section>"),
	},
}

func render(t *testing.T, known delta.Manifest, p panel) delta.Delta {
	t.Helper()
	result, err := delta.RenderDelta([]byte("k"), known, nil, htmlbind.Bind(panelPlan, p))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// panelID is what the chain leaf is numbered by. A chain member takes a
// positional identity — it is the same member of the same chain whatever its
// parameters say — while a component that names its own instance keeps the
// author-written id, which is what the rows below assert.
const panelID = "c0"

func find(ops []delta.Operation, id string) (delta.Operation, bool) {
	for _, op := range ops {
		if op.InstanceID == id {
			return op, true
		}
	}
	return delta.Operation{}, false
}

var base = panel{ID: "panel", Title: "Inbox", Rows: []row{
	{ID: "row-a", Text: "alpha"},
	{ID: "row-b", Text: "beta"},
}}

// A parent's fragment stops at its children: a hole carries the child's id and
// nothing of what the child rendered.
func TestFragmentCarriesHolesNotChildren(t *testing.T) {
	first := render(t, delta.Manifest{}, base)
	parent, ok := find(first.Operations, panelID)
	if !ok {
		t.Fatalf("no operation for the panel: %+v", first.Operations)
	}
	if strings.Contains(parent.HTML, "alpha") || strings.Contains(parent.HTML, "beta") {
		t.Fatalf("the panel fragment carries its children's bytes: %s", parent.HTML)
	}
	for _, id := range []string{"row-a", "row-b"} {
		if !strings.Contains(parent.HTML, `data-tb-id="`+id+`"`) {
			t.Fatalf("the panel fragment has no hole for %s: %s", id, parent.HTML)
		}
	}
	// The list is what separates a hole to fill from one to retain, and nothing
	// in the markup does.
	if strings.Join(parent.Boundaries, ",") != "row-a,row-b" {
		t.Fatalf("boundaries = %v", parent.Boundaries)
	}
}

// The case the change exists for: one row changes, and the row beside it is
// sent by nobody even though their shared parent is not re-sent either.
func TestOnlyTheChangedRowIsSent(t *testing.T) {
	known := render(t, delta.Manifest{}, base).Manifest
	next := base
	next.Rows = []row{{ID: "row-a", Text: "alpha"}, {ID: "row-b", Text: "gamma"}}

	result := render(t, known, next)
	if len(result.Operations) != 1 {
		t.Fatalf("operations = %+v", result.Operations)
	}
	if got := result.Operations[0].InstanceID; got != "row-b" {
		t.Fatalf("sent %q, want row-b", got)
	}
}

// A changed parent does not drag its unchanged children along. They arrive as
// holes in its fragment, and the client fills them from the DOM it already has.
func TestChangedParentRetainsUnchangedChildren(t *testing.T) {
	known := render(t, delta.Manifest{}, base).Manifest
	next := base
	next.Title = "Archive"

	result := render(t, known, next)
	if len(result.Operations) != 1 {
		t.Fatalf("operations = %+v, want the panel alone", result.Operations)
	}
	parent := result.Operations[0]
	if parent.InstanceID != panelID {
		t.Fatalf("sent %q, want the panel", parent.InstanceID)
	}
	// Both children are named as holes, and neither carries an operation, which
	// is what tells the client to retain rather than to wait for a fragment.
	if strings.Join(parent.Boundaries, ",") != "row-a,row-b" {
		t.Fatalf("boundaries = %v", parent.Boundaries)
	}
}

// A frame excludes a child's bytes and includes the child's hole. Without the
// second half a region that appeared would compare equal, and the client would
// be handed a fragment with nowhere to put it.
func TestAGainedChildChangesTheParentFrame(t *testing.T) {
	known := render(t, delta.Manifest{}, base).Manifest
	next := base
	next.Rows = append(append([]row{}, base.Rows...), row{ID: "row-c", Text: "delta"})

	result := render(t, known, next)
	parent, ok := find(result.Operations, panelID)
	if !ok {
		t.Fatalf("the panel was not re-sent, so the new row has no hole: %+v", result.Operations)
	}
	if !strings.Contains(parent.HTML, `data-tb-id="row-c"`) {
		t.Fatalf("the panel fragment has no hole for the new row: %s", parent.HTML)
	}
	// The new row is not in the known manifest, so it is sent rather than
	// retained: there is no node on screen to move into its hole.
	if _, ok := find(result.Operations, "row-c"); !ok {
		t.Fatalf("the new row was not sent: %+v", result.Operations)
	}
	// The rows that did not move are still retained, even though their parent is
	// being replaced around them.
	for _, id := range []string{"row-a", "row-b"} {
		if _, ok := find(result.Operations, id); ok {
			t.Fatalf("%s was re-sent though nothing about it changed", id)
		}
	}
}

// Nothing changed means nothing is sent, which is the property every other case
// is measured against.
func TestAnUnchangedRenderSendsNothing(t *testing.T) {
	known := render(t, delta.Manifest{}, base).Manifest
	if ops := render(t, known, base).Operations; len(ops) != 0 {
		t.Fatalf("operations = %+v", ops)
	}
}
