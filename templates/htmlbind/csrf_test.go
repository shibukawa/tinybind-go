package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// An author writes no token markup and every unsafe form carries one. That is
// the whole point: a security control an author has to remember to write is one
// an author will forget, and the omission renders a working page that fails only
// on submission.
func TestUnsafeFormCarriesTheTokenWithNoAuthoring(t *testing.T) {
	source := "package pages\n\nexport component Page(): html {\n" +
		"<form method=\"post\" action=\"/send\"><input name=\"body\"><button>Send</button></form>\n}\n" +
		"export component Pair(): html {\n<div>" +
		"<form method=\"post\" action=\"/a\"><input name=\"a\"></form>" +
		"<form method=\"post\" action=\"/b\"><input name=\"b\"></form></div>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	if !strings.Contains(generated, `CSRFField("_csrf")`) {
		t.Fatalf("an unsafe form must carry the field:\n%s", generated)
	}
	// First child, so a later field of the same name cannot displace it.
	field := strings.Index(generated, `CSRFField("_csrf")`)
	body := strings.Index(generated, `name=\"body\"`)
	if field < 0 || body < 0 || field > body {
		t.Fatalf("the field must come before the form's own inputs:\n%s", generated)
	}

	runGeneratedTests(t, []byte(generated), []byte(`package pages

import (
	"errors"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

func TestTokenIsWrittenAndEscaped(t *testing.T) {
	var out strings.Builder
	err := htmlbind.RenderChain(&out, nil, Page(PageParams{}),
		htmlbind.WithCSRFToken(`+"`"+`tok" onload="x`+"`"+`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `+"`"+`name="_csrf" value="tok&#34; onload=&#34;x"`+"`"+`) {
		t.Fatalf("token not written or not escaped: %s", out.String())
	}
}

// A forgotten option must not render a form that submits and is rejected with
// nothing pointing at the cause.
func TestMissingTokenFailsTheRender(t *testing.T) {
	var out strings.Builder
	err := htmlbind.RenderChain(&out, nil, Page(PageParams{}))
	if !errors.Is(err, htmlbind.ErrNoCSRFToken) {
		t.Fatalf("want ErrNoCSRFToken, got %v", err)
	}
}

// A render that is not a response — a mail body, a static export, a golden test
// — says so explicitly rather than being mistaken for a forgotten option.
func TestExplicitlyNoSessionRenders(t *testing.T) {
	var out strings.Builder
	if err := htmlbind.RenderChain(&out, nil, Page(PageParams{}), htmlbind.WithoutCSRFToken()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `+"`"+`value=""`+"`"+`) {
		t.Fatalf("want an empty token: %s", out.String())
	}
}

// Two forms on a page carry the same token, which is what a response header
// carrying one value requires.
func TestEveryFormCarriesTheSameToken(t *testing.T) {
	var out strings.Builder
	if err := htmlbind.RenderChain(&out, nil, Pair(PairParams{}), htmlbind.WithCSRFToken("tok")); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(out.String(), `+"`"+`value="tok"`+"`"+`); got != 2 {
		t.Fatalf("want both forms to carry it, got %d: %s", got, out.String())
	}
}
`))
}

// A GET form's fields become the query string, and a token in a URL reaches
// history, logs, and referrers.
func TestGetFormCarriesNoToken(t *testing.T) {
	source := "package pages\n\nexport component Page(): html {\n" +
		"<form method=\"get\" action=\"/search\"><input name=\"q\"></form>\n}\n"
	if generated := generateWith(t, source, htmlbind.GenerateOptions{}); strings.Contains(generated, "CSRFField") {
		t.Fatalf("a GET form must carry no token:\n%s", generated)
	}
	// A form with no method at all is a GET form.
	source = "package pages\n\nexport component Page(): html {\n<form action=\"/search\"><input name=\"q\"></form>\n}\n"
	if generated := generateWith(t, source, htmlbind.GenerateOptions{}); strings.Contains(generated, "CSRFField") {
		t.Fatalf("a form with no method is a GET form:\n%s", generated)
	}
}

// Inserting the token into a form that posts elsewhere would hand the session's
// secret to a third party.
func TestCrossOriginActionIsRefused(t *testing.T) {
	for _, action := range []string{"https://other.example.com/collect", "//other.example.com/collect"} {
		source := "package pages\n\nexport component Page(): html {\n" +
			"<form method=\"post\" action=\"" + action + "\"><input name=\"q\"></form>\n}\n"
		message := generateError(t, source, htmlbind.GenerateOptions{})
		if !strings.Contains(message, "another origin") {
			t.Fatalf("action %q: diagnostic = %q", action, message)
		}
	}
}

