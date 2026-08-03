package htmlbind_test

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// imageHook is the shape both driving cases take: a prefix match on a
// reference, an appended extension, and a produced file.
func imageHook(calls *int) htmlbind.ReferenceHook {
	return htmlbind.ReferenceHook{
		Name:      "image-format",
		Element:   "img",
		Attribute: "src",
		Match:     func(value string) bool { return strings.HasPrefix(value, "/public/") },
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			if calls != nil {
				*calls++
			}
			name := strings.TrimPrefix(request.Value, "/public/")
			return htmlbind.ReferenceResult{
				Value: request.Value + ".webp",
				Files: []htmlbind.ProducedFile{{
					Name: name + ".webp", MediaType: "image/webp", Content: []byte("webp bytes"),
				}},
				Read: []string{"public/" + name},
			}, nil
		},
	}
}

const hookSource = `package pages

export component Gallery(): html {
<img src="/public/a.png" alt="a">
<img src="/public/a.png" alt="again">
<img src="/public/b.jpg" alt="b">
<img src="https://cdn.example.com/c.png" alt="external">
}
`

// TestValueRewriteLeavesStructureIntact is the core acceptance case: every
// claimed reference is rewritten, an unclaimed one is untouched, and the
// element tree is exactly what the author wrote.
func TestValueRewriteLeavesStructureIntact(t *testing.T) {
	calls := 0
	result, err := htmlbind.GenerateModule("gallery.tb.html", []byte(hookSource), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{imageHook(&calls)},
	})
	if err != nil {
		t.Fatal(err)
	}
	generated := string(result.GoSource)
	for _, want := range []string{
		`<img src=\"/public/a.png.webp\" alt=\"a\">`,
		`<img src=\"/public/a.png.webp\" alt=\"again\">`,
		`<img src=\"/public/b.jpg.webp\" alt=\"b\">`,
		`<img src=\"https://cdn.example.com/c.png\" alt=\"external\">`,
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated markup lacks %s:\n%s", want, generated)
		}
	}
	if strings.Contains(generated, "<picture") {
		t.Fatalf("a value result must not change the element tree:\n%s", generated)
	}
	// Twenty elements naming one file convert once; the count is of distinct
	// values, not of occurrences.
	if calls != 2 {
		t.Fatalf("transform calls = %d, want 2 distinct values", calls)
	}
	if len(result.Produced) != 2 {
		t.Fatalf("produced %d files, want 2: %+v", len(result.Produced), result.Produced)
	}
	if result.Produced[0].Name != "a.png.webp" || result.Produced[1].Name != "b.jpg.webp" {
		t.Fatalf("produced files are not in a deterministic order: %+v", result.Produced)
	}
	if len(result.ReadSet) != 2 || result.ReadSet[0] != "public/a.png" {
		t.Fatalf("read set = %v, want both sources sorted", result.ReadSet)
	}
	var occurrences int
	for _, rewrite := range result.Rewrites {
		occurrences += rewrite.Occurrences
	}
	if occurrences != 3 {
		t.Fatalf("reported %d occurrences, want 3", occurrences)
	}
}

