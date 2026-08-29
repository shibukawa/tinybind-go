package htmlbind

import (
	"net/url"
	"strings"
	"testing"
)

// TestSafeURLDecidesOnWhatTheBrowserReads covers the cases the scheme policy
// exists for, including the two that defeat the obvious implementations: a
// hand-built url.URL whose Scheme field is empty, and a scheme split by a tab
// the browser removes before parsing.
func TestSafeURLDecidesOnWhatTheBrowserReads(t *testing.T) {
	opts := newRenderOptions(nil)
	for _, testcase := range []struct {
		name  string
		value string
		want  string
	}{
		{"ordinary https", "https://example.com/a?b=1#c", "https://example.com/a?b=1#c"},
		{"relative path", "/images/logo.png", "/images/logo.png"},
		{"scheme relative", "//cdn.example.com/x.js", "//cdn.example.com/x.js"},
		{"bare fragment", "#section", "#section"},
		{"relative with a later colon", "./javascript:alert(1)", "./javascript:alert(1)"},
		{"mailto", "mailto:someone@example.com", "mailto:someone@example.com"},
		{"tel", "tel:+81-3-0000-0000", "tel:+81-3-0000-0000"},
		{"javascript", "javascript:alert(1)", BlockedURL},
		{"javascript uppercase", "JAVASCRIPT:alert(1)", BlockedURL},
		{"javascript mixed case", "JaVaScRiPt:alert(1)", BlockedURL},
		{"javascript split by a tab", "java\tscript:alert(1)", BlockedURL},
		{"javascript split by a newline", "java\nscript:alert(1)", BlockedURL},
		{"javascript behind leading space", "   javascript:alert(1)", BlockedURL},
		{"javascript behind a control character", "\x01javascript:alert(1)", BlockedURL},
		{"vbscript", "vbscript:msgbox(1)", BlockedURL},
		{"data text/html", "data:text/html,<script>alert(1)</script>", BlockedURL},
		{"data image/png", "data:image/png;base64,iVBORw0KGgo=", "data:image/png;base64,iVBORw0KGgo="},
		{"data image/svg+xml", "data:image/svg+xml,<svg onload='alert(1)'/>", BlockedURL},
		{"data with no header separator", "data:whatever", BlockedURL},
		{"unknown scheme", "wow:whatever", BlockedURL},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			if got := opts.safeURL(testcase.value); got != testcase.want {
				t.Fatalf("safeURL(%q) = %q, want %q", testcase.value, got, testcase.want)
			}
		})
	}
}

// TestSafeURLRejectsTheEmptySchemeOpaqueForm is the case a gate reading
// url.URL.Scheme would pass: the field is empty, so the value looks relative,
// and String() still renders an executable URL.
func TestSafeURLRejectsTheEmptySchemeOpaqueForm(t *testing.T) {
	hostile := url.URL{Opaque: "javascript:alert(1)"}
	if hostile.Scheme != "" {
		t.Fatalf("precondition changed: Scheme is %q, so this case no longer proves anything", hostile.Scheme)
	}
	rendered := hostile.String()
	if rendered != "javascript:alert(1)" {
		t.Fatalf("precondition changed: String() = %q", rendered)
	}
	if got := newRenderOptions(nil).safeURL(rendered); got != BlockedURL {
		t.Fatalf("safeURL(%q) = %q, want %q", rendered, got, BlockedURL)
	}
}

// TestSafeURLHandBuiltUppercaseScheme covers the other struct-built form: Parse
// folds a scheme to lower case, but a field assigned directly keeps its case.
func TestSafeURLHandBuiltUppercaseScheme(t *testing.T) {
	hostile := url.URL{Scheme: "JAVASCRIPT", Opaque: "alert(1)"}
	if got := newRenderOptions(nil).safeURL(hostile.String()); got != BlockedURL {
		t.Fatalf("safeURL(%q) = %q, want %q", hostile.String(), got, BlockedURL)
	}
}

