package delta_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

// A fragment's static half is derived from the plan and never from a render, so
// a server can serve one back from its address. Its varying half travels per
// render. The property everything rests on is that the two reproduce exactly the
// bytes the render wrote — a split that does not round-trip is not a split.

func sequenceOf(t *testing.T, address string) *htmlbind.Sequence {
	t.Helper()
	sequence, ok := htmlbind.LookupSequence(address)
	if !ok {
		t.Fatalf("no sequence registered under %q", address)
	}
	return sequence
}

func TestSequenceAndValuesReproduceTheFragment(t *testing.T) {
	for _, rows := range [][]row{
		{},
		{{ID: "row-a", Text: "alpha"}},
		{{ID: "row-a", Text: "alpha"}, {ID: "row-b", Text: "beta"}},
	} {
		result := render(t, delta.Manifest{}, panel{ID: "panel", Title: "Inbox", Rows: rows})
		for _, operation := range result.Operations {
			if operation.Sequence == "" {
				t.Fatalf("%s carries no sequence", operation.InstanceID)
			}
			rebuilt, err := sequenceOf(t, operation.Sequence).Reassemble(operation.Values)
			if err != nil {
				t.Fatalf("%s: %v", operation.InstanceID, err)
			}
			if rebuilt != operation.HTML {
				t.Fatalf("%s round trip:\n got %q\nwant %q", operation.InstanceID, rebuilt, operation.HTML)
			}
		}
	}
}

// One address per component, whatever the data did. A five-row list and a
// six-row list share it, which is what makes a sequence worth caching at all —
// enumerating one per instruction path would be exponential in a component's
// conditionals, and one per row count would never be reused.
func TestOneAddressWhateverTheDataDid(t *testing.T) {
	addresses := map[string]bool{}
	for _, count := range []int{0, 1, 5, 30} {
		rows := make([]row, count)
		for i := range rows {
			rows[i] = row{ID: "row-" + strings.Repeat("x", i+1), Text: "t"}
		}
		result := render(t, delta.Manifest{}, panel{ID: "panel", Title: "Inbox", Rows: rows})
		for _, operation := range result.Operations {
			if operation.InstanceID == panelID {
				addresses[operation.Sequence] = true
			}
		}
	}
	if len(addresses) != 1 {
		t.Fatalf("the panel took %d addresses across row counts, want one: %v", len(addresses), addresses)
	}
}

// The hole frame is identical for every row, so it belongs to the static half.
// Leaving it in the values would make a hundred holes a hundred copies of one
// element, which is the cost the whole split exists to remove.
//
// The measurement is what the values do as rows are added, not what fraction of
// one render they are. A fraction measures the frame's length: the frame used to
// be a 67-byte custom element and is now a 35-byte template, so the same correct
// behaviour moved the ratio and failed a threshold. What the property actually
// says is that a row adds its identity to the values and nothing else.
func TestHoleFramesAreStatic(t *testing.T) {
	valuesFor := func(count int) []string {
		rows := make([]row, count)
		for i := range rows {
			rows[i] = row{ID: "row-" + strconv.Itoa(i), Text: "t"}
		}
		result := render(t, delta.Manifest{}, panel{ID: "panel", Title: "Inbox", Rows: rows})
		parent, ok := find(result.Operations, panelID)
		if !ok {
			t.Fatal("no operation for the panel")
		}
		return parent.Values
	}
	ten, twenty := valuesFor(10), valuesFor(20)
	// Three entries per hole: whether the call opened a boundary or rendered
	// inline, the attribute name, and the id. The attribute travels because a
	// boundary declares its own, so it is not knowable from the plan the
	// sequence was derived from; the frame around it is not in the list at all.
	if added := len(twenty) - len(ten); added != 30 {
		t.Fatalf("ten more rows added %d values, want 30 — the frame is travelling per row", added)
	}
	joined := strings.Join(twenty, "")
	for _, frame := range []string{"<template", "</template", "display:contents"} {
		if strings.Contains(joined, frame) {
			t.Fatalf("the hole frame travels with the values: %q", joined)
		}
	}
}

// Values that do not fit the tree are refused rather than reassembled into
// something plausible, because a mismatch means the two came from different
// renders or different builds.
func TestMismatchedValuesAreRefused(t *testing.T) {
	result := render(t, delta.Manifest{}, base)
	parent, ok := find(result.Operations, panelID)
	if !ok {
		t.Fatal("no operation for the panel")
	}
	sequence := sequenceOf(t, parent.Sequence)
	if _, err := sequence.Reassemble(nil); err == nil {
		t.Fatal("an empty value list must not reassemble")
	}
	if _, err := sequence.Reassemble(append(parent.Values, "extra")); err == nil {
		t.Fatal("leftover values must not reassemble")
	}
}

// A conditional is structure the tree carries, and which branch ran is data. Two
// renders taking different branches share one address and differ only in values.
func TestBothBranchesShareOneAddress(t *testing.T) {
	ops := htmlbind.Builder[panel]{}
	plan := &htmlbind.Plan[panel]{
		Boundary: &htmlbind.Boundary[panel]{
			ComponentID: "pages.panel.Switch", Attr: "data-tb-id",
			Instance: func(p panel) string { return p.ID },
			Input:    func(p panel) string { return delta.CanonString(p.Title) },
		},
		Ops: []htmlbind.Op[panel]{
			ops.Static("<div"), ops.BoundaryAttr(), ops.Static(">"),
			ops.If(func(p panel) bool { return p.Title == "Inbox" },
				[]htmlbind.Op[panel]{ops.Static("<b>"), ops.Text(func(p panel) string { return p.Title }), ops.Static("</b>")},
				[]htmlbind.Op[panel]{ops.Static("<i>none</i>")}),
			ops.Static("</div>"),
		},
	}
	seen := map[string]bool{}
	for _, title := range []string{"Inbox", "Archive"} {
		result, err := delta.RenderDelta([]byte("k"), delta.Manifest{}, nil,
			htmlbind.Bind(plan, panel{ID: "sw", Title: title}))
		if err != nil {
			t.Fatal(err)
		}
		operation := result.Operations[0]
		seen[operation.Sequence] = true
		rebuilt, err := sequenceOf(t, operation.Sequence).Reassemble(operation.Values)
		if err != nil {
			t.Fatalf("%s: %v", title, err)
		}
		if rebuilt != operation.HTML {
			t.Fatalf("%s round trip: got %q want %q", title, rebuilt, operation.HTML)
		}
	}
	if len(seen) != 1 {
		t.Fatalf("two branches took %d addresses, want one", len(seen))
	}
}

// The saving, measured rather than asserted. It needs repetition or a large
// static frame to show: on a two-element fragment the address costs more than
// the markup it replaces, and on a list it is the whole point.
func TestValuesShrinkWithRepetition(t *testing.T) {
	for _, count := range []int{1, 30, 100} {
		rows := make([]row, count)
		for i := range rows {
			rows[i] = row{ID: "r" + strconv.Itoa(i), Text: "hello there"}
		}
		result := render(t, delta.Manifest{}, panel{ID: "panel", Title: "Inbox", Rows: rows})
		markup, values := 0, 0
		for _, operation := range result.Operations {
			markup += len(operation.HTML)
			values += len(operation.Sequence)
			for _, value := range operation.Values {
				values += len(value)
			}
		}
		t.Logf("rows=%-4d markup=%-7d values=%-7d", count, markup, values)
		if values >= markup {
			t.Fatalf("rows=%d: values %d against markup %d, so the split bought nothing", count, values, markup)
		}
	}
}
