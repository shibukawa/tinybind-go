package htmlbind

import (
	"strings"
	"testing"
)

// TestURLPathSegmentCollapse covers the three shapes
// requirement:embedder-implicit-bindings names, including the one where the
// segment is the whole tail and the separator must survive.
func TestURLPathSegmentCollapse(t *testing.T) {
	cases := []struct {
		name     string
		prefix   string
		value    string
		collapse bool
		suffix   string
		want     string
	}{
		{"leading segment with a path after it", "/", "ja", true, "/about", "/ja/about"},
		{"leading segment empty, path after it", "/", "", true, "/about", "/about"},
		{"trailing slash form, present", "/", "ja", true, "/", "/ja/"},
		{"trailing slash form, empty", "/", "", true, "/", "/"},
		{"segment is the whole tail, present", "/", "ja", false, "", "/ja"},
		{"segment is the whole tail, empty", "/", "", false, "", "/"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := URLPathSegment(test.prefix, test.value, test.collapse) + test.suffix
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

// TestURLPathSegmentCannotComposeAPath is the security property. The value is
// embedder-supplied but usually request-derived, so it is not trusted content.
func TestURLPathSegmentCannotComposeAPath(t *testing.T) {
	hostile := []struct {
		value string
		want  string
	}{
		{"ja", "/ja"},
		{"../admin", "/..%2Fadmin"},
		{"..", "/%2E%2E"},
		{".", "/%2E"},
		{"a/b", "/a%2Fb"},
		{"//evil.example.com", "/%2F%2Fevil.example.com"},
		{"ja?x=1", "/ja%3Fx%3D1"},
		{"ja#frag", "/ja%23frag"},
		{"javascript:alert(1)", "/javascript%3Aalert%281%29"},
		{"a b", "/a%20b"},
		{"%2e%2e", "/%252e%252e"},
	}
	for _, test := range hostile {
		got := URLPathSegment("/", test.value, false)
		if got != test.want {
			t.Fatalf("URLPathSegment(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

// TestURLPathSegmentThroughTheAttributeOp is the integration the emitter relies
// on: the helper composes the value, and the URL attribute op applies the scheme
// policy and escaping to what it produced.
//
// It is written against the op rather than against the helper alone, because
// the emitter puts the helper inside the op's closure and a value that survives
// one but not the other would be a hole nothing else would catch.
func TestURLPathSegmentThroughTheAttributeOp(t *testing.T) {
	// The shape the emitter writes for `<a href="/{lang}/about">`.
	op := Builder[string]{}.URLAttr("href", func(lang string) (string, bool) {
		return URLPathSegment("/", lang, true) + "/about", true
	})
	cases := []struct {
		lang string
		want string
	}{
		{"ja", ` href="/ja/about"`},
		{"", ` href="/about"`},
		{"../admin", ` href="/..%2Fadmin/about"`},
		{"/evil.example.com", ` href="/%2Fevil.example.com/about"`},
	}
	for _, test := range cases {
		var out strings.Builder
		r := &Renderer{w: &out, opts: newRenderOptions(nil)}
		if err := op.Exec(r, test.lang); err != nil {
			t.Fatal(err)
		}
		if got := out.String(); got != test.want {
			t.Fatalf("lang %q wrote %q, want %q", test.lang, got, test.want)
		}
	}
}

// TestEmptySegmentNeverWritesAProtocolRelativeURL is the failure the collapse
// exists to prevent: a browser reads // as a different host, so an application
// whose default language carries no prefix would navigate off-site.
func TestEmptySegmentNeverWritesAProtocolRelativeURL(t *testing.T) {
	op := Builder[string]{}.URLAttr("href", func(lang string) (string, bool) {
		return URLPathSegment("/", lang, true) + "/about", true
	})
	var out strings.Builder
	r := &Renderer{w: &out, opts: newRenderOptions(nil)}
	if err := op.Exec(r, ""); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), `href="//`) {
		t.Fatalf("an empty segment produced a protocol-relative URL: %q", out.String())
	}
}
