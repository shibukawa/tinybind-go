//go:build !tinygo

package firestorefixture_test

import (
	"math"
	"testing"
	"time"

	"github.com/shibukawa/tinygodriver/nosql/datastore"

	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinybind-go/internal/firestorefixture"
)

// sample is a Reading with every supported property form set to something that
// is not a zero value, so a field the codec forgets shows up as a difference
// rather than as a coincidence.
func sample() firestorefixture.Reading {
	extra := "present"
	return firestorefixture.Reading{
		ID:     "r-1",
		Sensor: "s-9",
		Note:   "多バイト note",
		At:     time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC),
		Scale:  1.5,
		Small:  0.25,
		Count:  -7,
		Tiny:   65535,
		Active: true,
		Blob:   []byte{0x00, 0xff, 0x10},
		Ref:    datastore.NameKey("Other", "o-1"),
		Where:  datastore.LatLng{Latitude: 35.68, Longitude: 139.76},
		Tags:   []string{"a", "b"},
		Nested: firestorefixture.Site{City: "Tokyo", Zip: "100-0001"},
		Extra:  &extra,
		Body:   "long body that is never filtered on",
		Raw:    datastore.String("raw"),
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	want := sample()
	entity := want.EncodeEntity()

	var got firestorefixture.Reading
	if err := got.DecodeEntity(entity); err != nil {
		t.Fatalf("DecodeEntity: %v", err)
	}

	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
	if got.Note != want.Note {
		t.Errorf("Note: got %q, want %q", got.Note, want.Note)
	}
	if !got.At.Equal(want.At) {
		t.Errorf("At: got %v, want %v", got.At, want.At)
	}
	if got.Scale != want.Scale || got.Small != want.Small {
		t.Errorf("floats: got %v/%v, want %v/%v", got.Scale, got.Small, want.Scale, want.Small)
	}
	if got.Count != want.Count || got.Tiny != want.Tiny {
		t.Errorf("integers: got %v/%v, want %v/%v", got.Count, got.Tiny, want.Count, want.Tiny)
	}
	if got.Active != want.Active {
		t.Errorf("Active: got %v, want %v", got.Active, want.Active)
	}
	if string(got.Blob) != string(want.Blob) {
		t.Errorf("Blob: got %v, want %v", got.Blob, want.Blob)
	}
	if !got.Ref.Equal(want.Ref) {
		t.Errorf("Ref: got %v, want %v", got.Ref, want.Ref)
	}
	if got.Where != want.Where {
		t.Errorf("Where: got %v, want %v", got.Where, want.Where)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "a" || got.Tags[1] != "b" {
		t.Errorf("Tags: got %v", got.Tags)
	}
	if got.Nested != want.Nested {
		t.Errorf("Nested: got %v, want %v", got.Nested, want.Nested)
	}
	if got.Extra == nil || *got.Extra != *want.Extra {
		t.Errorf("Extra: got %v", got.Extra)
	}
	if got.Body != want.Body {
		t.Errorf("Body: got %q, want %q", got.Body, want.Body)
	}
	if text, ok := got.Raw.AsString(); !ok || text != "raw" {
		t.Errorf("Raw: got %v", got.Raw)
	}
}

// An int64 beyond float64 precision has to survive, which is the whole reason
// proto3 carries integerValue as text.
func TestLargeIntegerSurvivesExactly(t *testing.T) {
	r := sample()
	r.Count = math.MaxInt64
	entity := r.EncodeEntity()

	text, ok := entity.Properties["count"].AsNumber()
	if !ok {
		t.Fatalf("count is not an integer: %v", entity.Properties["count"])
	}
	if text != "9223372036854775807" {
		t.Errorf("count: got %q, want the exact decimal", text)
	}

	var got firestorefixture.Reading
	if err := got.DecodeEntity(entity); err != nil {
		t.Fatalf("DecodeEntity: %v", err)
	}
	if got.Count != math.MaxInt64 {
		t.Errorf("count: got %d, want MaxInt64", got.Count)
	}
}

// The key lives beside the properties, so the identity field must not also
// appear among them.
func TestIdentityIsNotAProperty(t *testing.T) {
	entity := sample().EncodeEntity()
	for _, name := range []string{"ID", "id", "-", "Parent", "Ver"} {
		if _, found := entity.Properties[name]; found {
			t.Errorf("property %q is present; identity belongs to the key", name)
		}
	}
	if entity.Key == nil {
		t.Fatal("no key on the encoded entity")
	}
	if entity.Key.Kind() != "Reading" {
		t.Errorf("kind: got %q, want Reading", entity.Key.Kind())
	}
	if len(entity.Key.Path) != 1 || entity.Key.Path[0].Name != "r-1" {
		t.Errorf("path: got %v", entity.Key.Path)
	}
}

func TestAncestorPath(t *testing.T) {
	r := sample()
	r.Parent = datastore.NameKey("Site", "s-1")
	key := r.EntityKey()

	if len(key.Path) != 2 {
		t.Fatalf("path: got %v, want two elements", key.Path)
	}
	if key.Path[0].Kind != "Site" || key.Path[0].Name != "s-1" {
		t.Errorf("ancestor: got %v", key.Path[0])
	}
	if key.Path[1].Kind != "Reading" || key.Path[1].Name != "r-1" {
		t.Errorf("leaf: got %v", key.Path[1])
	}

	// Decoding fills the ancestor back, so a value read by key carries the same
	// identity it was written with.
	var got firestorefixture.Reading
	if err := got.DecodeEntity(r.EncodeEntity()); err != nil {
		t.Fatalf("DecodeEntity: %v", err)
	}
	if !got.Parent.Equal(r.Parent) {
		t.Errorf("Parent: got %v, want %v", got.Parent, r.Parent)
	}
}

// omitempty makes a property absent, which is a different thing to a filter than
// a property set to null.
func TestOmitEmptyIsAbsentNotNull(t *testing.T) {
	r := sample()
	r.Tags = nil
	entity := r.EncodeEntity()
	if _, found := entity.Properties["tags"]; found {
		t.Error("tags is present; omitempty asks for absent")
	}

	r.Extra = nil
	entity = r.EncodeEntity()
	value, found := entity.Properties["extra"]
	if !found {
		t.Fatal("extra is absent; a nil pointer without omitempty is null, not missing")
	}
	if !value.IsNull() {
		t.Errorf("extra: got %v, want null", value)
	}
}

func TestNoIndexIsExcluded(t *testing.T) {
	entity := sample().EncodeEntity()
	if !entity.Properties["body"].ExcludeFromIndexes {
		t.Error("body is indexed; the noindex tag asks for the opposite")
	}
	if entity.Properties["note"].ExcludeFromIndexes {
		t.Error("note is excluded; only the tagged field should be")
	}
}

// Integer and Double are different types to Datastore, so a value stored as one
// must not decode into a field expecting the other.
func TestNumberKindsDoNotMerge(t *testing.T) {
	entity := sample().EncodeEntity()
	entity.Properties["scale"] = datastore.Int(2)

	var got firestorefixture.Reading
	err := got.DecodeEntity(entity)
	if err == nil {
		t.Fatal("an integer decoded into a float64 field; the two are different types")
	}
	fe, ok := firestorebind.AsError(err)
	if !ok {
		t.Fatalf("error is not a firestorebind Error: %v", err)
	}
	if fe.Property != "scale" || fe.Expected != "double" || fe.Got != "integer" {
		t.Errorf("error: got %+v, want scale/double/integer", fe)
	}
}

func TestVersionComesFromTheEntity(t *testing.T) {
	entity := sample().EncodeEntity()
	entity.Version = 42

	var got firestorefixture.Reading
	if err := got.DecodeEntity(entity); err != nil {
		t.Fatalf("DecodeEntity: %v", err)
	}
	if got.Ver != 42 {
		t.Errorf("Ver: got %d, want 42", got.Ver)
	}
	// It is never written back: the server assigns it.
	if _, found := entity.Properties["Ver"]; found {
		t.Error("Ver was written as a property")
	}
}

func TestEmptyStringIsStored(t *testing.T) {
	r := sample()
	r.Note = ""
	entity := r.EncodeEntity()
	value, found := entity.Properties["note"]
	if !found {
		t.Fatal("note is absent; an empty string is a value, not a missing property")
	}
	if text, ok := value.AsString(); !ok || text != "" {
		t.Errorf("note: got %v", value)
	}
}

func TestIDKeyedType(t *testing.T) {
	task := firestorefixture.Task{Number: 7, Title: "write it down"}
	key := task.EntityKey()
	if key.Kind() != "Task" || len(key.Path) != 1 || key.Path[0].ID != 7 {
		t.Fatalf("key: got %v", key)
	}

	var got firestorefixture.Task
	if err := got.DecodeEntity(task.EncodeEntity()); err != nil {
		t.Fatalf("DecodeEntity: %v", err)
	}
	if got.Number != 7 || got.Title != task.Title {
		t.Errorf("round trip: got %+v, want %+v", got, task)
	}
}

// A zero identity is an incomplete key, which is legal on insert and is where
// the server allocates.
func TestZeroIdentityIsIncomplete(t *testing.T) {
	if !(firestorefixture.Task{Title: "no id yet"}).EntityKey().Incomplete() {
		t.Error("a zero id produced a complete key")
	}
	if !(firestorefixture.Reading{}).EntityKey().Incomplete() {
		t.Error("an empty name produced a complete key")
	}
}
