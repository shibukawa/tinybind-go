package htmlbind_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

type scopeParams struct{ Text string }

// scopePlan builds the shape generated code emits for one component: the two
// scope declarations, and a cache policy when the component stores.
func scopePlan(declaresPrivate, declaresPublic bool, source string, cache *htmlbind.CachePolicy[scopeParams]) *htmlbind.Plan[scopeParams] {
	ops := htmlbind.Builder[scopeParams]{}
	return &htmlbind.Plan[scopeParams]{
		DeclaresPrivate: declaresPrivate,
		DeclaresPublic:  declaresPublic,
		PrivateSource:   source,
		Cache:           cache,
		Ops: []htmlbind.Op[scopeParams]{
			ops.Text(func(p scopeParams) string { return p.Text }),
		},
	}
}

func scopeFragment(declaresPrivate, declaresPublic bool, source string) htmlbind.Fragment {
	return htmlbind.Bind(scopePlan(declaresPrivate, declaresPublic, source, nil), scopeParams{Text: "x"})
}

type wrapperParams struct {
	Text     string
	Children htmlbind.Fragment
}

func scopeWrapper(declaresPrivate, declaresPublic bool, source string) htmlbind.Wrapper {
	ops := htmlbind.Builder[wrapperParams]{}
	plan := &htmlbind.Plan[wrapperParams]{
		DeclaresPrivate: declaresPrivate,
		DeclaresPublic:  declaresPublic,
		PrivateSource:   source,
		Ops: []htmlbind.Op[wrapperParams]{
			ops.Static("<div>"),
			ops.Slot(func(p wrapperParams) htmlbind.Fragment { return p.Children }, nil),
			ops.Static("</div>"),
		},
	}
	return htmlbind.BindWrapper(plan, wrapperParams{Text: "w"},
		func(target *wrapperParams, children htmlbind.Fragment) { target.Children = children })
}

// Undeclared reports private. That is the framework default rather than a
// property of the annotation: a component treated as shared that is actually
// per-reader serves one reader's output to another, and the other mistake costs
// a miss. The two are not comparable.
func TestUndeclaredFragmentIsPrivate(t *testing.T) {
	fragment := scopeFragment(false, false, "")
	if !fragment.IsPrivate() {
		t.Fatal("an undeclared fragment reported shared")
	}
	// Private by default names nothing, because only a declaration can be
	// pointed at and an author asking why has to be sent somewhere real.
	if source := fragment.PrivateSource(); source != "" {
		t.Fatalf("PrivateSource() = %q, want none", source)
	}
}

func TestChainScopeRules(t *testing.T) {
	private := func(source string) htmlbind.Wrapper { return scopeWrapper(true, false, source) }
	public := func() htmlbind.Wrapper { return scopeWrapper(false, true, "") }
	undeclared := func() htmlbind.Wrapper { return scopeWrapper(false, false, "") }

	for _, tc := range []struct {
		name     string
		wrappers []htmlbind.Wrapper
		leaf     htmlbind.Fragment
		want     bool
		source   string
	}{
		{
			// Nothing declared anywhere is the ordinary case, and the default
			// decides it.
			"nothing declared", nil, scopeFragment(false, false, ""), true, "",
		},
		{
			// The shape a login-gated application has: one declaration on the
			// authenticated layout covers every page beneath it.
			"outermost declares private",
			[]htmlbind.Wrapper{private("AuthLayout"), undeclared()},
			scopeFragment(false, false, ""), true, "AuthLayout",
		},
		{
			// A wrapper contains everything below it, so a public declaration on
			// the outermost member covers the whole chain.
			"outermost declares public",
			[]htmlbind.Wrapper{public(), undeclared()},
			scopeFragment(false, false, ""), false, "",
		},
		{
			// Private wins wherever it sits. This is also the case generation
			// could not have refused, because the chain is assembled here.
			"a leaf declares private under a public layout",
			[]htmlbind.Wrapper{public()},
			scopeFragment(true, false, "AccountPanel"), true, "AccountPanel",
		},
		{
			"an inner wrapper declares private",
			[]htmlbind.Wrapper{public(), private("Sidebar")},
			scopeFragment(false, false, ""), true, "Sidebar",
		},
		{
			// A declaration further in covers only what it wraps and says nothing
			// about the markup wrapped around it, so it cannot make the whole
			// response shared on its own. The undeclared layout is in the
			// response too.
			"a leaf declares public under an undeclared layout",
			[]htmlbind.Wrapper{undeclared()},
			scopeFragment(false, true, ""), true, "",
		},
		{
			// With no layout the leaf is the outermost member, so its assertion
			// does cover the whole response.
			"a leaf declares public with no layout",
			nil, scopeFragment(false, true, ""), false, "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := htmlbind.IsPrivate(tc.wrappers, tc.leaf); got != tc.want {
				t.Fatalf("IsPrivate() = %v, want %v", got, tc.want)
			}
			if got := htmlbind.PrivateSource(tc.wrappers, tc.leaf); got != tc.source {
				t.Fatalf("PrivateSource() = %q, want %q", got, tc.source)
			}
		})
	}
}