// TestNoHooksLeavesOutputByteIdentical is the constraint every accepted seam in
// this catalog carries: a project using none pays nothing.
func TestNoHooksLeavesOutputByteIdentical(t *testing.T) {
	plain, err := htmlbind.Generate("gallery.tb.html", []byte(hookSource), htmlbind.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	registered, err := htmlbind.Generate("gallery.tb.html", []byte(hookSource), htmlbind.GenerateOptions{
		// A hook whose element never appears must not change a byte either.
		ReferenceHooks: []htmlbind.ReferenceHook{{
			Name: "unused", Element: "video", Attribute: "poster",
			Transform: func(htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				t.Fatal("transform ran for an element the template never writes")
				return htmlbind.ReferenceResult{}, nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != string(registered) {
		t.Fatalf("registering an unused hook changed the output:\n--- plain ---\n%s--- registered ---\n%s", plain, registered)
	}
}

// TestSkipLeavesTheAttributeAndSaysWhy covers the case a transform declines,
// such as an encode larger than its source. Declining is neither an error nor a
// silent no-op.
func TestSkipLeavesTheAttributeAndSaysWhy(t *testing.T) {
	source := `package pages

export component Page(): html {
<img src="/public/logo.svg" alt="logo">
}
`
	result, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{{
			Name: "image-format", Element: "img", Attribute: "src",
			Transform: func(htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				return htmlbind.ReferenceResult{Skip: true, Reason: "already vector"}, nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.GoSource), `src=\"/public/logo.svg\" alt=\"logo\">`) {
		t.Fatalf("a skip must leave the attribute exactly as written:\n%s", result.GoSource)
	}
	if len(result.Rewrites) != 1 || !result.Rewrites[0].Skipped || result.Rewrites[0].Reason != "already vector" {
		t.Fatalf("skip was not reported with its reason: %+v", result.Rewrites)
	}
	if len(result.Produced) != 0 {
		t.Fatalf("a skip produced files: %+v", result.Produced)
	}
}

// TestExpressionValuedAttributeIsReported covers the half-optimized page: the
// hook cannot see a runtime value, and saying nothing about it is the failure
// this reporting exists to prevent.
func TestExpressionValuedAttributeIsReported(t *testing.T) {
	source := `package pages

export component Card(image: url): html {
<img src={image} alt="dynamic">
}
`
	result, err := htmlbind.GenerateModule("card.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{imageHook(nil)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.DynamicReferences) != 1 {
		t.Fatalf("dynamic references = %+v, want one", result.DynamicReferences)
	}
	if got := result.DynamicReferences[0]; got.Hook != "image-format" || got.Element != "img" || got.Attribute != "src" {
		t.Fatalf("dynamic reference = %+v, want the registered pair", got)
	}
	if got := result.DynamicReferences[0].Pos; got.Line == 0 || got.Col == 0 {
		t.Fatalf("dynamic reference carries no position: %+v", got)
	}
	if len(result.Rewrites) != 0 {
		t.Fatalf("an expression value was rewritten: %+v", result.Rewrites)
	}

	_, err = htmlbind.GenerateModule("card.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{imageHook(nil)}, StrictReferenceHooks: true,
	})
	if err == nil {
		t.Fatal("strict mode accepted an expression-valued reference")
	}
	if !strings.Contains(err.Error(), "card.tb.html:4:") {
		t.Fatalf("strict diagnostic lacks the template position: %v", err)
	}
}

// TestTwoHooksClaimingOneAttributeFail keeps output independent of the order a
// command happened to assemble its options, which `--check` compares bytes
// against.
func TestTwoHooksClaimingOneAttributeFail(t *testing.T) {
	source := `package pages

export component Page(): html {
<img src="/public/a.png" alt="a">
}
`
	hook := func(name string) htmlbind.ReferenceHook {
		return htmlbind.ReferenceHook{
			Name: name, Element: "img", Attribute: "src",
			Match: func(value string) bool { return strings.HasPrefix(value, "/public/") },
			Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				return htmlbind.ReferenceResult{Value: request.Value + "." + name}, nil
			},
		}
	}
	_, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{hook("first"), hook("second")},
	})
	if err == nil {
		t.Fatal("two hooks claiming one attribute were accepted")
	}
	// The message names both, sorted, so it does not depend on registration
	// order either.
	for _, want := range []string{"first", "second", "page.tb.html:4:"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("diagnostic lacks %q: %v", want, err)
		}
	}
}

// TestHooksMayShareAPairWhenTheirMatchesDoNot is the other half: two hooks on
// one pair are legitimate as long as no value is claimed twice.
func TestHooksMayShareAPairWhenTheirMatchesDoNot(t *testing.T) {
	source := `package pages

export component Page(): html {
<img src="/public/a.png" alt="a">
<img src="/static/b.png" alt="b">
}
`
	hook := func(name, prefix string) htmlbind.ReferenceHook {
		return htmlbind.ReferenceHook{
			Name: name, Element: "img", Attribute: "src",
			Match: func(value string) bool { return strings.HasPrefix(value, prefix) },
			Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				return htmlbind.ReferenceResult{Value: request.Value + "." + name}, nil
			},
		}
	}
	result, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{hook("public", "/public/"), hook("static", "/static/")},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`/public/a.png.public`, `/static/b.png.static`} {
		if !strings.Contains(string(result.GoSource), want) {
			t.Fatalf("generated markup lacks %s:\n%s", want, result.GoSource)
		}
	}
}

