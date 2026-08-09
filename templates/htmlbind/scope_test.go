package htmlbind_test

import (
	"strings"
	"testing"

	htmlbind "github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// A bare @cache declares private, which is the default the whole design turns
// on: an author who wanted a shared entry says so, because the cost of that
// mistake is a miss and the cost of the other one is one reader's output served
// to another.
func TestBareCacheDeclaresPrivateAndScopesItsKey(t *testing.T) {
	source := "package pages\n\n@cache(ttl: \"5m\")\ncomponent Panel(id: string): html {\n<div>{id}</div>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	for _, want := range []string{"Scoped: true", "DeclaresPrivate: true", `PrivateSource:   "Panel"`} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated code is missing %q:\n%s", want, generated)
		}
	}
	if strings.Contains(generated, "DeclaresPublic") {
		t.Fatalf("a private component declared public:\n%s", generated)
	}
}

// The opt-out keys on parameters alone, which is the behaviour every cached
// component had before scoping existed.
func TestPublicScopeLeavesTheKeyUnscoped(t *testing.T) {
	source := "package pages\n\n@cache(ttl: \"5m\", scope: \"public\")\ncomponent Panel(id: string): html {\n<div>{id}</div>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	if strings.Contains(generated, "Scoped: true") {
		t.Fatalf("a public component scoped its key:\n%s", generated)
	}
	if !strings.Contains(generated, "DeclaresPublic: true") {
		t.Fatalf("the public declaration never reached the plan:\n%s", generated)
	}
	if strings.Contains(generated, "DeclaresPrivate") {
		t.Fatalf("a public component declared private:\n%s", generated)
	}
}

// The declaration folds upward over the call graph, because a private
// component's bytes end up inside whatever renders it. A public declaration does
// not: asserting that a subtree is shared says nothing about the markup wrapped
// around it.
func TestPrivateFoldsUpwardAndPublicDoesNot(t *testing.T) {
	source := "package pages\n\n@cache(ttl: \"5m\")\ncomponent Panel(): html {\n<div>x</div>\n}\n" +
		"component Middle(): html {\n<section><Panel /></section>\n}\n" +
		"export component Page(): html {\n<main><Middle /></main>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	// Three plans, and every one of them has to carry the bit and name Panel.
	if got := strings.Count(generated, "DeclaresPrivate: true"); got != 3 {
		t.Fatalf("DeclaresPrivate reached %d plans, want 3:\n%s", got, generated)
	}
	if got := strings.Count(generated, `PrivateSource:   "Panel"`); got != 3 {
		t.Fatalf("PrivateSource named Panel on %d plans, want 3:\n%s", got, generated)
	}

	shared := "package pages\n\n@cache(ttl: \"5m\", scope: \"public\")\ncomponent Panel(): html {\n<div>x</div>\n}\n" +
		"export component Page(): html {\n<main><Panel /></main>\n}\n"
	generated = generateWith(t, shared, htmlbind.GenerateOptions{})
	if got := strings.Count(generated, "DeclaresPublic: true"); got != 1 {
		t.Fatalf("DeclaresPublic reached %d plans, want only the declaring one:\n%s", got, generated)
	}
}

// A layout carries the annotation to declare scope over the chain beneath it.
// It stores nothing, so it emits no policy at all, and none of the eligibility
// rules apply to it: each of them exists because bytes are stored.
func TestLayoutDeclaresScopeAndStoresNothing(t *testing.T) {
	source := "package pages\n\n@cache(scope: \"private\")\nexport component Layout(children: html): html {\n" +
		"<html><head><title>x</title></head><body><slot required /></body></html>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	if !strings.Contains(generated, "DeclaresPrivate: true") {
		t.Fatalf("the layout's declaration never reached the plan:\n%s", generated)
	}
	// A shell owning the document head and a slot owner are both refused a
	// stored form, and neither refusal has anything to say about a declaration.
	if strings.Contains(generated, "CachePolicy") || strings.Contains(generated, "Cache: &") {
		t.Fatalf("a declaring layout emitted a cache policy:\n%s", generated)
	}
}

