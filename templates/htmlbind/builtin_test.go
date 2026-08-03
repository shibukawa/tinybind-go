package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// appNonce stands in for a framework element whose value is per-request: its
// markup is fixed, and the value never enters template scope.
//
// It is deliberately not csrf-token. That was this seam's worked example until
// requirement:csrf-token-rendering made CSRF native, and leaving it here would
// suggest a framework still has to build one.
func appNonce() htmlbind.BuiltinElement {
	return htmlbind.BuiltinElement{
		Name:   "app-nonce",
		Shape:  htmlbind.BuiltinMarkup,
		Markup: `<meta name="{{.FieldName}}" content="{{.Token}}">`,
		Vary:   []string{"Cookie"},
		Provider: &htmlbind.ElementProvider{
			Name:   "NonceFor",
			Result: "Nonce",
		},
	}
}

// otelTracing is the other half of the design: a parameterized element with no
// provider, which folds entirely into static bytes.
func otelTracing() htmlbind.BuiltinElement {
	return htmlbind.BuiltinElement{
		Name:   "otel-tracing",
		Params: []htmlbind.ElementParam{{Name: "service-name", Type: "string", Required: true}},
		Markup: `<meta name="otel-service" content="{{.ServiceName}}">`,
	}
}

func generateWith(t *testing.T, source string, options htmlbind.GenerateOptions) string {
	t.Helper()
	generated, err := htmlbind.Generate("page.tb.html", []byte(source), options)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return string(generated)
}

func generateError(t *testing.T, source string, options htmlbind.GenerateOptions) string {
	t.Helper()
	if _, err := htmlbind.Generate("page.tb.html", []byte(source), options); err != nil {
		return err.Error()
	}
	t.Fatal("want a generation error")
	return ""
}