// TestHookReachesHeadDeclaration covers the TypeScript case: an entry point is
// ordinarily named from a head declaration, which asset extraction otherwise
// passes through untouched.
func TestHookReachesHeadDeclaration(t *testing.T) {
	source := `package pages

export component Page(): html {
<head>
<script src="/public/app.ts" type="module"></script>
</head>
<div>body</div>
}
`
	result, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{{
			Name: "script-compile", Element: "script", Attribute: "src",
			Match: func(value string) bool { return strings.HasSuffix(value, ".ts") },
			Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				// The naming rule is the opposite of the image case: replace,
				// not append. Two callers disagreeing is why this module has no
				// naming rule of its own.
				name := strings.TrimSuffix(request.Value, ".ts") + ".js"
				return htmlbind.ReferenceResult{
					Value: name,
					Files: []htmlbind.ProducedFile{
						{Name: "app.js", MediaType: "text/javascript", Content: []byte("export {}")},
						// A source map is produced and no attribute names it.
						{Name: "app.js.map", MediaType: "application/json", Content: []byte("{}")},
					},
					Read: []string{"public/app.ts", "public/lib/util.ts"},
				}, nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.GoSource), `<script src=\"/public/app.js\" type=\"module\"></script>`) {
		t.Fatalf("head declaration was not rewritten:\n%s", result.GoSource)
	}
	if len(result.Produced) != 2 {
		t.Fatalf("produced %d files, want the script and its map: %+v", len(result.Produced), result.Produced)
	}
	// The read set is wider than the reference, because a converter following
	// imports reads files no template names.
	if len(result.ReadSet) != 2 || result.ReadSet[1] != "public/lib/util.ts" {
		t.Fatalf("read set = %v, want the imported module too", result.ReadSet)
	}
}

// TestForeignContentIsOutOfScope keeps a standard SVG name out of the seam.
func TestForeignContentIsOutOfScope(t *testing.T) {
	source := `package pages

export component Icon(): html {
<svg width="10" height="10"><image href="/public/a.png"></image></svg>
}
`
	result, err := htmlbind.GenerateModule("icon.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{{
			Name: "svg-image", Element: "image", Attribute: "href",
			Transform: func(htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				t.Fatal("a hook reached inside an SVG subtree")
				return htmlbind.ReferenceResult{}, nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.GoSource), `/public/a.png`) {
		t.Fatalf("foreign content was altered:\n%s", result.GoSource)
	}
}

// TestTransformValueCannotEscapeItsAttribute is the escaping guarantee: a
// transform returning hostile bytes cannot break out of the attribute it
// rewrites.
func TestTransformValueCannotEscapeItsAttribute(t *testing.T) {
	source := `package pages

export component Page(): html {
<img src="/public/a.png" alt="a">
}
`
	result, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{{
			Name: "hostile", Element: "img", Attribute: "src",
			Transform: func(htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				return htmlbind.ReferenceResult{Value: `x" onerror="alert(1)`}, nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	generated := string(result.GoSource)
	if strings.Contains(generated, `onerror=\"alert(1)\"`) {
		t.Fatalf("a transform value escaped its attribute:\n%s", generated)
	}
	if !strings.Contains(generated, `&#34;`) {
		t.Fatalf("the quote was not escaped:\n%s", generated)
	}
}

// TestTransformErrorCarriesTheTemplatePosition keeps a diagnostic pointing at
// the template the author wrote, not at generated markup.
func TestTransformErrorCarriesTheTemplatePosition(t *testing.T) {
	source := `package pages

export component Page(): html {
<img src="/public/missing.png" alt="a">
}
`
	_, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{{
			Name: "image-format", Element: "img", Attribute: "src",
			Transform: func(htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				return htmlbind.ReferenceResult{}, errors.New("no such file public/missing.png")
			},
		}},
	})
	if err == nil {
		t.Fatal("a transform error was swallowed")
	}
	for _, want := range []string{"page.tb.html:4:", "image-format", "no such file"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("diagnostic lacks %q: %v", want, err)
		}
	}
}