// A declaration has no key, so nothing about its parameters needs encoding. The
// record encoder is the visible half: emitting one for a component that stores
// nothing is dead code, and the same walk reaches a declaring layout's html
// parameter, which has no encoding at all.
func TestDeclaringComponentEmitsNoKeyEncoder(t *testing.T) {
	source := "package pages\n\ntype Plan {\n  name: string\n}\n\n" +
		"@cache(scope: \"private\")\ncomponent Panel(plan: Plan): html {\n<div>{plan.name}</div>\n}\n" +
		"export component Page(plan: Plan): html {\n<main><Panel plan={plan} /></main>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	if strings.Contains(generated, "_tinybindKeyPlan") {
		t.Fatalf("a component that stores nothing emitted a key encoder:\n%s", generated)
	}
	if !strings.Contains(generated, "DeclaresPrivate: true") {
		t.Fatalf("the declaration never reached the plan:\n%s", generated)
	}

	// The same component with a ttl does need one, which is what shows the gate
	// is on storage rather than on the annotation.
	storing := strings.Replace(source, "@cache(scope: \"private\")", "@cache(ttl: \"5m\")", 1)
	generated = generateWith(t, storing, htmlbind.GenerateOptions{})
	if !strings.Contains(generated, "_tinybindKeyPlan") {
		t.Fatalf("a storing component lost its key encoder:\n%s", generated)
	}
}

// The refusal that makes the assertion mean something. It fires over the call
// graph and reports the position the scope was written at, because an author who
// wrote public and got private needs to know where to look.
func TestPublicOverDeclaredPrivateIsRefused(t *testing.T) {
	source := "package pages\n\n@cache(scope: \"private\")\ncomponent Account(): html {\n<div>x</div>\n}\n" +
		"@cache(ttl: \"5m\", scope: \"public\")\ncomponent Panel(): html {\n<div><Account /></div>\n}\n" +
		"export component Page(): html {\n<main><Panel /></main>\n}\n"
	message := generateError(t, source, htmlbind.GenerateOptions{})
	for _, want := range []string{"component Panel cannot declare @cache scope public", "Account declares private"} {
		if !strings.Contains(message, want) {
			t.Fatalf("diagnostic = %q, want it to mention %q", message, want)
		}
	}
}

// The other half of the same rule, and the one that keeps it usable. An
// undeclared component inherits the assertion; if it contradicted one, nothing
// could ever be declared public.
func TestUndeclaredComponentDoesNotBlockAPublicAssertion(t *testing.T) {
	source := "package pages\n\ncomponent Row(): html {\n<li>x</li>\n}\n" +
		"@cache(ttl: \"5m\", scope: \"public\")\ncomponent Panel(): html {\n<ul><Row /></ul>\n}\n" +
		"export component Page(): html {\n<main><Panel /></main>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	if !strings.Contains(generated, "DeclaresPublic: true") {
		t.Fatalf("an undeclared callee blocked a public assertion:\n%s", generated)
	}
}

// A component that cannot store can still declare its scope, which is what
// closes the hole a ttl-always-required rule would leave: a page that awaits is
// ineligible for storage and would otherwise be unable to assert anything.
func TestAwaitingComponentMayDeclareScopeWithoutATTL(t *testing.T) {
	source := "package pages\n\ntype User {\n  name: string\n}\n\nexternal async LoadUser(id: string): User\n\n" +
		"@cache(scope: \"public\")\nexport component Page(id: string): html {\n<main>\n" +
		"{await user = LoadUser(id)}\n<p>{user.name}</p>\n{fallback}\n<p>loading</p>\n{/await}\n</main>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	if !strings.Contains(generated, "DeclaresPublic: true") {
		t.Fatalf("an awaiting component could not declare its scope:\n%s", generated)
	}
	if strings.Contains(generated, "CachePolicy") {
		t.Fatalf("a declaration produced a cache policy:\n%s", generated)
	}
}

func TestScopeDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{
			"an unknown scope value",
			"@cache(scope: \"tenant\")\ncomponent Bad(): html {<p>x</p>}",
			"@cache scope is not private or public: tenant",
		},
		{
			"an unknown argument",
			"@cache(ttl: \"5m\", region: \"eu\")\ncomponent Bad(): html {<p>x</p>}",
			"unknown @cache argument region",
		},
		{
			// The eligibility rules still hold for anything that stores.
			"a storing component still cannot own the head",
			"@cache(ttl: \"5m\")\ncomponent Bad(): html {<html><head><title>x</title></head><body>y</body></html>}",
			"cannot own the document head",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := generateError(t, "package pages\n\n"+tc.source+"\n", htmlbind.GenerateOptions{})
			if !strings.Contains(message, tc.want) {
				t.Fatalf("diagnostic = %q, want it to mention %q", message, tc.want)
			}
		})
	}
}