// A builtin element is written by its bare name, with no prefix and no import,
// and the value it renders never enters template scope. That last part is the
// whole reason it is a seam rather than sugar over a function call: an author
// cannot interpolate the token elsewhere, because no name is bound to it.
func TestBuiltinElementRendersAPerRequestValue(t *testing.T) {
	source := "package pages\n\nexport component Page(): html {\n" +
		"<div><app-nonce /><app-nonce /></div>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{
		BuiltinElements: []htmlbind.BuiltinElement{appNonce()},
	})
	if !strings.Contains(generated, `htmlbind.Provide("app-nonce", "NonceFor", NonceFor`) {
		t.Fatalf("no provider step:\n%s", generated)
	}
	// The fixed part of the markup is folded into static bytes, so it costs the
	// same as if the author had typed it.
	if !strings.Contains(generated, `{Static: "<meta name=\"`) {
		t.Fatalf("markup was not folded into static bytes:\n%s", generated)
	}
	if !strings.Contains(generated, "v.FieldName") || !strings.Contains(generated, "v.Token") {
		t.Fatalf("holes do not read the provider result:\n%s", generated)
	}
	// The response now depends on a cookie and nothing in the template says so,
	// which is exactly why the axis is carried on the value a caller holds.
	if !strings.Contains(generated, `Vary:     []string{"Cookie"}`) {
		t.Fatalf("the declared vary axis never reached the plan:\n%s", generated)
	}

	runGeneratedTests(t, []byte(generated), []byte(`package pages

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

type Nonce struct {
	FieldName string
	Token     string
}

var issued int

func NonceFor(ctx context.Context) (Nonce, error) {
	if failing {
		return Nonce{}, errors.New("no session")
	}
	issued++
	return Nonce{FieldName: "app-nonce", Token: hostile}, nil
}

var failing bool
var hostile = "tok-1"

func render(t *testing.T) string {
	t.Helper()
	var out strings.Builder
	err := htmlbind.RenderChain(&out, nil, Page(PageParams{}),
		htmlbind.WithContext(context.Background()))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return out.String()
}

// The same page rendered twice produces two token values and identical
// surrounding bytes.
func TestTwoRendersDifferInTheTokenAlone(t *testing.T) {
	hostile = "tok-a"
	first := render(t)
	hostile = "tok-b"
	second := render(t)
	if first == second {
		t.Fatal("the token did not change between renders")
	}
	if strings.ReplaceAll(first, "tok-a", "X") != strings.ReplaceAll(second, "tok-b", "X") {
		t.Fatalf("more than the token changed:\n%s\n%s", first, second)
	}
	if !strings.Contains(first, `+"`"+`<meta name="app-nonce" content="tok-a">`+"`"+`) {
		t.Fatalf("token markup missing: %s", first)
	}
	// One call per render, not per occurrence. The token reaches the browser in a
	// response header as well as in these inputs, and a header carries one value:
	// two forms holding two different tokens is a bug nobody sees until one of
	// them is submitted.
	if issued != 2 {
		t.Fatalf("provider ran %d times across two renders, want once per render", issued)
	}
	if strings.Count(first, "tok-a") != 2 {
		t.Fatalf("both occurrences must carry the same token: %s", first)
	}
}

// A token carrying a quote cannot break out of the value attribute it sits in.
func TestHostileTokenCannotEscapeTheAttribute(t *testing.T) {
	hostile = `+"`"+`" onload="alert(1)`+"`"+`
	out := render(t)
	if strings.Contains(out, `+"`"+`onload="alert(1)"`+"`"+`) {
		t.Fatalf("the token broke out of its attribute: %s", out)
	}
	if !strings.Contains(out, "&#34; onload=&#34;alert(1)") {
		t.Fatalf("the token was not escaped: %s", out)
	}
}

// A provider failing during the initial pass ends the render, so a caller can
// still choose an error status rather than writing a half document.
func TestProviderFailureEndsTheRender(t *testing.T) {
	failing = true
	defer func() { failing = false }()
	var out strings.Builder
	err := htmlbind.RenderChain(&out, nil, Page(PageParams{}),
		htmlbind.WithContext(context.Background()))
	if err == nil {
		t.Fatal("want the provider failure")
	}
	if !strings.Contains(err.Error(), "app-nonce") {
		t.Fatalf("the failure must name the element, got %v", err)
	}
}

// Rendering with no context at all is the ordinary way a per-request value goes
// missing: a test, a mail body, a static export. It has to say so rather than
// render the absence.
func TestNoContextIsReported(t *testing.T) {
	var out strings.Builder
	err := htmlbind.RenderChain(&out, nil, Page(PageParams{}))
	if err == nil {
		t.Fatal("want a missing-context failure")
	}
	if !errors.Is(err, htmlbind.ErrNoRenderContext) || !strings.Contains(err.Error(), "app-nonce") {
		t.Fatalf("the failure must name the element and its kind, got %v", err)
	}
}

// The composed value reports the axis its element declared, so a caller can
// build a Vary header for a dependency the template never shows.
func TestVaryReachesTheBoundValue(t *testing.T) {
	page := Page(PageParams{})
	if got := page.Vary(); len(got) != 1 || got[0] != "Cookie" {
		t.Fatalf("vary = %q", got)
	}
	if got := htmlbind.MergeVary(nil, page); len(got) != 1 {
		t.Fatalf("chain vary = %q", got)
	}
}
`))
}

// A definition with no provider and no expression parameter reduces entirely to
// static bytes, so the element costs nothing at render time.
func TestParameterlessBuiltinElementFoldsAway(t *testing.T) {
	static := htmlbind.BuiltinElement{
		Name:   "app-banner",
		Markup: `<div class="banner">beta</div>`,
	}
	source := "package pages\n\nexport component Page(): html {\n<main><app-banner /></main>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{
		BuiltinElements: []htmlbind.BuiltinElement{static},
	})
	if strings.Contains(generated, "htmlbind.Provide") {
		t.Fatalf("a parameterless element must add no render-time step:\n%s", generated)
	}
	if !strings.Contains(generated, `><div class=\"banner\">beta</div></main>`) {
		t.Fatalf("the element did not fold into the surrounding static run:\n%s", generated)
	}
}

// A parameterized element with no provider is still free of a provider call:
// its hole is the call site's own expression.
func TestParameterizedBuiltinElementReadsItsAttribute(t *testing.T) {
	source := "package pages\n\nexport component Page(name: string): html {\n" +
		"<head><title>t</title></head><otel-tracing service-name={name} />\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{
		BuiltinElements: []htmlbind.BuiltinElement{otelTracing()},
	})
	if strings.Contains(generated, "htmlbind.Provide") {
		t.Fatalf("no provider is declared, so none may be called:\n%s", generated)
	}
	if !strings.Contains(generated, "Text(func(p PageParams) string { return p.Name })") {
		t.Fatalf("the hole does not read the attribute:\n%s", generated)
	}
}

