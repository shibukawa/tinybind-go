package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// firestoreModule writes a temp module whose single source is src, and returns
// its directory.
func firestoreModule(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	return dir
}

// firestoreSource composes a package that declares body and calls the
// firestorebind functions named in calls, so discovery has something to find.
func firestoreSource(body, calls string) string {
	return `package sample

import (
	"context"

	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

` + body + `

var _ = context.Background
var _ = datastore.String

func use(ctx context.Context) {
` + calls + `
}
`
}

func generateFirestore(t *testing.T, src string) string {
	t.Helper()
	dir := firestoreModule(t, src)
	plan, err := generator.AnalyzeFirestoreEntitiesWithOptions(dir, generator.DefaultOptions())
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	code, err := generator.EmitFirestoreEntities(plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return string(code)
}

func analyzeFirestoreErr(t *testing.T, src string) error {
	t.Helper()
	dir := firestoreModule(t, src)
	_, err := generator.AnalyzeFirestoreEntitiesWithOptions(dir, generator.DefaultOptions())
	return err
}

const nameKeyedReading = "type Reading struct {\n" +
	"\tID string `firestore:\"-,name\"`\n" +
	"\tNote string `firestore:\"note\"`\n" +
	"}"

func TestFirestoreEmitsTheFourMethods(t *testing.T) {
	code := generateFirestore(t, firestoreSource(nameKeyedReading,
		"\t_, _ = firestorebind.Load[Reading](ctx, datastore.NameKey(\"Reading\", \"r\"))\n"+
			"\t_, _ = firestorebind.Store(ctx, Reading{})\n"))

	for _, want := range []string{
		"func (v Reading) Kind() string { return \"Reading\" }",
		"func (v Reading) EntityKey() datastore.Key {",
		"func (v Reading) EncodeEntity() datastore.Entity {",
		"func (v *Reading) DecodeEntity(e datastore.Entity) error {",
		"var _ firestorebind.EntityEncoder = Reading{}",
		"var _ firestorebind.EntityDecoder = (*Reading)(nil)",
		"var _ firestorebind.Keyer = Reading{}",
		"var _ firestorebind.Kinder = Reading{}",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code is missing %q\n%s", want, code)
		}
	}
}

// A read-only use emits no encoder, which is what usage-directed generation is
// for: the write side costs nothing in a package that never writes.
func TestFirestoreUsageDirection(t *testing.T) {
	code := generateFirestore(t, firestoreSource(nameKeyedReading,
		"\t_, _ = firestorebind.Load[Reading](ctx, datastore.NameKey(\"Reading\", \"r\"))\n"))

	if strings.Contains(code, "func (v Reading) EncodeEntity()") {
		t.Errorf("a read-only package got an encoder\n%s", code)
	}
	if !strings.Contains(code, "func (v *Reading) DecodeEntity(") {
		t.Errorf("a read use got no decoder\n%s", code)
	}
	// The key builder comes from the tag rather than from a discovered call,
	// because the documented read is Load(ctx, v.EntityKey()) and using a method
	// is not a call the generator can find.
	if !strings.Contains(code, "func (v Reading) EntityKey()") {
		t.Errorf("a keyed type got no key builder\n%s", code)
	}
}

// The identity field is carried by the key, so it must not also be written as a
// property; that is the difference from the DynamoDB partition key.
func TestFirestoreIdentityIsNotAProperty(t *testing.T) {
	code := generateFirestore(t, firestoreSource(nameKeyedReading,
		"\t_, _ = firestorebind.Store(ctx, Reading{})\n"))

	if strings.Contains(code, `properties["ID"]`) || strings.Contains(code, `properties["-"]`) {
		t.Errorf("the identity field was written as a property\n%s", code)
	}
	if !strings.Contains(code, "datastore.NameKey(v.Kind(), string(v.ID))") {
		t.Errorf("the key is not built from the identity field\n%s", code)
	}
}

// An identity tag with a real property name is the deliberate opt-in to storing
// identity twice, for a caller who needs to filter on it.
func TestFirestoreIdentityCanBeStoredOnPurpose(t *testing.T) {
	body := "type Reading struct {\n" +
		"\tID string `firestore:\"sensor,name\"`\n" +
		"\tNote string `firestore:\"note\"`\n" +
		"}"
	code := generateFirestore(t, firestoreSource(body,
		"\t_, _ = firestorebind.Store(ctx, Reading{})\n"))

	if !strings.Contains(code, `properties["sensor"]`) {
		t.Errorf("a named identity field was not stored\n%s", code)
	}
}

func TestFirestoreGenerationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknown option",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tOops string `firestore:\"oops,stringset\"`\n}",
			want: `unknown firestore tag option "stringset"`,
		},
		{
			name: "duplicate property",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tA string `firestore:\"note\"`\n\tB string `firestore:\"note\"`\n}",
			want: `both map to property "note"`,
		},
		{
			name: "two identities",
			body: "type Reading struct {\n\tA string `firestore:\"-,name\"`\n\tB string `firestore:\"-,name\"`\n}",
			want: "are both name",
		},
		{
			name: "name and id together",
			body: "type Reading struct {\n\tA string `firestore:\"-,name\"`\n\tB int64 `firestore:\"-,id\"`\n}",
			want: "declare both a name and an id key",
		},
		{
			name: "name on a non-string",
			body: "type Reading struct {\n\tA int64 `firestore:\"-,name\"`\n}",
			want: "name needs a string field",
		},
		{
			name: "id on a non-integer",
			body: "type Reading struct {\n\tA string `firestore:\"-,id\"`\n}",
			want: "id needs an int64 field",
		},
		{
			name: "parent on an unrelated type",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tP string `firestore:\"-,parent\"`\n}",
			want: "parent needs a datastore.Key field",
		},
		{
			// The driver's own mapper reads this spelling, and two mappings on
			// one field disagree on every renamed property.
			name: "driver tag without ours",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tNote string `datastore:\"note\"`\n}",
			want: `found tag "datastore" but no "firestore" tag`,
		},
		{
			name: "map field",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tM map[string]string `firestore:\"m\"`\n}",
			want: "a map has no Datastore property type",
		},
		{
			// Datastore's integer is an int64, and the driver refuses to
			// marshal anything wider, so accepting the field would defer the
			// failure to the first large value.
			name: "uint64 field",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tN uint64 `firestore:\"n\"`\n}",
			want: "uint64 exceeds the int64 a Datastore integer holds",
		},
		{
			name: "uint field",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tN uint `firestore:\"n\"`\n}",
			want: "uint is 64 bits on the platforms this targets",
		},
		{
			name: "noindex on an unstored field",
			body: "type Reading struct {\n\tID string `firestore:\"-,name,noindex\"`\n}",
			want: "noindex on a field that is not stored",
		},
		{
			name: "method collision",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n}\n\nfunc (r Reading) EncodeEntity() datastore.Entity { return datastore.Entity{} }",
			want: "already declares EncodeEntity",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := analyzeFirestoreErr(t, firestoreSource(test.body,
				"\t_, _ = firestorebind.Load[Reading](ctx, datastore.NameKey(\"Reading\", \"r\"))\n"))
			if err == nil {
				t.Fatalf("got no error, want one containing %q", test.want)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %q does not contain %q", err, test.want)
			}
		})
	}
}

// Generation is deterministic: the same input has to produce byte-identical
// output, or a regenerate would churn the diff.
func TestFirestoreGenerationIsDeterministic(t *testing.T) {
	src := firestoreSource(nameKeyedReading,
		"\t_, _ = firestorebind.Load[Reading](ctx, datastore.NameKey(\"Reading\", \"r\"))\n"+
			"\t_, _ = firestorebind.Store(ctx, Reading{})\n")
	dir := firestoreModule(t, src)

	var first string
	for i := range 3 {
		plan, err := generator.AnalyzeFirestoreEntitiesWithOptions(dir, generator.DefaultOptions())
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		code, err := generator.EmitFirestoreEntities(plan)
		if err != nil {
			t.Fatalf("emit: %v", err)
		}
		if i == 0 {
			first = string(code)
			continue
		}
		if string(code) != first {
			t.Fatalf("run %d differs from the first", i)
		}
	}
}

// A nested type inherits its parent's operations but never a key: an entityValue
// carries none.
func TestFirestoreNestedTypeGetsNoKey(t *testing.T) {
	body := "type Site struct {\n\tCity string `firestore:\"city\"`\n}\n\n" +
		"type Reading struct {\n" +
		"\tID string `firestore:\"-,name\"`\n" +
		"\tWhere Site `firestore:\"site\"`\n" +
		"}"
	code := generateFirestore(t, firestoreSource(body,
		"\t_, _ = firestorebind.Store(ctx, Reading{})\n"))

	if !strings.Contains(code, "func (v Site) EncodeEntity()") {
		t.Errorf("the nested type got no encoder\n%s", code)
	}
	if strings.Contains(code, "func (v Site) EntityKey()") {
		t.Errorf("the nested type got a key builder; an entityValue carries no key\n%s", code)
	}
	if !strings.Contains(code, "datastore.Nested(v.Where.EncodeEntity())") {
		t.Errorf("the nested field is not encoded through its own codec\n%s", code)
	}
}
