package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

// A reloadable component is an update boundary wherever it renders, not only
// when it happens to be a chain member. Before that, a page could redraw a
// region the navigation delta had no entry for, so the only way a delta could
// notice the region changed was to replace an ancestor.
//
// A component names its own instance to opt in. Everything else stays out of
// the manifest, which is what keeps a manifest the size of the regions that
// update rather than the size of the document.

type rowParams struct {
	ID   string
	Text string
}

var rowOps = htmlbind.Builder[rowParams]{}

func rowPlan(instance func(rowParams) string) *htmlbind.Plan[rowParams] {
	return &htmlbind.Plan[rowParams]{
		Boundary: &htmlbind.Boundary[rowParams]{
			ComponentID: "pages.rows.Row",
			Attr:        "data-tb-id",
			Instance:    instance,
			Input:       func(p rowParams) string { return delta.CanonJoin(delta.CanonString(p.Text)) },
		},
		Ops: []htmlbind.Op[rowParams]{
			rowOps.Static("<li"),
			rowOps.BoundaryAttr(),
			rowOps.Static(">"),
			rowOps.Text(func(p rowParams) string { return p.Text }),
			rowOps.Static("</li>"),
		},
	}
}

type pageParams struct{ Rows []rowParams }

var pageOps = htmlbind.Builder[pageParams]{}

func pagePlan(rows *htmlbind.Plan[rowParams]) *htmlbind.Plan[pageParams] {
	return &htmlbind.Plan[pageParams]{
		Ops: []htmlbind.Op[pageParams]{
			pageOps.Static("<ul>"),
			htmlbind.For(
				func(p pageParams) []rowParams { return p.Rows },
				func(_ pageParams, item rowParams, _ int) rowParams { return item },
				[]htmlbind.Op[rowParams]{
					rowOps.Component(func(p rowParams) htmlbind.Fragment { return htmlbind.Bind(rows, p) }),
				}),
			pageOps.Static("</ul>"),
		},
	}
}

var namedRows = pagePlan(rowPlan(func(p rowParams) string { return p.ID }))

// unnamedRows is the same component with no instance of its own — the shape
// every ordinary call has.
var unnamedRows = pagePlan(rowPlan(nil))

func collectPage(t *testing.T, plan *htmlbind.Plan[pageParams]) (string, delta.Manifest) {
	t.Helper()
	var out strings.Builder
	manifest, err := delta.CollectChain(&out, []byte("k"), nil,
		htmlbind.Bind(plan, pageParams{Rows: []rowParams{
			{ID: "row-a", Text: "alpha"},
			{ID: "row-b", Text: "beta"},
		}}))
	if err != nil {
		t.Fatal(err)
	}
	return out.String(), manifest
}

func TestNamedComponentBecomesAnInstance(t *testing.T) {
	body, manifest := collectPage(t, namedRows)
	if len(manifest.Instances) != 2 {
		t.Fatalf("instances = %+v", manifest.Instances)
	}
	for i, want := range []string{"row-a", "row-b"} {
		if got := manifest.Instances[i].ID; got != want {
			t.Fatalf("instance %d id = %q, want %q", i, got, want)
		}
		if manifest.Instances[i].FrameValidator == "" {
			t.Fatalf("instance %d carries no frame validator", i)
		}
		// The instance attribute lands on the boundary's own root element, so a
		// browser can address the region the redraw will replace.
		if !strings.Contains(body, `<li data-tb-id="`+want+`">`) {
			t.Fatalf("body carries no instance attribute for %s: %s", want, body)
		}
	}
	// Two instances of one component are told apart by their author-written ids
	// and by nothing else, so their frames must differ when their content does.
	if manifest.Instances[0].FrameValidator == manifest.Instances[1].FrameValidator {
		t.Fatal("two rows with different text share a frame validator")
	}
}

// A component that names no instance stays out of the manifest, whatever else
// its plan declares. Without this an ordinary component call would enter the
// manifest of every page that renders it.
func TestUnnamedComponentStaysOutOfTheManifest(t *testing.T) {
	body, manifest := collectPage(t, unnamedRows)
	if len(manifest.Instances) != 0 {
		t.Fatalf("instances = %+v", manifest.Instances)
	}
	if strings.Contains(body, "data-tb-id") {
		t.Fatalf("body carries an instance attribute: %s", body)
	}
}

// A frame covers a boundary's own markup and excludes what a nested boundary
// wrote, so an ancestor whose own frame is unchanged keeps its DOM while a
// child is replaced. Changing one row must not move the other's frame.
func TestOneChangedRowLeavesTheOtherFrameAlone(t *testing.T) {
	var before, after strings.Builder
	first, err := delta.CollectChain(&before, []byte("k"), nil,
		htmlbind.Bind(namedRows, pageParams{Rows: []rowParams{
			{ID: "row-a", Text: "alpha"}, {ID: "row-b", Text: "beta"},
		}}))
	if err != nil {
		t.Fatal(err)
	}
	second, err := delta.CollectChain(&after, []byte("k"), nil,
		htmlbind.Bind(namedRows, pageParams{Rows: []rowParams{
			{ID: "row-a", Text: "alpha"}, {ID: "row-b", Text: "gamma"},
		}}))
	if err != nil {
		t.Fatal(err)
	}
	changed := second.Changed(first)
	if len(changed) != 1 || changed[0].ID != "row-b" {
		t.Fatalf("changed = %+v, want only row-b", changed)
	}
}

// Rendering without a collector is the ordinary document path, and it must stay
// byte-identical: the boundary machinery exists for a render that collects.
func TestNamedComponentWritesNothingExtraWhenNotCollecting(t *testing.T) {
	var out strings.Builder
	if err := htmlbind.Render(&out, htmlbind.Bind(namedRows, pageParams{
		Rows: []rowParams{{ID: "row-a", Text: "alpha"}},
	})); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "<ul><li>alpha</li></ul>" {
		t.Fatalf("body = %q", got)
	}
}