// A component handed in through a slot renders inside its owner, so its
// per-reader output makes the owner per-reader too. The public direction does
// not fold: a slot argument asserting it is shared says nothing about the markup
// wrapped around it.
func TestSlotFoldsPrivateButNotPublic(t *testing.T) {
	ops := htmlbind.Builder[wrapperParams]{}
	plan := &htmlbind.Plan[wrapperParams]{
		Slots: func(p wrapperParams) []htmlbind.Fragment { return []htmlbind.Fragment{p.Children} },
		Ops: []htmlbind.Op[wrapperParams]{
			ops.Slot(func(p wrapperParams) htmlbind.Fragment { return p.Children }, nil),
		},
	}
	owner := htmlbind.Bind(plan, wrapperParams{Children: scopeFragment(true, false, "AccountPanel")})
	if !owner.IsPrivate() {
		t.Fatal("a private slot argument left its owner shared")
	}
	if source := owner.PrivateSource(); source != "AccountPanel" {
		t.Fatalf("PrivateSource() = %q, want %q", source, "AccountPanel")
	}

	// The owner declares public and is handed a component that says it is
	// shared. Nothing about the argument changes what the owner declared.
	publicPlan := &htmlbind.Plan[wrapperParams]{
		DeclaresPublic: true,
		Slots:          func(p wrapperParams) []htmlbind.Fragment { return []htmlbind.Fragment{p.Children} },
		Ops: []htmlbind.Op[wrapperParams]{
			ops.Slot(func(p wrapperParams) htmlbind.Fragment { return p.Children }, nil),
		},
	}
	shared := htmlbind.Bind(publicPlan, wrapperParams{Children: scopeFragment(false, true, "")})
	if shared.IsPrivate() {
		t.Fatal("a public owner holding a public argument reported private")
	}
}

// The key gains a framed prefix, and framing is what keeps a scope value from
// spelling out another component's key.
func TestScopedKeyIsFramedAndSeparatesReaders(t *testing.T) {
	store := htmlbind.NewMemoryCache(16)
	policy := htmlbind.CachePolicy[scopeParams]{
		ID: "pages/x.tb.html:Panel:abc", TTL: time.Minute, Scoped: true,
		Key: func(p scopeParams) string { return htmlbind.KeyString(p.Text) },
	}
	plan := scopePlan(true, false, "Panel", &policy)

	render := func(scope, text string) string {
		var out bytes.Buffer
		if err := htmlbind.Render(&out, htmlbind.Bind(plan, scopeParams{Text: text}),
			htmlbind.WithCache(store), htmlbind.WithCacheScope(scope)); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if got := render("reader-1", "one"); got != "one" {
		t.Fatalf("render = %q, want %q", got, "one")
	}
	if store.Len() != 1 {
		t.Fatalf("one scope stored %d entries, want 1", store.Len())
	}
	// The same parameters under another scope must not read the first entry.
	if got := render("reader-2", "two"); got != "two" {
		t.Fatalf("a second reader read the first reader's entry: %q", got)
	}
	if store.Len() != 2 {
		t.Fatalf("two scopes stored %d entries, want 2", store.Len())
	}

	// Framing is the reason a scope value cannot be split off wrongly: a value
	// carrying the separator cannot alias a different scope-and-key pair.
	if a, b := framedKey("a:b", "c"), framedKey("a", "b:c"); a == b {
		t.Fatalf("two different scope and key pairs framed identically: %q", a)
	}
}

func framedKey(scope, id string) string { return htmlbind.KeyString(scope) + htmlbind.KeyString(id) }

// The fallback rather than the design. An entry under an empty scope would be a
// shared entry wearing a private label, so the miss is deliberate — and the
// output has to be identical to what a stored render produces.
func TestPrivateWithNoScopeStoresNothingAndRendersTheSame(t *testing.T) {
	store := htmlbind.NewMemoryCache(16)
	policy := htmlbind.CachePolicy[scopeParams]{
		ID: "pages/x.tb.html:Panel:abc", TTL: time.Minute, Scoped: true,
		Key: func(p scopeParams) string { return htmlbind.KeyString(p.Text) },
	}
	plan := scopePlan(true, false, "Panel", &policy)

	var unscoped bytes.Buffer
	if err := htmlbind.Render(&unscoped, htmlbind.Bind(plan, scopeParams{Text: "one"}), htmlbind.WithCache(store)); err != nil {
		t.Fatal(err)
	}
	if store.Len() != 0 {
		t.Fatalf("a private component with no scope stored %d entries, want 0", store.Len())
	}
	var scoped bytes.Buffer
	if err := htmlbind.Render(&scoped, htmlbind.Bind(plan, scopeParams{Text: "one"}),
		htmlbind.WithCache(store), htmlbind.WithCacheScope("reader-1")); err != nil {
		t.Fatal(err)
	}
	if scoped.String() != unscoped.String() {
		t.Fatalf("an unstored render differs from a stored one:\n%q\n%q", unscoped.String(), scoped.String())
	}
}

// A public component keeps the key it had before scoping existed, which is what
// makes the opt-out a true return to the previous behaviour rather than a second
// spelling of it.
func TestPublicComponentIgnoresTheScopeValue(t *testing.T) {
	store := htmlbind.NewMemoryCache(16)
	policy := htmlbind.CachePolicy[scopeParams]{
		ID: "pages/x.tb.html:Panel:abc", TTL: time.Minute,
		Key: func(p scopeParams) string { return htmlbind.KeyString(p.Text) },
	}
	plan := scopePlan(false, true, "", &policy)
	for _, scope := range []string{"reader-1", "reader-2", ""} {
		var out bytes.Buffer
		if err := htmlbind.Render(&out, htmlbind.Bind(plan, scopeParams{Text: "one"}),
			htmlbind.WithCache(store), htmlbind.WithCacheScope(scope)); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "one") {
			t.Fatalf("render lost its output under scope %q: %q", scope, out.String())
		}
	}
	if store.Len() != 1 {
		t.Fatalf("a public component stored %d entries across three scopes, want 1", store.Len())
	}
}
