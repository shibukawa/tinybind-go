package delta_test

import (
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

// A hole is markup a parser has to leave where the render wrote it, and for one
// context it did not.
//
// In table context the HTML tree construction algorithm foster-parents an
// unknown element: the in-table insertion mode processes anything it does not
// recognise with foster parenting enabled, which inserts the element immediately
// before the table. The hole this package used to write was such an element, so
// every hole a table's rows left ended up outside the table, the rows filling
// them landed loose on the page, and the list was left empty. The response was
// correct as bytes and the resulting DOM was valid, so neither side reported it.
//
// This is parsed rather than pattern-matched for that reason. Asserting the hole
// is spelled a particular way restates the code; asserting a conforming parser
// keeps it inside the tbody is the property the page depends on.

type cell struct {
	ID   string
	Text string
}

var cellOps = htmlbind.Builder[cell]{}

var cellPlan = &htmlbind.Plan[cell]{
	Boundary: &htmlbind.Boundary[cell]{
		ComponentID: "pages.table.Cell",
		Attr:        "data-tb-id",
		Instance:    func(p cell) string { return p.ID },
		Input:       func(p cell) string { return delta.CanonJoin(delta.CanonString(p.Text)) },
	},
	Ops: []htmlbind.Op[cell]{
		cellOps.Static("<tr"), cellOps.BoundaryAttr(), cellOps.Static("><td>"),
		cellOps.Text(func(p cell) string { return p.Text }),
		cellOps.Static("</td></tr>"),
	},
}

type grid struct {
	ID    string
	Cells []cell
}

var gridOps = htmlbind.Builder[grid]{}

var gridPlan = &htmlbind.Plan[grid]{
	Boundary: &htmlbind.Boundary[grid]{
		ComponentID: "pages.table.Grid",
		Attr:        "data-tb-id",
		Instance:    func(p grid) string { return p.ID },
		Input:       func(grid) string { return "" },
	},
	Ops: []htmlbind.Op[grid]{
		gridOps.Static("<table"), gridOps.BoundaryAttr(), gridOps.Static("><tbody>"),
		htmlbind.For(
			func(p grid) []cell { return p.Cells },
			func(_ grid, item cell, _ int) cell { return item },
			[]htmlbind.Op[cell]{
				cellOps.Component(func(p cell) htmlbind.Fragment { return htmlbind.Bind(cellPlan, p) }),
			}),
		gridOps.Static("</tbody></table>"),
	},
}

// parsedHoles names the element each hole ends up under once a conforming parser
// has read the fragment, keyed by the boundary id the hole carries.
func parsedHoles(t *testing.T, fragment string) map[string]string {
	t.Helper()
	document, err := html.Parse(strings.NewReader("<!doctype html><html><body>" + fragment))
	if err != nil {
		t.Fatal(err)
	}
	under := map[string]string{}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode {
				for _, attr := range child.Attr {
					if attr.Key == "data-tb-id" && child.Data != "table" {
						under[attr.Val] = node.Data
					}
				}
			}
			walk(child)
		}
	}
	walk(document)
	return under
}

func TestAHoleInsideATableStaysInsideTheTable(t *testing.T) {
	result, err := delta.RenderDelta([]byte("k"), delta.Manifest{}, nil,
		htmlbind.Bind(gridPlan, grid{ID: "grid", Cells: []cell{{ID: "c-1", Text: "a"}, {ID: "c-2", Text: "b"}}}))
	if err != nil {
		t.Fatal(err)
	}
	operation, ok := find(result.Operations, "grid")
	if !ok {
		t.Fatal("no operation for the grid")
	}
	under := parsedHoles(t, operation.HTML)
	for _, id := range []string{"c-1", "c-2"} {
		// tbody, not body: a foster-parented hole lands as a sibling of the
		// table, and the row that fills it then lands there too.
		if under[id] != "tbody" {
			t.Fatalf("hole %s parsed under %q, want tbody — the row filling it would land outside the table:\n%s",
				id, under[id], operation.HTML)
		}
	}
}

// The row a hole is filled with is parsed too, and a bare tr outside a template
// loses its tags. The client parses inside a template element for that reason;
// this asserts the fragment the server sends survives it.
func TestAFilledRowSurvivesBeingParsed(t *testing.T) {
	result, err := delta.RenderDelta([]byte("k"), delta.Manifest{}, nil,
		htmlbind.Bind(gridPlan, grid{ID: "grid", Cells: []cell{{ID: "c-1", Text: "a"}}}))
	if err != nil {
		t.Fatal(err)
	}
	operation, ok := find(result.Operations, "c-1")
	if !ok {
		t.Fatal("no operation for the cell")
	}
	if !strings.HasPrefix(operation.HTML, "<tr") {
		t.Fatalf("the cell fragment is not a row: %q", operation.HTML)
	}
}
