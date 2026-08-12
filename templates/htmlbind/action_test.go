package htmlbind

import (
	"strings"
	"testing"
)

func actionRefs(t *testing.T, source string) []ActionRef {
	t.Helper()
	got, err := ActionRefs("page.tb.html", []byte(source))
	if err != nil {
		t.Fatalf("ActionRefs: %v", err)
	}
	return got
}

func TestActionRefsReportsEveryReference(t *testing.T) {
	got := actionRefs(t, `
export component Page(id: string): html {
  <form server-action="Rename" method="post">
    <input name="name">
  </form>
  <button server-action="Delete" data-target="#row">x</button>
}
`)
	want := []ActionRef{
		{Component: "Page", Handler: "Rename", Element: "form"},
		{Component: "Page", Handler: "Delete", Element: "button"},
	}
	if len(got) != len(want) {
		t.Fatalf("refs = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		if got[i].Component != expected.Component || got[i].Handler != expected.Handler || got[i].Element != expected.Element {
			t.Errorf("ref %d = %+v, want %+v", i, got[i], expected)
		}
		if got[i].Pos.Line == 0 {
			t.Errorf("ref %d carries no position", i)
		}
	}
}

func TestActionRefsFindsReferencesInsideControlFlow(t *testing.T) {
	got := actionRefs(t, `
export component Page(ids: string[], ok: bool): html {
  {for id in ids}
    <button server-action="Delete">x</button>
  {/for}
  {if ok}
    <button server-action="Publish">go</button>
  {/if}
}
`)
	if len(got) != 2 {
		t.Fatalf("refs = %d, want 2: %+v", len(got), got)
	}
	if got[0].Handler != "Delete" || got[1].Handler != "Publish" {
		t.Errorf("handlers = %q, %q", got[0].Handler, got[1].Handler)
	}
}

func TestServerActionRejectsBadValues(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "computed value",
			source: `export component Page(n: string): html { <button server-action={n}>x</button> }`,
			want:   "must be a literal function name",
		},
		{
			name:   "unexported",
			source: `export component Page(): html { <button server-action="rename">x</button> }`,
			want:   "must be an exported Go function name",
		},
		{
			name:   "bare attribute",
			source: `export component Page(): html { <button server-action>x</button> }`,
			want:   "must name a Go function",
		},
		{
			name:   "reserved page entry point",
			source: `export component Page(): html { <button server-action="Load">x</button> }`,
			want:   "cannot name Load",
		},
		{
			name:   "form with its own action",
			source: `export component Page(): html { <form server-action="Rename" action="/x"></form> }`,
			want:   "cannot also carry action",
		},
		{
			name:   "form with a get method",
			source: `export component Page(): html { <form server-action="Rename" method="get"></form> }`,
			want:   `must use method="post"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ActionRefs("page.tb.html", []byte(tc.source))
			if err == nil {
				t.Fatalf("expected an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestGenerateLowersServerActionToTheURLAttribute(t *testing.T) {
	source := `export component Page(): html { <button server-action="Rename" data-target="#r">go</button> }`
	got, err := Generate("page.tb.html", []byte(source), GenerateOptions{
		Package:       "id_",
		ServerActions: map[string]string{"Rename": "/_action/9f3c2ab1e4d7/Rename"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, `data-tb-action=\"/_action/9f3c2ab1e4d7/Rename\"`) {
		t.Errorf("generated output does not carry the lowered attribute:\n%s", out)
	}
	if strings.Contains(out, "server-action") {
		t.Errorf("the reserved attribute reached the output:\n%s", out)
	}
	// Every other attribute is passed through unread, which is what lets a
	// framework author client behavior in its own vocabulary.
	if !strings.Contains(out, `data-target=\"#r\"`) {
		t.Errorf("a sibling attribute was dropped:\n%s", out)
	}
}