// A stored body outlives the request that produced it, so a per-request value in
// one is served to whoever asks next. For a token that is a security failure,
// not a staleness bug.
func TestPerRequestElementIsRefusedInsideACachedComponent(t *testing.T) {
	source := "package pages\n\n@cache(ttl: \"1m\")\ncomponent Panel(): html {\n<div><app-nonce /></div>\n}\n" +
		"export component Page(): html {\n<main><Panel /></main>\n}\n"
	message := generateError(t, source, htmlbind.GenerateOptions{
		BuiltinElements: []htmlbind.BuiltinElement{appNonce()},
	})
	for _, want := range []string{"cached component Panel", "app-nonce", "one request's value to the next"} {
		if !strings.Contains(message, want) {
			t.Fatalf("diagnostic = %q, want it to mention %q", message, want)
		}
	}
}

// The same exclusion one level up: a cached component that merely calls one
// writing the element is the same mistake.
func TestPerRequestExclusionFollowsTheCallGraph(t *testing.T) {
	source := "package pages\n\ncomponent Field(): html {\n<span><app-nonce /></span>\n}\n" +
		"@cache(ttl: \"1m\")\ncomponent Panel(): html {\n<div><Field /></div>\n}\n" +
		"export component Page(): html {\n<main><Panel /></main>\n}\n"
	message := generateError(t, source, htmlbind.GenerateOptions{
		BuiltinElements: []htmlbind.BuiltinElement{appNonce()},
	})
	if !strings.Contains(message, "cached component Panel") || !strings.Contains(message, "Field writes one") {
		t.Fatalf("diagnostic = %q, want it to name both components", message)
	}
}

// The typo case is the reason the space is closed. An unrecognized hyphenated
// element emitted unchanged renders nothing and reports nothing.
func TestUndeclaredHyphenatedElementIsRefused(t *testing.T) {
	source := "package pages\n\nexport component Page(): html {\n<form><app-noncc /></form>\n}\n"
	message := generateError(t, source, htmlbind.GenerateOptions{
		BuiltinElements: []htmlbind.BuiltinElement{appNonce()},
	})
	for _, want := range []string{"page.tb.html:4:", "undeclared element <app-noncc>", "did you mean <app-nonce>?"} {
		if !strings.Contains(message, want) {
			t.Fatalf("diagnostic = %q, want it to mention %q", message, want)
		}
	}
}

// Registering nothing closes the space and empties it, which is the one behavior
// change for an existing project. The diagnostic has to name the way out.
func TestZeroRegistrationRefusesEveryHyphenatedElement(t *testing.T) {
	source := "package pages\n\nexport component Page(): html {\n<div><sl-button>ok</sl-button></div>\n}\n"
	message := generateError(t, source, htmlbind.GenerateOptions{})
	for _, want := range []string{"undeclared element <sl-button>", "passthrough entry"} {
		if !strings.Contains(message, want) {
			t.Fatalf("diagnostic = %q, want it to mention %q", message, want)
		}
	}
}

// Without the passthrough kind a closed space would ban Web Components outright.
// A declared one is ordinary markup and produces no plan step.
func TestPassthroughElementIsEmittedVerbatim(t *testing.T) {
	source := "package pages\n\nexport component Page(label: string): html {\n" +
		"<div><sl-button variant=\"primary\">{label}</sl-button><my-widget /></div>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{
		PassthroughElements: []htmlbind.PassthroughElement{{Name: "sl-*"}, {Name: "my-widget"}},
	})
	if !strings.Contains(generated, `<sl-button variant=\"primary\">`) {
		t.Fatalf("a passthrough element must survive verbatim:\n%s", generated)
	}
	if !strings.Contains(generated, `<my-widget></my-widget>`) && !strings.Contains(generated, `<my-widget />`) {
		t.Fatalf("the exact-name passthrough is missing:\n%s", generated)
	}
	if strings.Contains(generated, "htmlbind.Provide") {
		t.Fatalf("a passthrough element must produce no plan step:\n%s", generated)
	}
}

