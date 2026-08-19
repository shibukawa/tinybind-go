package generator_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// A dynamo declaration carries a source path and a line and no column, so the
// mapping is per declaration and the whole generated function is the span. The
// constants above it are named by the emitter, not by the declaration, and keep
// their own position.
func TestDynamoQueryLineDirectivesMapEachDeclaration(t *testing.T) {
	dir := dynamoContextModule(t, nil)
	options := generator.DefaultOptions()
	options.TemplateLineDirectives = true
	generated := generateDynamoQueries(t, dir, options)

	// ReadingsSince is declared on line 1 and ReadingsPage on line 6 of the
	// fixture's readings.tb.dynamo. The path is absolute, which is the only form
	// go build and go vet both print correctly.
	source := filepath.Join(dir, "readings.tb.dynamo")
	for _, line := range []int{1, 6} {
		want := fmt.Sprintf("//line %s:%d", source, line)
		if !bytes.Contains(generated, []byte(want)) {
			t.Errorf("missing %q:\n%s", want, generated)
		}
	}
	if bytes.Contains(generated, []byte("tinybind_restore.go")) {
		t.Fatalf("a restore was left unresolved:\n%s", generated)
	}
	assertRestoresNameTheirOwnLine(t, generated, "dynamoquery_gen.go")
	for index, line := range strings.Split(string(generated), "\n") {
		if strings.Contains(line, "//line ") && !strings.HasPrefix(line, "//line ") {
			t.Fatalf("line %d holds an indented directive: %q", index+1, line)
		}
	}
}

func TestDynamoQueryLineDirectivesAreOffByDefault(t *testing.T) {
	dir := dynamoContextModule(t, nil)
	generated := generateDynamoQueries(t, dir, generator.DefaultOptions())
	if bytes.Contains(generated, []byte("//line ")) {
		t.Fatalf("directives emitted with the option off:\n%s", generated)
	}
}
