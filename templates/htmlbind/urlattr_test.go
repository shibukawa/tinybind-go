package htmlbind_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

func generateSource(t *testing.T, source string) string {
	t.Helper()
	generated, err := htmlbind.Generate("urlattr.txt", []byte(source), htmlbind.GenerateOptions{Package: "pages"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return string(generated)
}

// TestURLAttributesRouteThroughThePolicyOp pins that a URL-bearing attribute
// emits URLAttr rather than Attr, and that the value reaches it unescaped.
//
// The unescaped part is the load-bearing half: the op has to read the scheme
// before the ampersands are encoded, so an Escape call in the closure would put
// the check on the wrong side of the encoding.
func TestURLAttributesRouteThroughThePolicyOp(t *testing.T) {
	for _, testcase := range []struct{ name, markup, attr string }{
		{"href", `<a href={link}>x</a>`, "href"},
		{"src", `<img src={link}>`, "src"},
		{"object data", `<object data={link}></object>`, "data"},
		{"cite", `<blockquote cite={link}>x</blockquote>`, "cite"},
		{"xlink href", `<svg><a xlink:href={link}>x</a></svg>`, "xlink:href"},
		{"poster", `<video poster={link}></video>`, "poster"},
		{"formaction", `<button formaction={link}>x</button>`, "formaction"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			generated := generateSource(t, `component Page(link: url): html {`+testcase.markup+`}`)
			want := `URLAttr("` + testcase.attr + `"`
			if !strings.Contains(generated, want) {
				t.Fatalf("generated source does not call %s:\n%s", want, generated)
			}
			line := opLine(t, generated, want)
			if strings.Contains(line, "htmlbind.Escape") {
				t.Fatalf("%s escapes in the closure, so the policy would see an encoded value:\n%s", testcase.attr, line)
			}
		})
	}
}

// TestURLListAttributesCarryTheirGrammar covers the two list shapes, which are
// analyzed as text because neither is expressible as one url.URL, and still
// reach the policy one entry at a time.
func TestURLListAttributesCarryTheirGrammar(t *testing.T) {
	for _, testcase := range []struct{ name, markup, want string }{
		{"srcset", `<img srcset={candidates}>`, `URLListAttr("srcset", htmlbind.URLListSrcset`},
		{"imagesrcset", `<link imagesrcset={candidates}>`, `URLListAttr("imagesrcset", htmlbind.URLListSrcset`},
		{"ping", `<a ping={candidates}>x</a>`, `URLListAttr("ping", htmlbind.URLListSpace`},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			generated := generateSource(t, `component Page(candidates: string): html {`+testcase.markup+`}`)
			if !strings.Contains(generated, testcase.want) {
				t.Fatalf("generated source does not call %s:\n%s", testcase.want, generated)
			}
		})
	}
}

// TestOrdinaryAttributesAreUntouched is the other half of the scoping claim:
// widening the roster must not move an attribute that was never a URL.
func TestOrdinaryAttributesAreUntouched(t *testing.T) {
	generated := generateSource(t, `component Page(title: string): html {<p title={title} data-x={title}>x</p>}`)
	for _, attr := range []string{"title", "data-x"} {
		line := opLine(t, generated, `Attr("`+attr+`"`)
		if !strings.Contains(line, "htmlbind.Escape(p.Title)") {
			t.Fatalf("%s no longer escapes in the closure:\n%s", attr, line)
		}
	}
	if strings.Contains(generated, "URLAttr") {
		t.Fatalf("a non-URL attribute was routed through the URL policy:\n%s", generated)
	}
}

// opLine returns the one generated line containing marker, so an assertion can
// be made about that instruction rather than about the whole file.
func opLine(t *testing.T, generated, marker string) string {
	t.Helper()
	for _, line := range strings.Split(generated, "\n") {
		if strings.Contains(line, marker) {
			return line
		}
	}
	t.Fatalf("generated source contains no line with %q:\n%s", marker, generated)
	return ""
}

// TestEventAttributeAcceptsTrustedJavaScript is the escape hatch the diagnostic
// points at, and the reason the rule makes the type honest rather than banning
// the position outright.
func TestEventAttributeAcceptsTrustedJavaScript(t *testing.T) {
	generated := generateSource(t, `component Page(code: string): html {<button onclick={RawJavaScript(code)}>x</button>}`)
	if !strings.Contains(generated, `Attr("onclick"`) {
		t.Fatalf("a trusted_javascript handler did not emit an attribute:\n%s", generated)
	}
	// It is HTML-escaped, because an attribute value is HTML-decoded by the
	// parser before the handler body is compiled: escaping is what keeps the
	// value inside its quotes without changing the JavaScript the browser sees.
	if !strings.Contains(generated, "htmlbind.Escape") {
		t.Fatalf("a handler body must still be attribute-escaped:\n%s", generated)
	}
}

// TestStaticEventAttributeStillCompiles keeps the rule about insertion rather
// than about the attribute existing: authored markup with no expression in it
// is not what the gate is for.
func TestStaticEventAttributeStillCompiles(t *testing.T) {
	generated := generateSource(t, `component Page(): html {<button onclick="doThing()">x</button>}`)
	if !strings.Contains(generated, `onclick=\"doThing()\"`) {
		t.Fatalf("a static handler was not emitted verbatim:\n%s", generated)
	}
}

// TestHyphenatedOnNameIsNotAHandler pins the matching rule: on-click belongs to
// a custom element and is not an event handler content attribute.
func TestHyphenatedOnNameIsNotAHandler(t *testing.T) {
	generated := generateSource(t, `component Page(value: string): html {<p on-click={value}>x</p>}`)
	if !strings.Contains(generated, `Attr("on-click"`) {
		t.Fatalf("on-click was not emitted as an ordinary attribute:\n%s", generated)
	}
	if strings.Contains(generated, "URLAttr") {
		t.Fatalf("on-click reached the URL policy:\n%s", generated)
	}
}

// TestOnlyTheURLAttributeMovedInTheFixture guards the claim the decision makes
// about blast radius, using the one fixture that has a URL attribute in it.
func TestOnlyTheURLAttributeMovedInTheFixture(t *testing.T) {
	generated := []byte(generateSource(t, `type User { profile: url; name: string }
component Page(user: User): html {<a href={user.profile} title={user.name}>x</a>}`))
	if !bytes.Contains(generated, []byte(`URLAttr("href"`)) {
		t.Fatalf("href did not move to the policy op:\n%s", generated)
	}
	if !bytes.Contains(generated, []byte(`Attr("title"`)) || bytes.Contains(generated, []byte(`URLAttr("title"`)) {
		t.Fatalf("title moved when it should not have:\n%s", generated)
	}
}
