package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// bindingOptions declares one binding, standing in for what a framework
// supplies. Nothing about it is i18n-shaped on this side: the name and the
// provider are the embedder's.
func bindingOptions() htmlbind.GenerateOptions {
	return htmlbind.GenerateOptions{
		ImplicitBindings: []htmlbind.ImplicitBinding{{
			Name:     "lang",
			Provider: htmlbind.BindingProvider{Package: "example.com/app/framework", Name: "Lang"},
			VaryAxis: "Accept-Language",
		}},
	}
}

// TestImplicitBindingNeedsNoParameter is the point of the feature: a name in
// scope in every template, with nothing threaded through a chain.
func TestImplicitBindingNeedsNoParameter(t *testing.T) {
	out, err := htmlbind.Generate("b.txt",
		[]byte(`component Page(): html {<p>{lang}</p>}`), bindingOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	source := string(out)
	if !strings.Contains(source, "framework.Lang(ctx)") {
		t.Fatalf("the binding does not lower to its provider:\n%s", source)
	}
	if !strings.Contains(source, "TextCtx(") {
		t.Fatalf("the instruction does not carry the render context:\n%s", source)
	}
	if !strings.Contains(source, `"example.com/app/framework"`) {
		t.Fatalf("the provider package was not imported:\n%s", source)
	}
	if !strings.Contains(source, `"context"`) {
		t.Fatalf("the context import is missing:\n%s", source)
	}
}

// TestImplicitBindingCrossesAChainWithoutThreading covers the case the feature
// exists for: a layout between the shell and the page carries nothing.
func TestImplicitBindingCrossesAChainWithoutThreading(t *testing.T) {
	source := `component Layout(children: html): html {<main><slot required/></main>}
component Page(): html {<Layout><p>{lang}</p></Layout>}`
	if _, err := htmlbind.Generate("b.txt", []byte(source), bindingOptions()); err != nil {
		t.Fatalf("generate failed: %v", err)
	}
}

func TestImplicitBindingInAnAttribute(t *testing.T) {
	out, err := htmlbind.Generate("b.txt",
		[]byte(`component Page(): html {<html lang={lang}><body>x</body></html>}`), bindingOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if !strings.Contains(string(out), "AttrCtx(") {
		t.Fatalf("the attribute instruction does not carry the context:\n%s", out)
	}
	if !strings.Contains(string(out), "htmlbind.Escape(framework.Lang(ctx))") {
		t.Fatalf("the binding is not escaped for its position:\n%s", out)
	}
}

// TestShadowingABindingIsAnError covers every binder, not only the parameter
// list the request named: scope wins over the binding table by construction, so
// any binder that could take the name has to refuse it.
func TestShadowingABindingIsAnError(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"parameter",
			`component Page(lang: string): html {<p>{lang}</p>}`,
			"parameter lang shadows",
		},
		{
			"val binding",
			`external Pick(): string` + "\n" + `component Page(): html {<p>{val lang = Pick()}{lang}</p>}`,
			"val binding lang shadows",
		},
		{
			"loop variable",
			`component Page(items: string[]): html {{for lang in items}<p>{lang}</p>{/for}}`,
			"loop variable lang shadows",
		},
		{
			"loop index",
			`component Page(items: string[]): html {{for item, lang in items}<p>{item}</p>{/for}}`,
			"loop index lang shadows",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := htmlbind.Generate("b.txt", []byte(test.source), bindingOptions())
			if err == nil {
				t.Fatalf("%s shadowed the binding without an error", test.name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to name the collision as %q", err, test.want)
			}
		})
	}
}