func TestWithURLSchemesReplacesTheRoster(t *testing.T) {
	opts := newRenderOptions([]Option{WithURLSchemes("https", "ftp")})
	if got := opts.safeURL("ftp://files.example.com/x"); got != "ftp://files.example.com/x" {
		t.Fatalf("configured scheme was refused: %q", got)
	}
	if got := opts.safeURL("http://example.com/x"); got != BlockedURL {
		t.Fatalf("http survived a roster that omits it: %q", got)
	}
	if got := opts.safeURL("/relative"); got != "/relative" {
		t.Fatalf("a relative URL must never depend on the roster: %q", got)
	}
}

// TestWithURLSchemesEmptyIsNotUnset separates a caller who permits nothing from
// a caller who configured nothing, which is why the option carries a flag.
func TestWithURLSchemesEmptyIsNotUnset(t *testing.T) {
	opts := newRenderOptions([]Option{WithURLSchemes()})
	if got := opts.safeURL("https://example.com/"); got != BlockedURL {
		t.Fatalf("an empty roster still permitted https: %q", got)
	}
	if got := newRenderOptions(nil).safeURL("https://example.com/"); got != "https://example.com/" {
		t.Fatalf("the default roster refused https: %q", got)
	}
}

func TestWithDataURLMediaTypes(t *testing.T) {
	opts := newRenderOptions([]Option{WithDataURLMediaTypes("image/svg+xml")})
	if got := opts.safeURL("data:image/svg+xml,<svg/>"); got != "data:image/svg+xml,<svg/>" {
		t.Fatalf("configured media type was refused: %q", got)
	}
	if got := opts.safeURL("data:image/png;base64,iVBORw0KGgo="); got != BlockedURL {
		t.Fatalf("png survived a roster that omits it: %q", got)
	}
	none := newRenderOptions([]Option{WithDataURLMediaTypes()})
	if got := none.safeURL("data:image/png;base64,iVBORw0KGgo="); got != BlockedURL {
		t.Fatalf("an empty media type roster still permitted a data URL: %q", got)
	}
}

func TestSafeSrcsetKeepsTheGoodCandidates(t *testing.T) {
	opts := newRenderOptions(nil)
	got := opts.safeSrcsetURLs("/a.png 1x, javascript:alert(1) 2x, /b.png 3x")
	want := "/a.png 1x, /b.png 3x"
	if got != want {
		t.Fatalf("safeSrcsetURLs = %q, want %q", got, want)
	}
	if got := opts.safeSrcsetURLs("javascript:alert(1)"); got != "" {
		t.Fatalf("a srcset of one hostile candidate should empty, got %q", got)
	}
	if got := opts.safeSrcsetURLs("/only.png"); got != "/only.png" {
		t.Fatalf("a descriptorless candidate was mangled: %q", got)
	}
}

