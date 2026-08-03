package htmlbind

import (
	"strings"
	"testing"
)

// A component library's whole shape is a component supplied through a slot, so
// the outer component the caller names is not the one that owns the stylesheet.
type cardParams struct {
	Body Fragment
}

// cardPlan is the outer component: it declares one head tag of its own and one
// slot, and generation gives it the accessor that reaches what filled it.
func cardPlan(head []string, sources []string) *Plan[cardParams] {
	ops := Builder[cardParams]{}
	return &Plan[cardParams]{
		Head:        head,
		HeadSources: sources,
		Slots:       func(p cardParams) []Fragment { return []Fragment{p.Body} },
		Ops: []Op[cardParams]{
			ops.Static("<div>"),
			ops.Slot(func(p cardParams) Fragment { return p.Body }, nil),
			ops.Static("</div>"),
		},
	}
}

// The reported defect: Bind copied only the plan's own head, so a component
// handed in through a parameter contributed nothing.
func TestSlotFragmentHeadIsMerged(t *testing.T) {
	inner := bodyFragment([]string{`<link rel="stylesheet" href="/library.css">`}, "inner")
	card := Bind(cardPlan([]string{`<link rel="stylesheet" href="/card.css">`}, []string{"Card"}), cardParams{Body: inner})

	head := card.Head()
	if len(head) != 2 {
		t.Fatalf("head = %q, want the component's own tag and the slot's", head)
	}
	if head[0] != `<link rel="stylesheet" href="/card.css">` {
		t.Errorf("the component's own contribution must come first, got %q", head)
	}
	if head[1] != `<link rel="stylesheet" href="/library.css">` {
		t.Errorf("the slot's contribution is missing: %q", head)
	}
	// Head and HeadSources are two views of one list, so a caller reporting
	// which component to change cannot be handed a shorter one.
	if len(card.HeadSources()) != len(head) {
		t.Fatalf("HeadSources = %q, want one entry per tag", card.HeadSources())
	}

	// Rendering the same value writes both tags, so the merge and the accessor
	// agree.
	rendered := renderShell(t, card)
	if !strings.Contains(rendered, "/library.css") {
		t.Errorf("the slot's stylesheet never reached the document: %s", rendered)
	}
}

// The failure the reporter cares about is not the missing tag; it is that a
// guard built to refuse an undeliverable contribution stayed silent for exactly
// the cross-file composition case it exists for.
func TestSlotFragmentHeadIsVisibleToAGuard(t *testing.T) {
	inner := bodyFragment([]string{`<link rel="stylesheet" href="/library.css">`}, "inner")
	card := Bind(cardPlan(nil, nil), cardParams{Body: inner})
	if len(card.Head()) == 0 {
		t.Fatal("a caller refusing a fragment response that carries head has nothing to refuse")
	}
}

// Two components declaring one stylesheet emit one tag, which is the MergeHead
// rule applied where a slot brought the duplicate.
func TestSlotFragmentHeadDeduplicates(t *testing.T) {
	tag := `<link rel="stylesheet" href="/shared.css">`
	card := Bind(cardPlan([]string{tag}, []string{"Card"}), cardParams{Body: bodyFragment([]string{tag}, "inner")})
	if got := card.Head(); len(got) != 1 {
		t.Fatalf("head = %q, want one tag", got)
	}
}

// A plan is shared by every render, so folding a slot must not grow the slice
// the plan holds.
func TestSlotFoldLeavesThePlanAlone(t *testing.T) {
	plan := cardPlan([]string{`<link rel="stylesheet" href="/card.css">`}, []string{"Card"})
	Bind(plan, cardParams{Body: bodyFragment([]string{`<link rel="stylesheet" href="/a.css">`}, "a")})
	Bind(plan, cardParams{Body: bodyFragment([]string{`<link rel="stylesheet" href="/b.css">`}, "b")})
	if len(plan.Head) != 1 {
		t.Fatalf("the plan's own head grew to %q", plan.Head)
	}
	second := Bind(plan, cardParams{Body: bodyFragment([]string{`<link rel="stylesheet" href="/b.css">`}, "b")})
	if strings.Join(second.Head(), "") != `<link rel="stylesheet" href="/card.css"><link rel="stylesheet" href="/b.css">` {
		t.Fatalf("one bind leaked into the next: %q", second.Head())
	}
}

// An absent optional slot contributes nothing and costs nothing.
func TestAbsentSlotContributesNothing(t *testing.T) {
	card := Bind(cardPlan([]string{`<link rel="stylesheet" href="/card.css">`}, []string{"Card"}), cardParams{})
	if got := card.Head(); len(got) != 1 {
		t.Fatalf("head = %q, want only the component's own tag", got)
	}
}

