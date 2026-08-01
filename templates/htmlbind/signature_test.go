package htmlbind

import (
	"strings"
	"testing"
)

func signatures(t *testing.T, source string) []Signature {
	t.Helper()
	got, err := Signatures("page.tb.html", []byte(source))
	if err != nil {
		t.Fatalf("Signatures: %v", err)
	}
	return got
}

func TestSignaturesReportsGoTypes(t *testing.T) {
	got := signatures(t, `
type User { name: string }

export component Page(id: string, count: int, ratio: float, ok: bool, user: User): html {
  <p>{id}</p>
}
`)
	if len(got) != 1 {
		t.Fatalf("signatures = %d, want 1", len(got))
	}
	page := got[0]
	if page.Name != "Page" || !page.Exported {
		t.Errorf("Name/Exported = %q/%v", page.Name, page.Exported)
	}

	want := []SignatureParam{
		{Name: "id", GoType: "string"},
		{Name: "count", GoType: "int"},
		{Name: "ratio", GoType: "float64"},
		{Name: "ok", GoType: "bool"},
		{Name: "user", GoType: "User"},
	}
	if len(page.Parameters) != len(want) {
		t.Fatalf("parameters = %d, want %d: %+v", len(page.Parameters), len(want), page.Parameters)
	}
	for i, w := range want {
		got := page.Parameters[i]
		if got.Name != w.Name || got.GoType != w.GoType {
			t.Errorf("parameter %d = {%s %s}, want {%s %s}", i, got.Name, got.GoType, w.Name, w.GoType)
		}
	}
}

func TestSignaturesPreservesDeclarationOrder(t *testing.T) {
	got := signatures(t, `
export component First(a: string): html { <p>{a}</p> }
export component Second(b: string): html { <p>{b}</p> }
component Third(c: string): html { <p>{c}</p> }
`)
	if len(got) != 3 {
		t.Fatalf("signatures = %d, want 3", len(got))
	}
	for i, want := range []string{"First", "Second", "Third"} {
		if got[i].Name != want {
			t.Errorf("signature %d = %q, want %q", i, got[i].Name, want)
		}
	}
	if got[2].Exported {
		t.Error("private component reported as exported")
	}
}

func TestSignaturesWrapsAsyncParameters(t *testing.T) {
	got := signatures(t, `
type Order { id: string }

export component Page(orders: async Order[]): html {
  {await list = orders}
    <ul>{for o in list}<li>{o.id}</li>{/for}</ul>
  {fallback}
    <p>loading</p>
  {/await}
}
`)
	page := got[0]
	if len(page.Parameters) != 1 {
		t.Fatalf("parameters = %+v", page.Parameters)
	}
	param := page.Parameters[0]
	if !param.Async {
		t.Error("async parameter not marked")
	}
	// The handle wraps the whole settled type, so it is one Pending of a slice.
	if param.GoType != "htmlbind.Pending[[]Order]" {
		t.Errorf("GoType = %q, want htmlbind.Pending[[]Order]", param.GoType)
	}
	if !strings.Contains(param.TemplateType, "async") {
		t.Errorf("TemplateType = %q, want it to keep the async spelling", param.TemplateType)
	}
}

func TestSignaturesMarksSlotParameters(t *testing.T) {
	got := signatures(t, `
export component Layout(children: html): html {
  <main><slot required /></main>
}
`)
	layout := got[0]
	if len(layout.Parameters) != 1 {
		t.Fatalf("parameters = %+v", layout.Parameters)
	}
	if !layout.Parameters[0].Slot {
		t.Error("html parameter not marked as a slot")
	}
	if layout.Parameters[0].GoType != "htmlbind.Fragment" {
		t.Errorf("GoType = %q, want htmlbind.Fragment", layout.Parameters[0].GoType)
	}
}

func TestSignaturesArrayAndOptional(t *testing.T) {
	got := signatures(t, `
type Order { id: string }

export component Page(orders: Order[], note: string?): html {
  <p>{note}</p>
}
`)
	page := got[0]
	byName := map[string]SignatureParam{}
	for _, p := range page.Parameters {
		byName[p.Name] = p
	}
	if got := byName["orders"].GoType; got != "[]Order" {
		t.Errorf("orders GoType = %q, want []Order", got)
	}
	if got := byName["note"].GoType; got != "*string" {
		t.Errorf("note GoType = %q, want *string", got)
	}
}

func TestSignaturesZeroParameterComponent(t *testing.T) {
	got := signatures(t, `export component Page(): html { <p>hi</p> }`)
	if len(got) != 1 {
		t.Fatalf("signatures = %+v", got)
	}
	if len(got[0].Parameters) != 0 {
		t.Errorf("parameters = %+v, want none", got[0].Parameters)
	}
}

func TestSignaturesFailsOnAnInvalidModule(t *testing.T) {
	// Analysis runs in full, so a module that would not compile is reported
	// here rather than yielding a partial signature.
	if _, err := Signatures("page.tb.html", []byte(`export component Page(x: Missing): html { <p>{x}</p> }`)); err == nil {
		t.Fatal("unresolved type accepted, want error")
	}
}

func TestSignaturesFailsOnAParseError(t *testing.T) {
	if _, err := Signatures("page.tb.html", []byte(`export component Page(`)); err == nil {
		t.Fatal("unparsable source accepted, want error")
	}
}

func TestLookup(t *testing.T) {
	got := signatures(t, `
export component Page(a: string): html { <p>{a}</p> }
export component Other(b: string): html { <p>{b}</p> }
`)
	page, ok := Lookup(got, "Page")
	if !ok || page.Name != "Page" {
		t.Errorf("Lookup(Page) = %+v, %v", page, ok)
	}
	if _, ok := Lookup(got, "Absent"); ok {
		t.Error("Lookup found a declaration that does not exist")
	}
}