// TestProducedFileCannotEscapeTheOutputRoot guards the caller, which writes
// these files unexamined.
func TestProducedFileCannotEscapeTheOutputRoot(t *testing.T) {
	source := `package pages

export component Page(): html {
<img src="/public/a.png" alt="a">
}
`
	for _, name := range []string{"../outside.webp", "/etc/passwd", `dir\file.webp`, ""} {
		_, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
			ReferenceHooks: []htmlbind.ReferenceHook{{
				Name: "image-format", Element: "img", Attribute: "src",
				Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
					return htmlbind.ReferenceResult{
						Value: request.Value + ".webp",
						Files: []htmlbind.ProducedFile{{Name: name, Content: []byte("x")}},
					}, nil
				},
			}},
		})
		if err == nil {
			t.Fatalf("produced file name %q was accepted", name)
		}
	}
}

// TestTwoSourcesOneOutputNameFail: this module owns the produced file list, so
// it is the one place a collision can be seen at all.
func TestTwoSourcesOneOutputNameFail(t *testing.T) {
	source := `package pages

export component Page(): html {
<img src="/public/a.png" alt="a">
<img src="/public/b.png" alt="b">
}
`
	_, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{{
			Name: "image-format", Element: "img", Attribute: "src",
			Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				return htmlbind.ReferenceResult{
					Value: request.Value + ".webp",
					Files: []htmlbind.ProducedFile{{Name: "out.webp", Content: []byte(request.Value)}},
				}, nil
			},
		}},
	})
	if err == nil {
		t.Fatal("two different files claiming one name were accepted")
	}
	if !strings.Contains(err.Error(), "out.webp") {
		t.Fatalf("diagnostic does not name the file: %v", err)
	}
}

func TestValidateReferenceHooks(t *testing.T) {
	transform := func(htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
		return htmlbind.ReferenceResult{}, nil
	}
	cases := []struct {
		name  string
		hooks []htmlbind.ReferenceHook
		want  string
	}{
		{"no name", []htmlbind.ReferenceHook{{Element: "img", Attribute: "src", Transform: transform}}, "no name"},
		{"duplicate name", []htmlbind.ReferenceHook{
			{Name: "a", Element: "img", Attribute: "src", Transform: transform},
			{Name: "a", Element: "video", Attribute: "poster", Transform: transform},
		}, "duplicate"},
		{"no element", []htmlbind.ReferenceHook{{Name: "a", Attribute: "src", Transform: transform}}, "no element"},
		{"no attribute", []htmlbind.ReferenceHook{{Name: "a", Element: "img", Transform: transform}}, "no attribute"},
		{"no transform", []htmlbind.ReferenceHook{{Name: "a", Element: "img", Attribute: "src"}}, "no transform"},
		{"uppercase element", []htmlbind.ReferenceHook{{Name: "a", Element: "Img", Attribute: "src", Transform: transform}}, "invalid element"},
		// The hyphenated space belongs to the builtin element whitelist, and a
		// hook silently claiming a name there would make two mechanisms fight
		// over one element.
		{"hyphenated element", []htmlbind.ReferenceHook{{Name: "a", Element: "my-image", Attribute: "src", Transform: transform}}, "hyphenated"},
		{"invalid attribute", []htmlbind.ReferenceHook{{Name: "a", Element: "img", Attribute: "sr c", Transform: transform}}, "invalid attribute"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := htmlbind.ValidateReferenceHooks(test.hooks)
			if err == nil {
				t.Fatalf("%s was accepted", test.name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error %q lacks %q", err, test.want)
			}
		})
	}
	if err := htmlbind.ValidateReferenceHooks([]htmlbind.ReferenceHook{
		{Name: "a", Element: "img", Attribute: "src", Transform: transform},
		{Name: "b", Element: "img", Attribute: "src", Transform: transform},
	}); err != nil {
		t.Fatalf("two hooks on one pair were rejected at registration: %v", err)
	}
}

// TestRewritingIsDeterministic guards the property `--check` depends on.
func TestRewritingIsDeterministic(t *testing.T) {
	first, err := htmlbind.Generate("gallery.tb.html", []byte(hookSource), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{imageHook(nil)},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		again, err := htmlbind.Generate("gallery.tb.html", []byte(hookSource), htmlbind.GenerateOptions{
			ReferenceHooks: []htmlbind.ReferenceHook{imageHook(nil)},
		})
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(again) {
			t.Fatalf("rewriting is not deterministic:\n--- first ---\n%s--- again ---\n%s", first, again)
		}
	}
}

