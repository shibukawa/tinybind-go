package dynamobind_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/dynamobind"
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

func format(t *testing.T, source string) string {
	t.Helper()
	out, err := dynamobind.Format("x.tb.dynamo", []byte(source), syntax.PrintOptions{})
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	return string(out)
}

func TestFormatCanonicalizesOneLineBody(t *testing.T) {
	got := format(t, "export statement ReadingsForSensor(sensor: Sensor): dynamo.many<Reading> {\n  table readings; key sensor = {sensor}\n}")
	want := "export statement ReadingsForSensor(sensor: Sensor): dynamo.many<Reading> {\n  table readings\n  key sensor = {sensor}\n}\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatKeepsFixtureComments(t *testing.T) {
	source, err := os.ReadFile("../../internal/dynamofixture/readings.tb.dynamo")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	got := format(t, string(source))
	t.Logf("\n%s", got)
	if !strings.Contains(got, "// Access patterns for Reading.") {
		t.Fatalf("header comment lost:\n%s", got)
	}
	if format(t, got) != got {
		t.Fatalf("not idempotent:\n%s", got)
	}
}

// TestFormatKeepsDeclarations is the dynamo half of rule:template-format-fidelity:
// the declarations the generator plans from must survive formatting unchanged.
func TestFormatKeepsDeclarations(t *testing.T) {
	source, err := os.ReadFile("../../internal/dynamofixture/readings.tb.dynamo")
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	before, err := dynamobind.ParseQueries("readings.tb.dynamo", source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	formatted := format(t, string(source))
	after, err := dynamobind.ParseQueries("readings.tb.dynamo", []byte(formatted))
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if len(before) != len(after) {
		t.Fatalf("declaration count changed: %d then %d", len(before), len(after))
	}
	for i := range before {
		// Line numbers move; that is exactly what formatting does.
		a, b := before[i], after[i]
		a.Line, b.Line = 0, 0
		a.TableLine, b.TableLine = 0, 0
		for j := range a.Params {
			a.Params[j].Line = 0
		}
		for j := range b.Params {
			b.Params[j].Line = 0
		}
		for j := range a.Key {
			a.Key[j].Line = 0
		}
		for j := range b.Key {
			b.Key[j].Line = 0
		}
		if fmt.Sprintf("%+v", a) != fmt.Sprintf("%+v", b) {
			t.Errorf("declaration %d changed:\n%+v\n%+v", i, a, b)
		}
	}
}
