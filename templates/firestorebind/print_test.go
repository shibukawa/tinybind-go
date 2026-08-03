package firestorebind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/firestorebind"
	"github.com/shibukawa/tinybind-go/templates/templatefmt"
)

func format(t *testing.T, source string) string {
	t.Helper()
	out, err := templatefmt.Source("queries.tb.firestore", []byte(source), templatefmt.Options{})
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	return string(out)
}

func TestFormatNormalizesLayout(t *testing.T) {
	got := format(t, "export    statement BySensor(sensor:Sensor):firestore.many<Reading>{where sensor=={sensor};order at desc}\n")
	want := `export statement BySensor(sensor: Sensor): firestore.many<Reading> {
  where sensor == {sensor}
  order at desc
}
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Clauses come out in a fixed order regardless of how they were written, so
// reading order follows what the query does rather than what was typed first.
func TestFormatOrdersClauses(t *testing.T) {
	got := format(t, `export statement A(p: datastore.Key, n: int): firestore.batch<Reading> {
  index at desc
  offset 5
  limit {n}
  order at
  ancestor {p}
  where celsius > {n}
}
`)
	want := `export statement A(p: datastore.Key, n: int): firestore.batch<Reading> {
  where celsius > {n}
  ancestor {p}
  order at
  limit {n}
  offset 5
  index at desc
}
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// An ascending direction is the default, so writing it adds nothing and the
// formatter drops it.
func TestFormatDropsRedundantAscending(t *testing.T) {
	got := format(t, "export statement A(s: Sensor): firestore.many<Reading> {\n where sensor == {s}\n order at asc, celsius desc\n}\n")
	if !strings.Contains(got, "order at, celsius desc") {
		t.Errorf("ascending was not dropped:\n%s", got)
	}
}

// Comments survive, which the grammar itself cannot do: it drops them while
// tokenizing, so the formatter reads them back from the source.
func TestFormatKeepsComments(t *testing.T) {
	got := format(t, `// Everything a sensor reported.
export statement A(s: Sensor): firestore.many<Reading> {
  // Only that sensor.
  where sensor == {s} // and nothing else
}

// A note at the end.
`)
	for _, want := range []string{
		"// Everything a sensor reported.",
		"// Only that sensor.",
		"// and nothing else",
		"// A note at the end.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("comment %q was dropped:\n%s", want, got)
		}
	}
}

// Formatting an already-formatted source changes nothing, which is what makes
// it safe to run on save.
func TestFormatIsIdempotent(t *testing.T) {
	sources := []string{
		"export statement A(s: Sensor): firestore.many<Reading> {\n where sensor == {s}\n}\n",
		"// A comment.\nexport statement A(p: datastore.Key): firestore.batch<Reading> {\n ancestor {p}\n order at desc\n limit 10\n index at desc\n}\n",
		"export statement A(xs: []Sensor): firestore.count<Reading> {\n where sensor in {xs}\n}\n",
	}
	for _, source := range sources {
		once := format(t, source)
		twice := format(t, once)
		if once != twice {
			t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", once, twice)
		}
	}
}

// A source that does not parse is reported rather than guessed at, so a caller
// leaves the file untouched.
func TestFormatRejectsUnparsable(t *testing.T) {
	_, err := templatefmt.Source("queries.tb.firestore", []byte("statement {"), templatefmt.Options{})
	if err == nil {
		t.Fatal("an unparsable source formatted successfully")
	}
}

// The language is identified by the file name, beside the other three.
func TestIdentifiesTheFirestorePattern(t *testing.T) {
	format, err := templatefmt.Identify("readings.tb.firestore", templatefmt.Options{})
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if format != templatefmt.Firestore {
		t.Errorf("got %q, want %q", format, templatefmt.Firestore)
	}
}

func TestDefaultPatternMatchesTheGenerator(t *testing.T) {
	if firestorebind.DefaultTemplatePattern != templatefmt.DefaultFirestorePattern {
		t.Errorf("the formatter and the grammar disagree on the pattern: %q vs %q",
			firestorebind.DefaultTemplatePattern, templatefmt.DefaultFirestorePattern)
	}
}
