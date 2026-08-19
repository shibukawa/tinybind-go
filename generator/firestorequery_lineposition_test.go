package generator_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

const firestorePositionDecls = `
export statement BySensor(sensor: Sensor): firestore.many<Reading> {
  where sensor == {sensor}
}

export statement Since(from: time.Time): firestore.batch<Reading> {
  where at > {from}
}
`

// Firestore declarations map on the same terms as dynamo ones: the parser
// records a line and no column, so a whole generated function is the span.
func TestFirestoreQueryLineDirectivesMapEachDeclaration(t *testing.T) {
	dir := firestoreQueryModule(t, queryPackage(queryReading), firestorePositionDecls)
	options := generator.DefaultOptions()
	options.TemplateLineDirectives = true
	code, err := (&generator.Generator{Options: options}).EmitFirestoreQueriesFor(dir)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	// BySensor is declared on line 2 and Since on line 6. The path is absolute,
	// which is the only form go build and go vet both print correctly.
	source := filepath.Join(dir, "queries.tb.firestore")
	for _, line := range []int{2, 6} {
		want := fmt.Sprintf("//line %s:%d", source, line)
		if !strings.Contains(string(code), want) {
			t.Errorf("missing %q:\n%s", want, code)
		}
	}
	if strings.Contains(string(code), "tinybind_restore.go") {
		t.Fatalf("a restore was left unresolved:\n%s", code)
	}
	assertRestoresNameTheirOwnLine(t, code, "firestorequery_gen.go")
	for index, line := range strings.Split(string(code), "\n") {
		if strings.Contains(line, "//line ") && !strings.HasPrefix(line, "//line ") {
			t.Fatalf("line %d holds an indented directive: %q", index+1, line)
		}
	}
}

func TestFirestoreQueryLineDirectivesAreOffByDefault(t *testing.T) {
	code := generateFirestoreQuery(t, firestorePositionDecls)
	if strings.Contains(code, "//line ") {
		t.Fatalf("directives emitted with the option off:\n%s", code)
	}
}
