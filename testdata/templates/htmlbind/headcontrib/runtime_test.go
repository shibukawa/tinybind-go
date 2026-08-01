package pages

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// A component's contribution is one entry per tag, and HeadSources names the
// component that declared each one, in the same order.
func TestHeadTagsCarryTheirSource(t *testing.T) {
	badge := Badge(BadgeParams{Label: "new"})
	head, sources := badge.Head(), badge.HeadSources()
	if len(head) != 2 {
		t.Fatalf("head = %q, want one entry per tag", head)
	}
	if len(sources) != len(head) {
		t.Fatalf("sources = %q, want the same length as head %q", sources, head)
	}
	if !strings.HasPrefix(sources[0], "Badge (") || !strings.HasPrefix(sources[1], "Badge (") {
		t.Fatalf("sources = %q, want each to name Badge", sources)
	}
	for index, tag := range head {
		if !strings.Contains(sources[index], ".txt:") {
			t.Errorf("source %d = %q for tag %q, want a file position", index, sources[index], tag)
		}
	}
}

// Two components sharing a stylesheet emit one link, which
// requirement:head-merging requires and per-component granularity could not do.
func TestSharedStylesheetCollapses(t *testing.T) {
	panel := Panel(PanelParams{Label: "new", Text: "hi"})
	head := panel.Head()
	links := 0
	for _, tag := range head {
		if strings.Contains(tag, `href="/shared.css"`) {
			links++
		}
	}
	if links != 1 {
		t.Fatalf("head = %q, want exactly one shared.css link, got %d", head, links)
	}
	// Both scoped styles were extracted into the one stylesheet of this
	// generation unit, so they contribute a single link rather than two blocks.
	if len(head) != 2 {
		t.Fatalf("head = %q, want the shared link plus the extracted stylesheet", head)
	}
	if strings.Contains(strings.Join(head, ""), "<style") {
		t.Fatalf("head = %q, want no inline style block", head)
	}
	if len(panel.HeadSources()) != len(head) {
		t.Fatalf("sources = %q, want the same length as head %q", panel.HeadSources(), head)
	}
}

// The wrapper form carries the same pair, because a chain member may be either.
func TestWrapperExposesTheSamePair(t *testing.T) {
	document := BindDocument(DocumentParams{Label: "a", Text: "b"})
	if len(document.Head()) != len(document.HeadSources()) {
		t.Fatalf("wrapper head = %q, sources = %q", document.Head(), document.HeadSources())
	}
}

// A component with no contribution reports nothing rather than an empty tag.
func TestNoContributionReportsNothing(t *testing.T) {
	document := Document(DocumentParams{Label: "a", Text: "b"})
	if len(document.Head()) != 0 || len(document.HeadSources()) != 0 {
		t.Fatalf("head = %q, sources = %q, want both empty", document.Head(), document.HeadSources())
	}
}

// Merging still deduplicates across chain members, which a member cannot do
// because it cannot see the members it is composed with.
func TestMergeStillDeduplicatesAcrossMembers(t *testing.T) {
	merged := htmlbind.MergeHead(nil, Panel(PanelParams{Label: "a", Text: "b"}))
	links := 0
	for _, tag := range merged {
		if strings.Contains(tag, `href="/shared.css"`) {
			links++
		}
	}
	if links != 1 {
		t.Fatalf("merged = %q, want one shared.css link", merged)
	}
}