func TestGenerateHonorsACustomActionAttribute(t *testing.T) {
	source := `export component Page(): html { <button server-action="Rename" hx-target="#r">go</button> }`
	got, err := Generate("page.tb.html", []byte(source), GenerateOptions{
		Package:          "id_",
		ServerActions:    map[string]string{"Rename": "/_action/9f3c2ab1e4d7/Rename"},
		ServerActionAttr: "hx-post",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out := string(got); !strings.Contains(out, `hx-post=\"/_action/9f3c2ab1e4d7/Rename\"`) {
		t.Errorf("custom attribute not emitted:\n%s", out)
	}
}

func TestGenerateRejectsAnUnresolvedServerAction(t *testing.T) {
	source := `export component Page(): html { <button server-action="Rename">go</button> }`
	_, err := Generate("page.tb.html", []byte(source), GenerateOptions{Package: "id_"})
	if err == nil {
		t.Fatal("expected an error for an unresolved action")
	}
	if !strings.Contains(err.Error(), "no server action was resolved") {
		t.Errorf("error = %v", err)
	}
}

func TestGenerateAsksTheResolverForAnUnknownAction(t *testing.T) {
	source := `export component Page(): html { <button server-action="Rename">go</button> }`
	got, err := Generate("page.tb.html", []byte(source), GenerateOptions{
		Package: "handlers",
		ServerActionResolver: func(name string) (string, bool) {
			return "/app/rename", name == "Rename"
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// This is the whole point of the hook: a template with no route package
	// beside it still lowers to an address its framework owns.
	if out := string(got); !strings.Contains(out, `data-tb-action=\"/app/rename\"`) {
		t.Errorf("resolver URL not emitted:\n%s", out)
	}
}

func TestGenerateLetsADeclaredActionWinOverTheResolver(t *testing.T) {
	source := `export component Page(): html { <button server-action="Rename">go</button> }`
	got, err := Generate("page.tb.html", []byte(source), GenerateOptions{
		Package:              "id_",
		ServerActions:        map[string]string{"Rename": "/_action/9f3c2ab1e4d7/Rename"},
		ServerActionResolver: func(string) (string, bool) { return "/app/rename", true },
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// Adding a resolver must not retarget an action the declaring package owns.
	out := string(got)
	if !strings.Contains(out, `data-tb-action=\"/_action/9f3c2ab1e4d7/Rename\"`) {
		t.Errorf("the declared endpoint lost to the resolver:\n%s", out)
	}
	if strings.Contains(out, "/app/rename") {
		t.Errorf("resolver answered a declared action:\n%s", out)
	}
}

func TestGenerateNamesBothSourcesWhenNothingResolves(t *testing.T) {
	source := `export component Page(): html { <button server-action="Rename">go</button> }`
	_, err := Generate("page.tb.html", []byte(source), GenerateOptions{
		Package:              "id_",
		ServerActionResolver: func(string) (string, bool) { return "", false },
	})
	if err == nil {
		t.Fatal("expected an error for an unresolved action")
	}
	// With a resolver configured the handler may live anywhere, so the message
	// must not claim it has to sit beside the template.
	if !strings.Contains(err.Error(), "configured resolver") {
		t.Errorf("error = %v, want it to name both attempted sources", err)
	}
}

// generateAction compiles one source with an action resolved to both an address
// and a selector, which is what the integrated routetree path supplies.
func generateAction(t *testing.T, source string) string {
	t.Helper()
	got, err := Generate("page.tb.html", []byte(source), GenerateOptions{
		Package:               "id_",
		ServerActions:         map[string]string{"Save": "/_action/9f3c2ab1e4d7/Save"},
		ServerActionSelectors: map[string]string{"Save": "9f3c2ab1e4d7/Save"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return string(got)
}

func TestFormCarriesBothLoweringsFromOneCompile(t *testing.T) {
	out := generateAction(t, `export component Page(): html { <form server-action="Save"><input name="title" /></form> }`)
	// The scripted half, which a browser runtime binds to.
	if !strings.Contains(out, `data-tb-action=\"/_action/9f3c2ab1e4d7/Save\"`) {
		t.Errorf("the URL attribute is missing:\n%s", out)
	}
	// The native half. Without the method this is a GET form to the current URL,
	// which is what shipped and what this test exists to keep from returning.
	if !strings.Contains(out, `method=\"post\"`) {
		t.Errorf("the form is not a POST form:\n%s", out)
	}
	if !strings.Contains(out, `name=\"_action\" value=\"9f3c2ab1e4d7/Save\"`) {
		t.Errorf("the selector field is missing:\n%s", out)
	}
	if !strings.Contains(out, `CSRFField("_csrf")`) {
		t.Errorf("the token is missing, which the absent method used to suppress:\n%s", out)
	}
}

func TestActionFormWritesNoActionAttribute(t *testing.T) {
	out := generateAction(t, `export component Page(): html { <form server-action="Save"></form> }`)
	// A form declaring no action submits to the document URL, which is already
	// the page pattern, and a POST keeps that URL's query rather than replacing
	// it. Writing one would need the concrete request path at render time.
	// The leading space is what separates a standalone action from the lowered
	// data-tb-action, which ends in the same characters.
	if strings.Contains(out, ` action=\"`) {
		t.Errorf("an action attribute was emitted:\n%s", out)
	}
}

func TestAuthoredPostMethodIsNotDoubled(t *testing.T) {
	out := generateAction(t, `export component Page(): html { <form server-action="Save" method="post"></form> }`)
	if n := strings.Count(out, `method=\"post\"`); n != 1 {
		t.Errorf("method written %d times, want 1:\n%s", n, out)
	}
}

func TestBareButtonKeepsTheScriptedLoweringAlone(t *testing.T) {
	out := generateAction(t, `export component Page(): html { <button server-action="Save">go</button> }`)
	if !strings.Contains(out, `data-tb-action=\"/_action/9f3c2ab1e4d7/Save\"`) {
		t.Errorf("the URL attribute is missing:\n%s", out)
	}
	// A button carries no fields and belongs to no form the generator can see,
	// so there is nothing native to emit and this is not an error.
	if strings.Contains(out, `_action`) && strings.Contains(out, "hidden") {
		t.Errorf("a bare button carried form markup:\n%s", out)
	}
}

func TestFormWithNoSelectorKeepsTheScriptedLoweringAlone(t *testing.T) {
	// A framework resolving an address from its own route table owns the route a
	// form would post to, so this module writes no form markup for it.
	got, err := Generate("page.tb.html", []byte(
		`export component Page(): html { <form server-action="Save"></form> }`), GenerateOptions{
		Package:       "id_",
		ServerActions: map[string]string{"Save": "/app/save"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	out := string(got)
	if strings.Contains(out, `name=\"_action\"`) {
		t.Errorf("a selector was emitted with none supplied:\n%s", out)
	}
	if strings.Contains(out, `method=\"post\"`) {
		t.Errorf("a method was emitted with no native channel to use it:\n%s", out)
	}
}

func TestFormWithNoSelectorStaysTokenFree(t *testing.T) {
	// With no native channel the form is still a GET form, and a token in a GET
	// form reaches history, logs, and referrers. Analysis and emission have to
	// agree about which of the two this is.
	got, err := Generate("page.tb.html", []byte(
		`export component Page(): html { <form server-action="Save"></form> }`), GenerateOptions{
		Package:       "id_",
		ServerActions: map[string]string{"Save": "/app/save"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out := string(got); strings.Contains(out, "CSRFField") {
		t.Errorf("a GET form carried a token:\n%s", out)
	}
}
