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
	"time"

	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

` + body + `

var _ = context.Background
var _ = datastore.String
var _ = time.Now

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

// TestFirestoreUsageFollowsHandleCallSites pins the parameter form of
// requirement:firestore-parameter-api. The gap it closes was the same one
// TestDynamoUsageFollowsHandleCallSites closes, and it hid longer here because
// the downstream scaffold carried a declaration and no Go call at all.
func TestFirestoreUsageFollowsHandleCallSites(t *testing.T) {
	tests := []struct {
		name    string
		call    string
		want    []string
		notWant []string
	}{
		{
			name:    "loadon emits the decoder, not the encoder",
			call:    "\t_, _ = firestorebind.LoadOn[Reading](ctx, h, datastore.NameKey(\"Reading\", \"r\"))\n",
			want:    []string{"func (v *Reading) DecodeEntity("},
			notWant: []string{"EncodeEntity"},
		},
		{
			name:    "loadallon discovers the element",
			call:    "\t_, _, _, _ = firestorebind.LoadAllOn[Reading](ctx, h, nil)\n",
			want:    []string{"func (v *Reading) DecodeEntity("},
			notWant: []string{"EncodeEntity"},
		},
		{
			name:    "queryon emits the decoder",
			call:    "\t_ = firestorebind.QueryOn[Reading](ctx, h, nil)\n",
			want:    []string{"func (v *Reading) DecodeEntity("},
			notWant: []string{"EncodeEntity"},
		},
		{
			name:    "querypageon emits the decoder",
			call:    "\t_, _ = firestorebind.QueryPageOn[Reading](ctx, h, nil)\n",
			want:    []string{"func (v *Reading) DecodeEntity("},
			notWant: []string{"EncodeEntity"},
		},
		{
			name:    "storeon emits the encoder, not the decoder",
			call:    "\t_, _ = firestorebind.StoreOn(ctx, h, Reading{})\n",
			want:    []string{"func (v Reading) EncodeEntity("},
			notWant: []string{"DecodeEntity"},
		},
		{
			name:    "inserton emits the encoder",
			call:    "\t_, _ = firestorebind.InsertOn(ctx, h, Reading{})\n",
			want:    []string{"func (v Reading) EncodeEntity("},
			notWant: []string{"DecodeEntity"},
		},
		{
			name:    "updateon emits the encoder",
			call:    "\t_ = firestorebind.UpdateOn(ctx, h, Reading{})\n",
			want:    []string{"func (v Reading) EncodeEntity("},
			notWant: []string{"DecodeEntity"},
		},
		{
			name:    "storeallon discovers the slice element",
			call:    "\t_, _ = firestorebind.StoreAllOn(ctx, h, []Reading{})\n",
			want:    []string{"func (v Reading) EncodeEntity("},
			notWant: []string{"DecodeEntity"},
		},
		{
			name:    "insertallon discovers the slice element",
			call:    "\t_, _ = firestorebind.InsertAllOn(ctx, h, []Reading{})\n",
			want:    []string{"func (v Reading) EncodeEntity("},
			notWant: []string{"DecodeEntity"},
		},
		{
			name:    "removeon emits the key, and neither codec",
			call:    "\t_ = firestorebind.RemoveOn(ctx, h, Reading{})\n",
			want:    []string{"func (v Reading) EntityKey("},
			notWant: []string{"EncodeEntity", "DecodeEntity"},
		},
		{
			name:    "removeallon emits the key",
			call:    "\t_ = firestorebind.RemoveAllOn(ctx, h, []Reading{})\n",
			want:    []string{"func (v Reading) EntityKey("},
			notWant: []string{"EncodeEntity", "DecodeEntity"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code := generateFirestore(t, firestoreSource(nameKeyedReading,
				"\tvar h firestorebind.Handle\n"+test.call))
			for _, want := range test.want {
				if !strings.Contains(code, want) {
					t.Errorf("missing %q in:\n%s", want, code)
				}
			}
			for _, notWant := range test.notWant {
				if strings.Contains(code, notWant) {
					t.Errorf("unexpected %q in:\n%s", notWant, code)
				}
			}
		})
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
			// A TTL policy expires by reading a stored timestamp, so a policy
			// pointed at anything else expires nothing.
			name: "ttl on a non-timestamp",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tExp int64 `firestore:\"exp,ttl\"`\n}",
			want: "ttl needs a time.Time field",
		},
		{
			// Naming a property that is never written describes a policy that
			// can never fire, which is the noindex contradiction again.
			name: "ttl on an unstored field",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tExp time.Time `firestore:\"-,ttl\"`\n}",
			want: "ttl on a field that is not stored",
		},
		{
			name: "two ttl fields",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tA time.Time `firestore:\"a,ttl\"`\n\tB time.Time `firestore:\"b,ttl\"`\n}",
			want: "are both ttl",
		},
		{
			name: "method collision",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n}\n\nfunc (r Reading) EncodeEntity() datastore.Entity { return datastore.Entity{} }",
			want: "already declares EncodeEntity",
		},
		{
			name: "expiry accessor collision",
			body: "type Reading struct {\n\tID string `firestore:\"-,name\"`\n\tExp time.Time `firestore:\"exp,ttl\"`\n}\n\nfunc (r Reading) ExpiryProperty() (string, bool) { return \"\", false }",
			want: "already declares ExpiryProperty",
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
