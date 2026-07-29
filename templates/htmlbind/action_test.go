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