// stylesheetHook is the CSS-module case: a TypeScript entry point whose build
// emits a companion stylesheet that no attribute in the template can name.
func stylesheetHook(entry, script, stylesheet string) htmlbind.ReferenceHook {
	return htmlbind.ReferenceHook{
		Name: "ts-build", Element: "script", Attribute: "src",
		Match: func(value string) bool { return strings.HasSuffix(value, ".ts") },
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			if request.Value != entry {
				return htmlbind.ReferenceResult{Skip: true, Reason: "not the entry point"}, nil
			}
			return htmlbind.ReferenceResult{
				Value: script,
				Head: []htmlbind.HeadEntry{{
					Element:    "link",
					Attributes: map[string]string{"rel": "stylesheet", "href": stylesheet},
				}},
			}, nil
		},
	}
}

// TestHookContributesHead covers the case the whole feature exists for: the
// build produces a stylesheet, and without a contribution no page loads it.
func TestHookContributesHead(t *testing.T) {
	source := `package pages

export component Page(): html {
<head>
<script src="/public/app.ts" type="module"></script>
</head>
<div>body</div>
}
`
	result, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{
			stylesheetHook("/public/app.ts", "/public/app.js", "/public/app.css"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	generated := string(result.GoSource)
	if !strings.Contains(generated, `<link href=\"/public/app.css\" rel=\"stylesheet\">`) {
		t.Fatalf("the produced stylesheet was not linked:\n%s", generated)
	}
	// The author's own entry comes first: a contribution is a consequence of a
	// rewrite, not something placed in the template.
	script := strings.Index(generated, `/public/app.js`)
	link := strings.Index(generated, `/public/app.css`)
	if script < 0 || link < 0 || script > link {
		t.Fatalf("a contribution did not land after the authored head entry:\n%s", generated)
	}
}

// TestHookContributesHeadWithoutADeclaration covers a component that writes no
// head of its own, which is the ordinary case for one that names its entry
// point from the document shell.
func TestHookContributesHeadWithoutADeclaration(t *testing.T) {
	source := `package pages

export component Widget(): html {
<div><script src="/public/app.ts" type="module"></script></div>
}
`
	result, err := htmlbind.GenerateModule("widget.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{
			stylesheetHook("/public/app.ts", "/public/app.js", "/public/app.css"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	generated := string(result.GoSource)
	if !strings.Contains(generated, `<link href=\"/public/app.css\" rel=\"stylesheet\">`) {
		t.Fatalf("a component with no head declaration got no contribution:\n%s", generated)
	}
	// The synthesized head is hoisted, so nothing of it is emitted in the body.
	if strings.Contains(generated, `<div><link`) {
		t.Fatalf("a contribution was emitted in place rather than hoisted:\n%s", generated)
	}
}

// TestHookHeadContributionIsDeduplicated covers two entry points importing one
// CSS module, which is one link and not two.
func TestHookHeadContributionIsDeduplicated(t *testing.T) {
	source := `package pages

export component Page(): html {
<div>
<script src="/public/a.ts" type="module"></script>
<script src="/public/b.ts" type="module"></script>
<script src="/public/a.ts" type="module"></script>
</div>
}
`
	shared := htmlbind.ReferenceHook{
		Name: "ts-build", Element: "script", Attribute: "src",
		Match: func(value string) bool { return strings.HasSuffix(value, ".ts") },
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			return htmlbind.ReferenceResult{
				Value: strings.TrimSuffix(request.Value, ".ts") + ".js",
				Head: []htmlbind.HeadEntry{{
					Element:    "link",
					Attributes: map[string]string{"rel": "stylesheet", "href": "/public/shared.css"},
				}},
			}, nil
		},
	}
	result, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{shared},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(result.GoSource), "/public/shared.css"); count != 1 {
		t.Fatalf("one stylesheet produced %d links, want 1:\n%s", count, result.GoSource)
	}
}

// TestHookHeadContributionsDisagree covers two conversions naming one file with
// different attributes, which cannot both be right and must not be settled by
// whichever template was walked first.
func TestHookHeadContributionsDisagree(t *testing.T) {
	source := `package pages

export component Page(): html {
<div>
<script src="/public/a.ts" type="module"></script>
<script src="/public/b.ts" type="module"></script>
</div>
}
`
	disagreeing := htmlbind.ReferenceHook{
		Name: "ts-build", Element: "script", Attribute: "src",
		Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			media := "all"
			if strings.Contains(request.Value, "b.ts") {
				media = "print"
			}
			return htmlbind.ReferenceResult{
				Value: strings.TrimSuffix(request.Value, ".ts") + ".js",
				Head: []htmlbind.HeadEntry{{
					Element:    "link",
					Attributes: map[string]string{"rel": "stylesheet", "href": "/public/shared.css", "media": media},
				}},
			}, nil
		},
	}
	_, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{disagreeing},
	})
	if err == nil {
		t.Fatal("two contributions disagreeing about one file were accepted")
	}
	if !strings.Contains(err.Error(), "shared.css") {
		t.Fatalf("the conflict does not name the file: %v", err)
	}
}

