package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

// A boundary's instance id is author/request data — a reloadable component
// reads it from a struct field the call site fills, and a redraw takes it
// straight from a request header. It lands in a double-quoted HTML attribute
// on the delta path: the boundary's own element via BoundaryAttr, and a nested
// boundary's <template> hole via the placeholder. Both are HTML the client
// runtime parses into the DOM, so the id must be escaped there like any other
// attribute value. It was not: an id of x"><img> broke out of the attribute.
func TestBoundaryInstanceIDIsEscaped(t *testing.T) {
	const hostileID = `x"><img src=x onerror=alert(1)>`

	// The boundary's own element carries the id via BoundaryAttr, written
	// during delta collection where the collector is live.
	var top strings.Builder
	if _, err := delta.CollectChain(&top, []byte("k"), nil,
		htmlbind.Bind(rowPlan(func(p rowParams) string { return p.ID }),
			rowParams{ID: hostileID, Text: "hi"})); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(top.String(), `<img src=x`) {
		t.Fatalf("BoundaryAttr let the id break the attribute:\n%s", top.String())
	}

	// A nested boundary leaves a <template> placeholder carrying its id.
	var page strings.Builder
	if _, err := delta.CollectChain(&page, []byte("k"), nil,
		htmlbind.Bind(pagePlan(rowPlan(func(p rowParams) string { return p.ID })),
			pageParams{Rows: []rowParams{{ID: hostileID, Text: "hi"}}})); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page.String(), `<img src=x`) {
		t.Fatalf("delta placeholder let the id break the attribute:\n%s", page.String())
	}
}