// A hyphenated name inside SVG or MathML is a standard foreign-namespace element
// rather than a custom one, so the whitelist does not reach into it.
func TestForeignContentIsOutsideTheWhitelist(t *testing.T) {
	source := "package pages\n\nexport component Page(): html {\n" +
		"<svg><color-profile /></svg>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	if !strings.Contains(generated, "<color-profile") {
		t.Fatalf("foreign content must survive verbatim:\n%s", generated)
	}
}

// A head-only contribution written in the body becomes a generation error rather
// than a page that half works.
func TestHeadOnlyElementInTheBodyIsRefused(t *testing.T) {
	head := otelTracing()
	head.Placement = htmlbind.PlaceHead
	source := "package pages\n\nexport component Page(): html {\n<main><otel-tracing service-name=\"api\" /></main>\n}\n"
	message := generateError(t, source, htmlbind.GenerateOptions{
		BuiltinElements: []htmlbind.BuiltinElement{head},
	})
	if !strings.Contains(message, "head-only") || !strings.Contains(message, "otel-tracing") {
		t.Fatalf("diagnostic = %q", message)
	}
}

// An attribute expression is checked against the declared parameter type exactly
// as on an ordinary element.
func TestBuiltinElementAttributeDiagnostics(t *testing.T) {
	tests := []struct{ name, body, want string }{
		{"unknown attribute", `<otel-tracing service-name="api" region="eu" />`, "has no attribute region"},
		{"missing required", `<otel-tracing />`, "requires the attribute service-name"},
		{"wrong type", `<otel-tracing service-name={count} />`, "expects string, got int"},
		{"children", `<otel-tracing service-name="api">x</otel-tracing>`, "takes no children"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "package pages\n\nexport component Page(count: int): html {\n<main>" + test.body + "</main>\n}\n"
			message := generateError(t, source, htmlbind.GenerateOptions{
				BuiltinElements: []htmlbind.BuiltinElement{otelTracing()},
			})
			if !strings.Contains(message, test.want) {
				t.Fatalf("diagnostic = %q, want %q", message, test.want)
			}
		})
	}
}

// A registration mistake belongs to whoever wrote the generate command, so it is
// reported there rather than waiting for a template that happens to use it.
func TestRegistrationDiagnostics(t *testing.T) {
	provider := &htmlbind.ElementProvider{Name: "NonceFor", Result: "Nonce"}
	tests := []struct {
		name    string
		entries []htmlbind.BuiltinElement
		through []htmlbind.PassthroughElement
		want    string
	}{
		{"no hyphen", []htmlbind.BuiltinElement{{Name: "banner", Markup: "<b>x</b>"}}, nil, "interior hyphen"},
		{
			"duplicate", []htmlbind.BuiltinElement{appNonce(), appNonce()}, nil,
			"declared twice",
		},
		{
			"declared as both kinds", []htmlbind.BuiltinElement{appNonce()},
			[]htmlbind.PassthroughElement{{Name: "app-nonce"}}, "declared twice",
		},
		{
			"hole with no provider",
			[]htmlbind.BuiltinElement{{Name: "app-badge", Markup: `<b>{{.Count}}</b>`}}, nil,
			"needs a provider",
		},
		{
			"provider with no result",
			[]htmlbind.BuiltinElement{{Name: "app-badge", Markup: `<b>{{.Count}}</b>`,
				Provider: &htmlbind.ElementProvider{Name: "CountFor"}}}, nil,
			"Result type named",
		},
		{
			"provider no hole uses",
			[]htmlbind.BuiltinElement{{Name: "app-badge", Markup: `<b>x</b>`, Provider: provider}}, nil,
			"no hole uses",
		},
		{
			"hole in a tag name",
			[]htmlbind.BuiltinElement{{Name: "app-badge", Markup: `<{{.Tag}}>x</b>`, Provider: provider}}, nil,
			"tag name or an attribute name",
		},
		{
			"hole inside script",
			[]htmlbind.BuiltinElement{{Name: "app-badge", Markup: `<script>var t = {{.Token}};</script>`, Provider: provider}}, nil,
			"opaque shape",
		},
		{
			"opaque shape",
			[]htmlbind.BuiltinElement{{Name: "app-badge", Shape: "opaque", Markup: "<b>x</b>"}}, nil,
			"not implemented",
		},
		{
			"bad glob", nil, []htmlbind.PassthroughElement{{Name: "sl*"}}, "end at a hyphen",
		},
		{
			// Emission has to decide whether to qualify the result with the
			// provider's package. Anything but a name, a qualified name, or a
			// scalar makes that a guess, and a guess here is wrong quietly.
			"slice result",
			[]htmlbind.BuiltinElement{{Name: "app-badge", Markup: `<b>{{.Count}}</b>`,
				Provider: &htmlbind.ElementProvider{Name: "CountFor", Result: "[]Item"}}}, nil,
			"cannot carry named holes",
		},
		{
			"pointer result",
			[]htmlbind.BuiltinElement{{Name: "app-badge", Markup: `<b>{{.Count}}</b>`,
				Provider: &htmlbind.ElementProvider{Name: "CountFor", Result: "*Counts"}}}, nil,
			"cannot carry named holes",
		},
		{
			"map result",
			[]htmlbind.BuiltinElement{{Name: "app-badge", Markup: `<b>{{.Count}}</b>`,
				Provider: &htmlbind.ElementProvider{Name: "CountFor", Result: "map[string]string"}}}, nil,
			"cannot carry named holes",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "package pages\n\nexport component Page(): html {\n<main>ok</main>\n}\n"
			message := generateError(t, source, htmlbind.GenerateOptions{
				BuiltinElements:     test.entries,
				PassthroughElements: test.through,
			})
			if !strings.Contains(message, test.want) {
				t.Fatalf("diagnostic = %q, want %q", message, test.want)
			}
		})
	}
}