// The capability flags follow the same rule, because a caller decides from them
// whether the response needs the runtime that applies boundaries at all.
func TestSlotFragmentCapabilitiesFold(t *testing.T) {
	ops := Builder[struct{}]{}
	awaiting := Bind(&Plan[struct{}]{HasAwaitBlock: true, Ops: []Op[struct{}]{ops.Static("x")}}, struct{}{})
	card := Bind(cardPlan(nil, nil), cardParams{Body: awaiting})
	if !card.HasAwaitBlock() {
		t.Error("a slot holding an await boundary must be counted")
	}
	if card.HasLiveBlock() {
		t.Error("nothing here owns a live boundary")
	}

	living := Bind(&Plan[struct{}]{HasAwaitBlock: true, HasLiveBlock: true, Ops: []Op[struct{}]{ops.Static("x")}}, struct{}{})
	if !Bind(cardPlan(nil, nil), cardParams{Body: living}).HasLiveBlock() {
		t.Error("a slot holding a live boundary must be counted")
	}
}

// A wrapper's named slots fold the same way. Its unnamed one is filled by the
// chain after binding, and the chain merges that member itself.
func TestWrapperSlotHeadIsMerged(t *testing.T) {
	type panelParams struct {
		Header   Fragment
		Children Fragment
	}
	ops := Builder[panelParams]{}
	plan := &Plan[panelParams]{
		Head:        []string{`<link rel="stylesheet" href="/panel.css">`},
		HeadSources: []string{"Panel"},
		Slots:       func(p panelParams) []Fragment { return []Fragment{p.Header, p.Children} },
		Ops: []Op[panelParams]{
			ops.Static("<section>"),
			ops.Slot(func(p panelParams) Fragment { return p.Header }, nil),
			ops.Slot(func(p panelParams) Fragment { return p.Children }, nil),
			ops.Static("</section>"),
		},
	}
	wrapper := BindWrapper(plan, panelParams{
		Header: bodyFragment([]string{`<link rel="stylesheet" href="/header.css">`}, "h"),
	}, func(p *panelParams, children Fragment) { p.Children = children })

	if got := wrapper.Head(); len(got) != 2 || got[1] != `<link rel="stylesheet" href="/header.css">` {
		t.Fatalf("head = %q, want the wrapper's own tag and its filled slot's", got)
	}
	merged := MergeHead([]Wrapper{wrapper}, bodyFragment([]string{`<link rel="stylesheet" href="/page.css">`}, "p"))
	if len(merged) != 3 {
		t.Fatalf("merged = %q, want the wrapper, its slot, and the page", merged)
	}
}

// The required set is what Head cannot be. A head entry is a ready-to-write tag,
// so a caller reading it gets markup rather than something it can compare
// against what the document already carries, or refuse.
func TestAssetsFoldThroughSlotsAndTheChain(t *testing.T) {
	library := Asset{ID: "lib.script.abc123", Type: AssetTypeScript, URL: "/public/lib.script.abc123.js"}
	sheet := Asset{ID: "unit.style.def456", Type: AssetTypeStyle, URL: "/public/unit.style.def456.css"}

	ops := Builder[struct{}]{}
	inner := Bind(&Plan[struct{}]{Assets: []Asset{library}, Ops: []Op[struct{}]{ops.Static("inner")}}, struct{}{})

	plan := cardPlan(nil, nil)
	plan.Assets = []Asset{sheet}
	card := Bind(plan, cardParams{Body: inner})

	got := card.Assets()
	if len(got) != 2 || got[0] != sheet || got[1] != library {
		t.Fatalf("assets = %+v, want the component's own and its slot's", got)
	}
	// Two binds of one plan must not accumulate.
	if len(plan.Assets) != 1 {
		t.Fatalf("the plan's own set grew to %+v", plan.Assets)
	}

	// The chain aggregate answers the question a document shell asks: what does
	// this whole page need, before anything is rendered.
	shell, _ := shellPlan(nil)
	wrapper := BindWrapper(shell, struct{ Children Fragment }{}, func(p *struct{ Children Fragment }, children Fragment) {
		p.Children = children
	})
	merged := MergeAssets([]Wrapper{wrapper}, card)
	if len(merged) != 2 {
		t.Fatalf("merged = %+v, want the page's two", merged)
	}
}

// Three components requiring one asset produce one entry, because identity is
// the file rather than the component that asked for it.
func TestAssetsDeduplicateByIdentity(t *testing.T) {
	shared := Asset{ID: "unit.style.def456", Type: AssetTypeStyle, URL: "/public/unit.style.def456.css"}
	ops := Builder[struct{}]{}
	inner := Bind(&Plan[struct{}]{Assets: []Asset{shared}, Ops: []Op[struct{}]{ops.Static("inner")}}, struct{}{})
	plan := cardPlan(nil, nil)
	plan.Assets = []Asset{shared}
	if got := Bind(plan, cardParams{Body: inner}).Assets(); len(got) != 1 {
		t.Fatalf("assets = %+v, want one", got)
	}
}

// A component requiring nothing carries nothing, which is what keeps a project
// extracting no asset unaffected.
func TestAComponentWithNoAssetsRequiresNone(t *testing.T) {
	if got := bodyFragment(nil, "plain").Assets(); got != nil {
		t.Fatalf("assets = %+v, want none", got)
	}
	if got := MergeAssets(nil, bodyFragment(nil, "plain")); got != nil {
		t.Fatalf("merged = %+v, want none", got)
	}
}