// TestDeclaringABindingNobodyReadsChangesNothing keeps the cost of declaring
// one at zero, which is what lets a framework declare its whole set.
func TestDeclaringABindingNobodyReadsChangesNothing(t *testing.T) {
	source := `component Page(name: string): html {<p>{name}</p>}`
	withBindings, err := htmlbind.Generate("b.txt", []byte(source), bindingOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	without, err := htmlbind.Generate("b.txt", []byte(source), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if string(withBindings) != string(without) {
		t.Fatalf("declaring an unused binding changed the output:\n%s\n---\n%s", withBindings, without)
	}
}

func TestBindingRegistrationMistakes(t *testing.T) {
	cases := []struct {
		name    string
		binding htmlbind.ImplicitBinding
		want    string
	}{
		{"no name", htmlbind.ImplicitBinding{Provider: htmlbind.BindingProvider{Name: "Lang"}}, "has no name"},
		{"bad name", htmlbind.ImplicitBinding{Name: "Lang", Provider: htmlbind.BindingProvider{Name: "Lang"}}, "lowerCamelCase"},
		{"no provider", htmlbind.ImplicitBinding{Name: "lang"}, "has no provider"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			options := htmlbind.GenerateOptions{ImplicitBindings: []htmlbind.ImplicitBinding{test.binding}}
			_, err := htmlbind.Generate("b.txt", []byte(`component Page(): html {<p>x</p>}`), options)
			if err == nil {
				t.Fatalf("%s was accepted", test.name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

// TestCachedComponentKeysOnTheBinding replaces the earlier refusal: a stored
// body is now distinguished per binding value, which is what
// decision:implicit-binding-cache-identity settles.
func TestCachedComponentKeysOnTheBinding(t *testing.T) {
	source := "@cache(ttl: \"5m\")\ncomponent Page(): html {<p>{lang}</p>}"
	out, err := htmlbind.Generate("b.txt", []byte(source), bindingOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	rendered := string(out)
	if !strings.Contains(rendered, "Bindings: func(ctx context.Context) string") {
		t.Fatalf("the policy does not key on the binding:\n%s", rendered)
	}
	if !strings.Contains(rendered, "htmlbind.KeyString(framework.Lang(ctx))") {
		t.Fatalf("the binding value is not framed into the key:\n%s", rendered)
	}
}

// TestCachedComponentKeysOnABindingItReachesThroughACall covers the call graph:
// a nested read makes the caller's output depend on the binding.
func TestCachedComponentKeysOnABindingItReachesThroughACall(t *testing.T) {
	source := "component Inner(): html {<p>{lang}</p>}\n\n@cache(ttl: \"5m\")\ncomponent Page(): html {<Inner/>}"
	out, err := htmlbind.Generate("b.txt", []byte(source), bindingOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if !strings.Contains(string(out), "Bindings: func(ctx context.Context) string") {
		t.Fatalf("a binding reached through a call did not enter the key:\n%s", out)
	}
}

// TestCachedComponentReadingNoBindingIsUnchanged keeps the promise that a
// project declaring none pays nothing.
func TestCachedComponentReadingNoBindingIsUnchanged(t *testing.T) {
	source := "@cache(ttl: \"5m\")\ncomponent Page(name: string): html {<p>{name}</p>}"
	withBindings, err := htmlbind.Generate("b.txt", []byte(source), bindingOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	without, err := htmlbind.Generate("b.txt", []byte(source), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if string(withBindings) != string(without) {
		t.Fatalf("a cached component reading no binding changed:\n%s\n---\n%s", withBindings, without)
	}
}

// TestBindingVaryAxisFoldsIntoThePlan is the outside-the-component half: a
// caller writing a Vary header has to see what a nested component depends on.
func TestBindingVaryAxisFoldsIntoThePlan(t *testing.T) {
	source := "component Inner(): html {<p>{lang}</p>}\n\ncomponent Page(): html {<Inner/>}"
	out, err := htmlbind.Generate("b.txt", []byte(source), bindingOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if !strings.Contains(string(out), `"Accept-Language"`) {
		t.Fatalf("the declared vary axis did not reach the plan:\n%s", out)
	}
}

// TestBindingWithNoVaryAxisContributesNone is what an application carrying the
// value in its URL declares: two languages are already two URLs, and an axis
// would only fragment an intermediary's cache.
func TestBindingWithNoVaryAxisContributesNone(t *testing.T) {
	options := htmlbind.GenerateOptions{
		ImplicitBindings: []htmlbind.ImplicitBinding{{
			Name:     "lang",
			Provider: htmlbind.BindingProvider{Package: "example.com/app/framework", Name: "Lang"},
		}},
	}
	out, err := htmlbind.Generate("b.txt", []byte(`component Page(): html {<p>{lang}</p>}`), options)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if strings.Contains(string(out), "Vary:") {
		t.Fatalf("a binding declaring no axis contributed one:\n%s", out)
	}
}

// segmentOptions declares a path-segment binding, the kind permitted into a URL
// attribute.
func segmentOptions() htmlbind.GenerateOptions {
	return htmlbind.GenerateOptions{
		ImplicitBindings: []htmlbind.ImplicitBinding{{
			Name:        "lang",
			Provider:    htmlbind.BindingProvider{Package: "example.com/app/framework", Name: "Lang"},
			PathSegment: true,
		}},
	}
}

// TestPathSegmentBindingReachesAURLAttribute is the one exception to the url
// type gate, and the reason E is an amendment to
// requirement:url-attribute-scheme-safety rather than an additive feature.
func TestPathSegmentBindingReachesAURLAttribute(t *testing.T) {
	out, err := htmlbind.Generate("b.txt",
		[]byte(`component Page(): html {<a href="/{lang}/about">go</a>}`), segmentOptions())
	if err != nil {
		t.Fatalf("a path-segment binding was refused: %v", err)
	}
	if !strings.Contains(string(out), `htmlbind.URLPathSegment("/", framework.Lang(ctx), true)`) {
		t.Fatalf("the segment does not go through the helper:\n%s", out)
	}
	// The separator is written by the helper, so the static part must not also
	// carry it or an empty segment would leave a doubled slash.
	if strings.Contains(string(out), `"/" + htmlbind.URLPathSegment`) {
		t.Fatalf("the separator was emitted twice:\n%s", out)
	}
}

// TestPathSegmentCollapseShapes covers the three forms the requirement names,
// by reading the emitted arguments rather than by rendering.
func TestPathSegmentCollapseShapes(t *testing.T) {
	cases := []struct {
		name     string
		markup   string
		collapse string
	}{
		{"path follows the segment", `<a href="/{lang}/about">x</a>`, "true"},
		{"only a trailing slash follows", `<a href="/{lang}/">x</a>`, "true"},
		{"the segment is the whole tail", `<a href="/{lang}">x</a>`, "false"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			out, err := htmlbind.Generate("b.txt",
				[]byte(`component Page(): html {`+test.markup+`}`), segmentOptions())
			if err != nil {
				t.Fatalf("generate failed: %v", err)
			}
			want := `framework.Lang(ctx), ` + test.collapse + `)`
			if !strings.Contains(string(out), want) {
				t.Fatalf("collapse flag is not %s:\n%s", test.collapse, out)
			}
		})
	}
}

// TestOrdinaryBindingStillCannotReachAURLAttribute keeps the exception scoped to
// the kind. A rule stated over emptiness, or over bindings in general, would
// widen the gate the security review narrowed.
func TestOrdinaryBindingStillCannotReachAURLAttribute(t *testing.T) {
	_, err := htmlbind.Generate("b.txt",
		[]byte(`component Page(): html {<a href="/{lang}/about">go</a>}`), bindingOptions())
	if err == nil {
		t.Fatal("an ordinary binding reached a URL attribute")
	}
	if !strings.Contains(err.Error(), "requires url") {
		t.Fatalf("error = %v, want the url type gate", err)
	}
}

// TestAPlainStringStillCannotReachAURLAttribute is the property the gate exists
// for, unchanged by the exception.
func TestAPlainStringStillCannotReachAURLAttribute(t *testing.T) {
	_, err := htmlbind.Generate("b.txt",
		[]byte(`component Page(q: string): html {<a href="/search/{q}">go</a>}`), segmentOptions())
	if err == nil {
		t.Fatal("a plain string reached a URL attribute")
	}
	if !strings.Contains(err.Error(), "requires url") {
		t.Fatalf("error = %v, want the url type gate", err)
	}
}

// TestPathSegmentIsNotCollapsedOutsideAURLContext scopes the collapse to URL
// attributes, since collapsing in prose would be wrong.
func TestPathSegmentIsNotCollapsedOutsideAURLContext(t *testing.T) {
	out, err := htmlbind.Generate("b.txt",
		[]byte(`component Page(): html {<p>/{lang}/ is a prefix</p>}`), segmentOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if strings.Contains(string(out), "URLPathSegment") {
		t.Fatalf("a segment in prose went through the URL helper:\n%s", out)
	}
}

// TestPathSegmentBindingMustReturnAString keeps the registration honest: the
// helper percent-encodes a string, so a typed provider has nothing to encode.
func TestPathSegmentBindingMustReturnAString(t *testing.T) {
	options := htmlbind.GenerateOptions{
		ImplicitBindings: []htmlbind.ImplicitBinding{{
			Name:        "lang",
			Provider:    htmlbind.BindingProvider{Package: "example.com/app/framework", Name: "Lang", Result: "framework.Locale"},
			PathSegment: true,
		}},
	}
	_, err := htmlbind.Generate("b.txt", []byte(`component Page(): html {<p>x</p>}`), options)
	if err == nil {
		t.Fatal("a typed path-segment binding was accepted")
	}
	if !strings.Contains(err.Error(), "must return a string") {
		t.Fatalf("error = %v, want the registration refusal", err)
	}
}

// TestPathSegmentInHarderPositions locks the emitter's index arithmetic, which
// is where a collapse rule goes wrong quietly: the separator a segment takes
// over has to come from the part before it, and a second segment must not
// inherit the first one's.
func TestPathSegmentInHarderPositions(t *testing.T) {
	cases := []struct {
		name   string
		markup string
		want   []string
	}{
		{
			"segment in the middle of a path",
			`<a href="/a/{lang}/b">x</a>`,
			[]string{`"/a" + htmlbind.URLPathSegment("/", framework.Lang(ctx), true) + "/b"`},
		},
		{
			"two segments in one path",
			`<a href="/{lang}/{lang}/x">y</a>`,
			[]string{
				`htmlbind.URLPathSegment("/", framework.Lang(ctx), true)`,
				`+ "/x"`,
			},
		},
		{
			"segment with no separator before it",
			`<a href="{lang}/about">x</a>`,
			[]string{`htmlbind.URLPathSegment("", framework.Lang(ctx), true)`},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			out, err := htmlbind.Generate("b.txt",
				[]byte(`component Page(): html {`+test.markup+`}`), segmentOptions())
			if err != nil {
				t.Fatalf("generate failed: %v", err)
			}
			for _, want := range test.want {
				if !strings.Contains(string(out), want) {
					t.Fatalf("output lacks %q:\n%s", want, out)
				}
			}
			// A separator must never be written twice: once by the static part
			// and once by the helper.
			if strings.Contains(string(out), `"/" + htmlbind.URLPathSegment("/"`) {
				t.Fatalf("a separator was emitted twice:\n%s", out)
			}
		})
	}
}