// A builtin element's assets join the required set of every component that
// writes it, which is what lets a document carry them before a later swap needs
// them.
func TestBuiltinElementAssetsJoinTheRequiredSet(t *testing.T) {
	widget := htmlbind.BuiltinElement{
		Name:   "app-widget",
		Markup: `<div class="widget"></div>`,
		Assets: []htmlbind.Asset{{
			Kind: htmlbind.AssetScript, Base: "widget.script.abc123", Extension: "js",
			URL: "/public/generated/widget.script.abc123.js",
		}},
	}
	source := "package pages\n\ncomponent Inner(): html {\n<span><app-widget /></span>\n}\n" +
		"export component Page(): html {\n<main><Inner /></main>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{
		BuiltinElements: []htmlbind.BuiltinElement{widget},
	})
	// Twice: on the component writing it and on the one that calls that.
	want := `{ID: "widget.script.abc123", Type: "text/javascript", URL: "/public/generated/widget.script.abc123.js"}`
	if got := strings.Count(generated, want); got != 2 {
		t.Fatalf("required asset appears %d times, want the writer and its caller:\n%s", got, generated)
	}
}

// A project registering nothing regenerates byte for byte, so neither field is
// written when there is nothing to say.
func TestNoBuiltinElementsLeavesOutputUnchanged(t *testing.T) {
	source := "package pages\n\nexport component Page(name: string): html {\n<p>{name}</p>\n}\n"
	plain := generateWith(t, source, htmlbind.GenerateOptions{})
	registered := generateWith(t, source, htmlbind.GenerateOptions{
		BuiltinElements:     []htmlbind.BuiltinElement{appNonce()},
		PassthroughElements: []htmlbind.PassthroughElement{{Name: "sl-*"}},
	})
	if plain != registered {
		t.Fatalf("registering an unused element changed the output:\n%s\n%s", plain, registered)
	}
	if strings.Contains(plain, "Vary:") {
		t.Fatalf("a component depending on nothing must declare no axis:\n%s", plain)
	}
}

