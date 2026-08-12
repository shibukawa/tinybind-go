package htmlbind

import (
	"strings"
	"testing"
)

const reportSource = `export component Counter(label: string, count: int): html {
<script component>
export function setup({ label }) {
  return { increment() { console.log("{ not template }") } }
}
</script>
<div><button on-click="increment">{label}{count}</button></div>
}

export component Plain(): html { <p>nothing</p> }
`

func TestComponentScriptsReportsTheBlockVerbatim(t *testing.T) {
	got, err := ComponentScripts("page.tb.html", []byte(reportSource))
	if err != nil {
		t.Fatalf("ComponentScripts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("reported %d components, want 1: %+v", len(got), got)
	}
	entry := got[0]
	if entry.Component != "Counter" {
		t.Errorf("Component = %q", entry.Component)
	}
	// A brace and a template literal inside the block reach the caller as
	// authored, which is the whole reason the module reports it rather than
	// letting the caller find it in the source.
	if !strings.Contains(entry.Script, `console.log("{ not template }")`) {
		t.Errorf("the block was reinterpreted: %q", entry.Script)
	}
	if !strings.Contains(entry.Script, "export function setup({ label })") {
		t.Errorf("the block is not the authored text: %q", entry.Script)
	}
	if entry.Pos.Line == 0 {
		t.Error("the block carries no position")
	}
}

func TestComponentScriptsReportsReferencedHandlers(t *testing.T) {
	got, err := ComponentScripts("page.tb.html", []byte(reportSource))
	if err != nil {
		t.Fatalf("ComponentScripts: %v", err)
	}
	if want := []string{"increment"}; strings.Join(got[0].Handlers, ",") != strings.Join(want, ",") {
		t.Errorf("Handlers = %v, want %v", got[0].Handlers, want)
	}
}

func TestComponentScriptsReportsTheDeclaredParameters(t *testing.T) {
	got, err := ComponentScripts("page.tb.html", []byte(reportSource))
	if err != nil {
		t.Fatalf("ComponentScripts: %v", err)
	}
	if want := "label,count"; strings.Join(got[0].Parameters, ",") != want {
		t.Errorf("Parameters = %v, want %s", got[0].Parameters, want)
	}
}

func TestComponentScriptsOmitsAComponentWithNoBlock(t *testing.T) {
	got, err := ComponentScripts("page.tb.html", []byte(reportSource))
	if err != nil {
		t.Fatalf("ComponentScripts: %v", err)
	}
	for _, entry := range got {
		if entry.Component == "Plain" {
			t.Errorf("a component declaring no block was reported: %+v", entry)
		}
	}
}

func TestComponentScriptsFailsLikeGenerate(t *testing.T) {
	// The reader runs the analysis Generate runs, so a broken module fails here
	// with the same diagnostic rather than yielding a partial answer.
	_, err := ComponentScripts("page.tb.html", []byte(
		`export component Counter(): html { <button server-action="lower">x</button> }`))
	if err == nil {
		t.Fatal("expected the analysis error")
	}
	if !strings.Contains(err.Error(), "exported Go function name") {
		t.Errorf("error = %v", err)
	}
}

func TestComponentScriptsIsStableAcrossRuns(t *testing.T) {
	first, err := ComponentScripts("page.tb.html", []byte(reportSource))
	if err != nil {
		t.Fatalf("ComponentScripts: %v", err)
	}
	second, err := ComponentScripts("page.tb.html", []byte(reportSource))
	if err != nil {
		t.Fatalf("ComponentScripts: %v", err)
	}
	if len(first) != len(second) || first[0].Script != second[0].Script ||
		strings.Join(first[0].Handlers, ",") != strings.Join(second[0].Handlers, ",") {
		t.Error("the report is not stable for unchanged input")
	}
}