// TestHookSkipContributesNoHead covers a declined conversion, which produced no
// file and so has nothing to load.
func TestHookSkipContributesNoHead(t *testing.T) {
	source := `package pages

export component Page(): html {
<div><script src="/public/vendor.ts" type="module"></script></div>
}
`
	result, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{
			stylesheetHook("/public/app.ts", "/public/app.js", "/public/app.css"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result.GoSource), "app.css") {
		t.Fatalf("a skip contributed a head entry:\n%s", result.GoSource)
	}
}

// TestHookHeadContributionIsRestricted covers a hook reaching past loading what
// it produced and into rewriting the document.
func TestHookHeadContributionIsRestricted(t *testing.T) {
	source := `package pages

export component Page(): html {
<div><script src="/public/app.ts" type="module"></script></div>
}
`
	for _, element := range []string{"title", "meta", "base"} {
		_, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
			ReferenceHooks: []htmlbind.ReferenceHook{{
				Name: "ts-build", Element: "script", Attribute: "src",
				Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
					return htmlbind.ReferenceResult{
						Value: "/public/app.js",
						Head:  []htmlbind.HeadEntry{{Element: element, Attributes: map[string]string{"content": "x"}}},
					}, nil
				},
			}},
		})
		if err == nil {
			t.Fatalf("a hook contributed a %s to the head", element)
		}
		if !strings.Contains(err.Error(), element) {
			t.Fatalf("the diagnostic does not name %s: %v", element, err)
		}
	}
}

// TestHookHeadContributionEscapes covers a transform handing over a value that
// would break out of the attribute it lands in.
func TestHookHeadContributionEscapes(t *testing.T) {
	source := `package pages

export component Page(): html {
<div><script src="/public/app.ts" type="module"></script></div>
}
`
	result, err := htmlbind.GenerateModule("page.tb.html", []byte(source), htmlbind.GenerateOptions{
		ReferenceHooks: []htmlbind.ReferenceHook{{
			Name: "ts-build", Element: "script", Attribute: "src",
			Transform: func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
				return htmlbind.ReferenceResult{
					Value: "/public/app.js",
					Head: []htmlbind.HeadEntry{{
						Element:    "link",
						Attributes: map[string]string{"rel": "stylesheet", "href": `x" onload="alert(1)`},
					}},
				}, nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result.GoSource), `onload=\"alert`) {
		t.Fatalf("a contributed value escaped its attribute:\n%s", result.GoSource)
	}
}

// TestCollectReferencesCallsNoTransform covers the discovery pass: it must find
// exactly what the rewrite would claim, and convert none of it.
func TestCollectReferencesCallsNoTransform(t *testing.T) {
	source := []byte(`package pages

export component Page(): html {
<head>
<script src="/public/app.ts" type="module"></script>
</head>
<div>
<img src="/public/a.png">
<img src="/public/a.png">
<img src="/public/b.png">
<img src={item.Image}>
<svg><image src="/public/c.png"/></svg>
</div>
}
`)
	calls := 0
	hooks := []htmlbind.ReferenceHook{
		imageHook(&calls),
		stylesheetHook("/public/app.ts", "/public/app.js", "/public/app.css"),
	}
	claims, err := htmlbind.CollectReferences("page.tb.html", source, hooks)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("discovery called %d transforms, want none", calls)
	}
	var found []string
	for _, claim := range claims {
		found = append(found, claim.Hook+" "+claim.Value)
	}
	want := []string{"image-format /public/a.png", "image-format /public/b.png", "ts-build /public/app.ts"}
	got := append([]string(nil), found...)
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("discovery found %v, want %v", got, want)
	}
}