// A hole and a parameter are matched on a folded spelling rather than by
// splitting words, because splitting has to guess where an acronym ends. Getting
// that wrong is worse than it looks: with a provider declared, a hole that fails
// to match a parameter is read as a provider field, so the value comes silently
// from the wrong place.
func TestHoleNamesFindTheirParameterAcrossSpellings(t *testing.T) {
	for _, test := range []struct{ param, hole string }{
		{"id", "{{.ID}}"},
		{"api-url", "{{.APIURL}}"},
		{"service-name", "{{.ServiceName}}"},
	} {
		t.Run(test.param, func(t *testing.T) {
			element := htmlbind.BuiltinElement{
				Name:     "app-thing",
				Params:   []htmlbind.ElementParam{{Name: test.param, Type: "string", Required: true}},
				Markup:   `<b data-x="` + test.hole + `">{{.Token}}</b>`,
				Provider: &htmlbind.ElementProvider{Name: "NonceFor", Result: "Tok"},
			}
			source := "package pages\n\nexport component Page(v: string): html {\n" +
				"<main><app-thing " + test.param + "={v} /></main>\n}\n"
			generated := generateWith(t, source, htmlbind.GenerateOptions{
				BuiltinElements: []htmlbind.BuiltinElement{element},
			})
			if !strings.Contains(generated, "return p.V") {
				t.Fatalf("the hole did not bind to the declared parameter:\n%s", generated)
			}
			if strings.Contains(generated, "v."+strings.TrimSuffix(strings.TrimPrefix(test.hole, "{{."), "}}")) {
				t.Fatalf("the hole silently read the provider instead:\n%s", generated)
			}
		})
	}
}

// A provider in another package qualifies its result with that package, and a
// result already qualified is used as written. Both are exercised because the
// two paths are the whole reason the input space had to be closed.
func TestProviderResultQualification(t *testing.T) {
	for _, test := range []struct{ name, result, want string }{
		{"bare name takes the provider package", "Nonce", "v fw.Nonce"},
		{"already qualified is verbatim", "other.Nonce", "v other.Nonce"},
		{"a scalar is never qualified", "string", "v string"},
	} {
		t.Run(test.name, func(t *testing.T) {
			element := appNonce()
			if test.result == "string" {
				element.Markup = `<meta name="app-nonce" content="{{.}}">`
			}
			element.Provider = &htmlbind.ElementProvider{
				Package: "example.com/fw", Name: "NonceFor", Result: test.result,
			}
			source := "package pages\n\nexport component Page(): html {\n<form><app-nonce /></form>\n}\n"
			generated := generateWith(t, source, htmlbind.GenerateOptions{
				BuiltinElements: []htmlbind.BuiltinElement{element},
			})
			if !strings.Contains(generated, test.want) {
				t.Fatalf("want %q in:\n%s", test.want, generated)
			}
			// The provider's package is imported only because an element using
			// one is actually written.
			if !strings.Contains(generated, `"example.com/fw"`) {
				t.Fatalf("the provider package was not imported:\n%s", generated)
			}
		})
	}
}

// Every occurrence is its own plan step — the markup lands in two places — but
// each names the same provider, which is what makes them share one value.
func TestEveryOccurrenceNamesTheSameProvider(t *testing.T) {
	source := "package pages\n\nexport component Page(): html {\n" +
		"<div><form><app-nonce /></form><form><app-nonce /></form></div>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{
		BuiltinElements: []htmlbind.BuiltinElement{appNonce()},
	})
	if got := strings.Count(generated, `htmlbind.Provide("app-nonce", "NonceFor"`); got != 2 {
		t.Fatalf("two occurrences produced %d steps:\n%s", got, generated)
	}
}

// The memo key is the provider, not the element, so two elements backed by one
// function cannot disagree. A token in a hidden input and the same token in a
// meta tag is exactly that shape.
func TestTwoElementsShareOneProviderKey(t *testing.T) {
	meta := htmlbind.BuiltinElement{
		Name:     "app-nonce-meta",
		Markup:   `<meta name="nonce-2" content="{{.Token}}">`,
		Provider: &htmlbind.ElementProvider{Package: "example.com/fw", Name: "NonceFor", Result: "Nonce"},
	}
	token := appNonce()
	token.Provider = &htmlbind.ElementProvider{Package: "example.com/fw", Name: "NonceFor", Result: "Nonce"}
	source := "package pages\n\nexport component Page(): html {\n" +
		"<div><app-nonce-meta /><form><app-nonce /></form></div>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{
		BuiltinElements: []htmlbind.BuiltinElement{token, meta},
	})
	if got := strings.Count(generated, `"example.com/fw.NonceFor"`); got != 2 {
		t.Fatalf("the two elements must name one provider key, got %d:\n%s", got, generated)
	}
}
