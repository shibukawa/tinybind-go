package htmlbind

import (
	"strings"
	"testing"
)

const paramSource = `record Row { id: string, tags: string[] }

export component Card(label: string, count: int?, row: Row): html {
<script component>
export function setup({ label, count, row }) { return {} }
</script>
<div class="card">{label}</div>
}`

func generateParams(t *testing.T, source string, options GenerateOptions) (string, error) {
	t.Helper()
	options.Package = "id_"
	got, err := GenerateModule("page.tb.html", []byte(source), options)
	if err != nil {
		return "", err
	}
	return string(got.GoSource), nil
}

func TestComponentParametersAreEmittedOntoTheRoot(t *testing.T) {
	out, err := generateParams(t, paramSource, GenerateOptions{
		ComponentParameters: map[string][]string{"Card": {"label", "count", "row"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, `Attr("data-tb-props"`) {
		t.Errorf("the parameter attribute is missing:\n%s", out)
	}
	// A record is emitted rather than refused: the rule that refuses one is the
	// query-string rule of a reloadable component, and an attribute holding JSON
	// is not a query string.
	if !strings.Contains(out, `JSONMember(body, "row"`) {
		t.Errorf("a record parameter was not emitted:\n%s", out)
	}
	// An absent optional omits its key rather than writing null.
	if !strings.Contains(out, "if p.Count != nil {") {
		t.Errorf("an optional was emitted unconditionally:\n%s", out)
	}
	if strings.Contains(out, "null") {
		t.Errorf("an absence was written as null:\n%s", out)
	}
	// The attribute op writes its value verbatim, so the closure escapes.
	if !strings.Contains(out, `htmlbind.Escape("{" + body + "}")`) {
		t.Errorf("the object is not escaped for an attribute:\n%s", out)
	}
}

func TestComponentParametersEmitNothingWhenUnnamed(t *testing.T) {
	out, err := generateParams(t, paramSource, GenerateOptions{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(out, "data-tb-props") {
		t.Errorf("a component naming no parameters emitted an attribute:\n%s", out)
	}
	// The declaration marker is unaffected, so the two ride the same root
	// independently.
	if !strings.Contains(out, "data-tb-component") {
		t.Errorf("the declaration marker was lost:\n%s", out)
	}
}

func TestComponentParametersRejectAnUnrepresentableType(t *testing.T) {
	source := `export component Card(body: html): html {
<script component>
export function setup({ body }) {}
</script>
<div>{body}</div>
}`
	_, err := generateParams(t, source, GenerateOptions{
		ComponentParameters: map[string][]string{"Card": {"body"}},
	})
	if err == nil {
		t.Fatal("expected an error for a parameter with no JSON form")
	}
	if !strings.Contains(err.Error(), "no JSON form") {
		t.Errorf("error = %v", err)
	}
}

func TestComponentParametersRejectAnUndeclaredName(t *testing.T) {
	_, err := generateParams(t, paramSource, GenerateOptions{
		ComponentParameters: map[string][]string{"Card": {"nope"}},
	})
	if err == nil {
		t.Fatal("expected an error for a parameter the component does not declare")
	}
	if !strings.Contains(err.Error(), "has no parameter") {
		t.Errorf("error = %v", err)
	}
}

func TestComponentParametersNeedAScriptBlock(t *testing.T) {
	// Nothing would consume the object, and the single-root invariant it rides
	// exists for the declaration marker of a component that declares a block.
	_, err := generateParams(t, `export component Card(label: string): html { <div>{label}</div> }`,
		GenerateOptions{ComponentParameters: map[string][]string{"Card": {"label"}}})
	if err == nil {
		t.Fatal("expected an error for a component with no script block")
	}
	if !strings.Contains(err.Error(), "declares no script block") {
		t.Errorf("error = %v", err)
	}
}

func TestComponentParametersRejectAnUnknownComponent(t *testing.T) {
	_, err := generateParams(t, paramSource, GenerateOptions{
		ComponentParameters: map[string][]string{"Nope": {"label"}},
	})
	if err == nil {
		t.Fatal("expected an error for a component this module does not declare")
	}
	if !strings.Contains(err.Error(), "does not declare") {
		t.Errorf("error = %v", err)
	}
}