// A candidate's URL ends at whitespace, not at a comma, so a data URL — the one
// form whose payload holds commas — survives the walk it used to be torn by.
func TestSafeSrcsetKeepsDataURLsWhole(t *testing.T) {
	opts := newRenderOptions(nil)
	const inline = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg=="
	for _, tc := range []struct{ name, in, want string }{
		{"alone", inline, inline},
		{"with a descriptor", inline + " 1x", inline + " 1x"},
		{"beside another candidate", inline + " 1x, /b.png 2x", inline + " 1x, /b.png 2x"},
		{"after another candidate", "/a.png 1x, " + inline + " 2x", "/a.png 1x, " + inline + " 2x"},
	} {
		if got := opts.safeSrcsetURLs(tc.in); got != tc.want {
			t.Errorf("%s: safeSrcsetURLs = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A refused candidate leaves nothing behind. Splitting on commas used to drop
// only the header of a refused data URL and keep its payload, which is a
// relative URL the browser then fetches from the document's own origin.
func TestSafeSrcsetDropsARefusedDataURLWhole(t *testing.T) {
	opts := newRenderOptions(nil)
	const script = "data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg=="
	for _, tc := range []struct{ name, in, want string }{
		{"alone", script + " 1x", ""},
		{"before a good one", script + " 1x, /b.png 2x", "/b.png 2x"},
		{"after a good one", "/a.png 1x, " + script + " 2x", "/a.png 1x"},
	} {
		if got := opts.safeSrcsetURLs(tc.in); got != tc.want {
			t.Errorf("%s: safeSrcsetURLs = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The separator forms the grammar allows, none of which may turn one candidate
// into two or admit a scheme the policy refuses.
func TestSafeSrcsetSeparatorForms(t *testing.T) {
	opts := newRenderOptions(nil)
	for _, tc := range []struct{ name, in, want string }{
		{"empty", "", ""},
		{"separators only", " , , ", ""},
		// A comma inside the URL token belongs to the URL, which is the same
		// rule that keeps a data URL whole, stated on a relative one.
		{"comma inside the url", "/a.png,/b.png 2x", "/a.png,/b.png 2x"},
		{"trailing comma ends a descriptorless candidate", "/a.png, /b.png 2x", "/a.png, /b.png 2x"},
		{"several trailing commas", "/a.png,,, /b.png 2x", "/a.png, /b.png 2x"},
		{"repeated commas", "/a.png 1x,,/b.png 2x", "/a.png 1x, /b.png 2x"},
		{"leading separators", " ,/a.png 1x", "/a.png 1x"},
		{"tab and newline separate", "/a.png\t1x,\n/b.png\t2x", "/a.png 1x, /b.png 2x"},
		{"hostile candidate after a comma", "/a.png 1x,javascript:alert(1) 2x", "/a.png 1x"},
		{"hostile candidate with no descriptor", "/a.png 1x, javascript:alert(1)", "/a.png 1x"},
	} {
		if got := opts.safeSrcsetURLs(tc.in); got != tc.want {
			t.Errorf("%s: safeSrcsetURLs(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestSafeSpaceURLsKeepsTheGoodEntries(t *testing.T) {
	opts := newRenderOptions(nil)
	got := opts.safeSpaceURLs("https://a.example/p javascript:alert(1) https://b.example/p")
	want := "https://a.example/p https://b.example/p"
	if got != want {
		t.Fatalf("safeSpaceURLs = %q, want %q", got, want)
	}
}

// TestURLAttrOpAppliesThePolicy exercises the op rather than the helper, which
// is what proves the render option actually reaches the attribute: the value
// closure never sees the renderer, so the check has to happen in Exec.
func TestURLAttrOpAppliesThePolicy(t *testing.T) {
	var out strings.Builder
	r := &Renderer{w: &out, opts: newRenderOptions(nil)}
	op := Builder[string]{}.URLAttr("href", func(p string) (string, bool) { return p, true })
	if err := op.Exec(r, "javascript:alert(1)"); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != ` href="`+BlockedURL+`"` {
		t.Fatalf("URLAttr wrote %q", got)
	}

	out.Reset()
	if err := op.Exec(r, "https://example.com/?a=1&b=2"); err != nil {
		t.Fatal(err)
	}
	// The permitted value is still HTML-escaped, because it lands inside a
	// double-quoted attribute like any other value.
	if got := out.String(); got != ` href="https://example.com/?a=1&amp;b=2"` {
		t.Fatalf("URLAttr wrote %q", got)
	}
}

func TestURLAttrOpHonoursAConfiguredRoster(t *testing.T) {
	var out strings.Builder
	r := &Renderer{w: &out, opts: newRenderOptions([]Option{WithURLSchemes("https", "ftp")})}
	op := Builder[string]{}.URLAttr("href", func(p string) (string, bool) { return p, true })
	if err := op.Exec(r, "ftp://files.example.com/x"); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != ` href="ftp://files.example.com/x"` {
		t.Fatalf("a configured scheme was refused at the op: %q", got)
	}
}

func TestURLAttrOpOmitsAnAbsentValue(t *testing.T) {
	var out strings.Builder
	r := &Renderer{w: &out, opts: newRenderOptions(nil)}
	op := Builder[string]{}.URLAttr("href", func(string) (string, bool) { return "", false })
	if err := op.Exec(r, ""); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Fatalf("an absent value wrote %q", out.String())
	}
}

func TestURLListAttrOp(t *testing.T) {
	var out strings.Builder
	r := &Renderer{w: &out, opts: newRenderOptions(nil)}
	op := Builder[string]{}.URLListAttr("srcset", URLListSrcset, func(p string) (string, bool) { return p, true })
	if err := op.Exec(r, "/a.png 1x, javascript:alert(1) 2x"); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != ` srcset="/a.png 1x"` {
		t.Fatalf("URLListAttr wrote %q", got)
	}
}