// The escape for a form that genuinely posts off-origin, and the marker never
// reaches the browser because it means nothing there.
func TestOptOutMarkerSuppressesTheFieldAndDoesNotTravel(t *testing.T) {
	source := "package pages\n\nexport component Page(): html {\n" +
		"<form method=\"post\" action=\"https://other.example.com/collect\" data-tb-no-csrf><input name=\"q\"></form>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	if strings.Contains(generated, "CSRFField") {
		t.Fatalf("an opted-out form must carry no token:\n%s", generated)
	}
	if strings.Contains(generated, "no-csrf") {
		t.Fatalf("the marker is read at generation time and must not travel:\n%s", generated)
	}
}

// A method this module cannot read at generation time cannot be decided, and
// guessing either leaks the token into a query string or leaves a form open.
func TestDynamicMethodIsRefused(t *testing.T) {
	source := "package pages\n\nexport component Page(method: string): html {\n" +
		"<form method={method} action=\"/send\"><input name=\"q\"></form>\n}\n"
	if message := generateError(t, source, htmlbind.GenerateOptions{}); !strings.Contains(message, "must be static") {
		t.Fatalf("diagnostic = %q", message)
	}
}

// A hand-written token still works and must not be doubled.
func TestExistingFieldIsNotDoubled(t *testing.T) {
	source := "package pages\n\nexport component Page(token: string): html {\n" +
		"<form method=\"post\" action=\"/send\"><input type=\"hidden\" name=\"_csrf\" value={token}></form>\n}\n"
	if generated := generateWith(t, source, htmlbind.GenerateOptions{}); strings.Contains(generated, "CSRFField") {
		t.Fatalf("a form already carrying the field must be left alone:\n%s", generated)
	}
}

// A stored body would serve one session's token to whoever asked next, which is
// a security failure rather than a staleness bug.
func TestCachedComponentWithAFormIsRefused(t *testing.T) {
	source := "package pages\n\n@cache(ttl: \"1m\")\ncomponent Panel(): html {\n" +
		"<form method=\"post\" action=\"/send\"><input name=\"q\"></form>\n}\n" +
		"export component Page(): html {\n<main><Panel /></main>\n}\n"
	message := generateError(t, source, htmlbind.GenerateOptions{})
	for _, want := range []string{"cached component Panel", "<form>", "one request's value to the next"} {
		if !strings.Contains(message, want) {
			t.Fatalf("diagnostic = %q, want it to mention %q", message, want)
		}
	}
}

// Splitting the cached list from the uncached form is the composition the
// exclusion pushes a project toward, and it has to work.
func TestACachedListBesideAnUncachedFormIsFine(t *testing.T) {
	source := "package pages\n\n@cache(ttl: \"1m\")\ncomponent List(rows: string[]): html {\n" +
		"<ul>{for row in rows}<li>{row}</li>{/for}</ul>\n}\n" +
		"component Form(): html {\n<form method=\"post\" action=\"/send\"><input name=\"q\"></form>\n}\n" +
		"export component Page(rows: string[]): html {\n<main><List rows={rows} /><Form /></main>\n}\n"
	if generated := generateWith(t, source, htmlbind.GenerateOptions{}); !strings.Contains(generated, "CSRFField") {
		t.Fatalf("the uncached form still needs its token:\n%s", generated)
	}
}

// A deployment that settled on origin checks alone turns the token off and gets
// its cacheable form-bearing components back.
func TestCSRFOffEmitsNothingAndRestoresCaching(t *testing.T) {
	source := "package pages\n\n@cache(ttl: \"1m\")\ncomponent Panel(): html {\n" +
		"<form method=\"post\" action=\"/send\"><input name=\"q\"></form>\n}\n" +
		"export component Page(): html {\n<main><Panel /></main>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{CSRFMode: htmlbind.CSRFOff})
	if strings.Contains(generated, "CSRFField") {
		t.Fatalf("the token is off:\n%s", generated)
	}
	// Off also lifts the per-request marking, which is the point of the switch.
	if !strings.Contains(generated, "Cache:") {
		t.Fatalf("a form-bearing component must be cacheable again:\n%s", generated)
	}
}

// The field name has to agree with whatever middleware reads the token back out.
func TestFieldNameIsConfigurable(t *testing.T) {
	source := "package pages\n\nexport component Page(): html {\n" +
		"<form method=\"post\" action=\"/send\"><input name=\"q\"></form>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{CSRFFieldName: "authenticity_token"})
	if !strings.Contains(generated, `CSRFField("authenticity_token")`) {
		t.Fatalf("the configured name never reached the plan:\n%s", generated)
	}
}

// A project with no unsafe form regenerates byte-identical Go.
func TestAProjectWithNoUnsafeFormIsUnchanged(t *testing.T) {
	source := "package pages\n\nexport component Page(name: string): html {\n<p>{name}</p>\n}\n"
	auto := generateWith(t, source, htmlbind.GenerateOptions{})
	off := generateWith(t, source, htmlbind.GenerateOptions{CSRFMode: htmlbind.CSRFOff})
	if auto != off {
		t.Fatalf("a template with no form must not notice the mode:\n%s\n%s", auto, off)
	}
	if strings.Contains(auto, "CSRF") {
		t.Fatalf("nothing about CSRF belongs in this output:\n%s", auto)
	}
}
