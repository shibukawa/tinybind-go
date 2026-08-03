package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// firestoreQueryModule writes a temp module carrying the Go source and one
// declaration file, and returns its directory.
func firestoreQueryModule(t *testing.T, src, decls string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "queries.tb.firestore"), []byte(decls), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	return dir
}

// queryReading is the bound type the declarations below query.
const queryReading = "type Sensor string\n\n" +
	"type Reading struct {\n" +
	"\tID string `firestore:\"-,name\"`\n" +
	"\tParent datastore.Key `firestore:\"-,parent\"`\n" +
	"\tSensor Sensor `firestore:\"sensor\"`\n" +
	"\tAt time.Time `firestore:\"at\"`\n" +
	"\tCelsius float64 `firestore:\"celsius\"`\n" +
	"\tTags []string `firestore:\"tags\"`\n" +
	"\tBody string `firestore:\"body,noindex\"`\n" +
	"}"

// queryPackage composes a package that declares body and does nothing else. The
// declaration file is the only use, which is the case that proves a package
// whose sole Firestore use is a declaration still gets a codec.
func queryPackage(body string) string {
	return `package sample

import (
	"time"

	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

` + body + `

var _ = time.Now
var _ = datastore.String
`
}

func generateFirestoreQuery(t *testing.T, decls string) string {
	t.Helper()
	dir := firestoreQueryModule(t, queryPackage(queryReading), decls)
	g := &generator.Generator{Options: generator.DefaultOptions()}
	code, err := g.EmitFirestoreQueriesFor(dir)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return string(code)
}

func firestoreQueryErr(t *testing.T, decls string) error {
	t.Helper()
	dir := firestoreQueryModule(t, queryPackage(queryReading), decls)
	g := &generator.Generator{Options: generator.DefaultOptions()}
	_, err := g.EmitFirestoreQueriesFor(dir)
	return err
}

func TestFirestoreQueryEmitsOneFunctionPerDeclaration(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement BySensor(sensor: Sensor): firestore.many<Reading> {
  where sensor == {sensor}
}

export statement Since(from: time.Time): firestore.batch<Reading> {
  where at > {from}
}
`)
	for _, want := range []string{
		"func BySensor(ctx context.Context, sensor Sensor, opts ...datastore.ReadOption) iter.Seq2[Reading, error] {",
		"func Since(ctx context.Context, from time.Time, opts ...datastore.ReadOption) (firestorebind.Page[Reading], error) {",
		`q = q.Filter("sensor", datastore.Equal, datastore.String(string(sensor)))`,
		`q = q.Filter("at", datastore.GreaterThan, datastore.Time(from))`,
		"firestorebind.Query[Reading](ctx, q, opts...)",
		"firestorebind.QueryPage[Reading](ctx, q, opts...)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code is missing %q\n%s", want, code)
		}
	}
}

// The kind comes from the type, so a declaration names none and the signature
// carries neither a kind nor a client.
func TestFirestoreQueryNamesNoKind(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement BySensor(sensor: Sensor): firestore.many<Reading> {
  where sensor == {sensor}
}
`)
	if !strings.Contains(code, `const bySensorKind = "Reading"`) {
		t.Errorf("the kind constant is missing or wrong\n%s", code)
	}
	if strings.Contains(code, "kind string") || strings.Contains(code, "client") {
		t.Errorf("the signature carries a kind or a client\n%s", code)
	}
}

func TestFirestoreQueryShapes(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement CountAll(sensor: Sensor): firestore.count<Reading> {
  where sensor == {sensor}
}

export statement KeysOnly(sensor: Sensor): firestore.keys<Reading> {
  where sensor == {sensor}
}
`)
	for _, want := range []string{
		"func CountAll(ctx context.Context, sensor Sensor, opts ...datastore.ReadOption) (int64, error) {",
		"firestorebind.Count(ctx, q, opts...)",
		"func KeysOnly(ctx context.Context, sensor Sensor, opts ...datastore.ReadOption) (firestorebind.KeyPage, error) {",
		"q = q.KeysOnly()",
		"firestorebind.QueryKeysPage(ctx, q, opts...)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code is missing %q\n%s", want, code)
		}
	}
}

// in takes the candidates, so the parameter is a slice and the value is built
// through the same encoder the codec uses.
func TestFirestoreQueryInTakesASlice(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement AnyOf(sensors: []Sensor): firestore.batch<Reading> {
  where sensor in {sensors}
}
`)
	if !strings.Contains(code, "sensors []Sensor") {
		t.Errorf("the parameter is not a slice\n%s", code)
	}
	if !strings.Contains(code, "datastore.Array(") || !strings.Contains(code, "datastore.In") {
		t.Errorf("in is not built as an array filter\n%s", code)
	}
}

func TestFirestoreQueryAncestorOrderAndBounds(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement Under(parent: datastore.Key, size: int): firestore.batch<Reading> {
  ancestor {parent}
  order at desc, celsius
  limit {size}
  offset 10
}
`)
	for _, want := range []string{
		"q = q.Ancestor(parent)",
		`q = q.OrderDesc("at")`,
		`q = q.Order("celsius")`,
		"q = q.Limit(int32(size))",
		"q = q.Offset(10)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code is missing %q\n%s", want, code)
		}
	}
}

// An index is declared, never derived, and it is exported when its statement is,
// because a deploy step has to reach it.
func TestFirestoreQueryIndexIsDeclaredAndExported(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement Warm(sensor: Sensor, from: time.Time): firestore.many<Reading> {
  where sensor == {sensor} and at > {from}
  order at desc
  index sensor, at desc
}
`)
	for _, want := range []string{
		"var WarmIndex = datastore.Index{",
		"Kind: warmKind,",
		`{Name: "sensor", Direction: datastore.Ascending},`,
		`{Name: "at", Direction: datastore.Descending},`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code is missing %q\n%s", want, code)
		}
	}
	// A declared index replaces the guess, so the hint is not also emitted.
	if strings.Contains(code, "may require a") {
		t.Errorf("a declared index still got the may-require hint\n%s", code)
	}
}

// Where no index is declared and the shape commonly needs one, the godoc says
// so, and says only "may": deriving the actual index is declined.
func TestFirestoreQueryHintsWithoutDeriving(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement Warm(sensor: Sensor, from: time.Time): firestore.many<Reading> {
  where sensor == {sensor} and at > {from}
  order at desc
}
`)
	if !strings.Contains(code, "may require a") {
		t.Errorf("a multi-property query got no index hint\n%s", code)
	}
	if strings.Contains(code, "datastore.Index{") {
		t.Errorf("an index was derived; it is only ever declared\n%s", code)
	}
}

func TestFirestoreQueryErrors(t *testing.T) {
	tests := []struct {
		name  string
		decls string
		want  string
	}{
		{
			name:  "renamed property",
			decls: "export statement A(s: Sensor): firestore.many<Reading> {\n where sensr == {s}\n}",
			want:  `Reading has no property "sensr"`,
		},
		{
			name:  "wrong parameter type",
			decls: "export statement A(s: string): firestore.many<Reading> {\n where sensor == {s}\n}",
			want:  "parameter s is string, but property sensor is stored from Sensor",
		},
		{
			name:  "in without a slice",
			decls: "export statement A(s: Sensor): firestore.many<Reading> {\n where sensor in {s}\n}",
			want:  "takes a slice of what property sensor stores, which is []Sensor",
		},
		{
			name:  "filter on the identity",
			decls: "export statement A(s: string): firestore.many<Reading> {\n where ID == {s}\n}",
			want:  "carries the key of Reading rather than a property",
		},
		{
			name:  "filter on an unindexed property",
			decls: "export statement A(s: string): firestore.many<Reading> {\n where body == {s}\n}",
			want:  "is tagged noindex",
		},
		{
			name:  "unknown parameter",
			decls: "export statement A(s: Sensor): firestore.many<Reading> {\n where sensor == {other}\n}",
			want:  "no parameter named other is declared",
		},
		{
			name:  "unused parameter",
			decls: "export statement A(s: Sensor, extra: int): firestore.many<Reading> {\n where sensor == {s}\n}",
			want:  "parameter extra is declared but never used",
		},
		{
			name:  "or is not on the wire",
			decls: "export statement A(s: Sensor): firestore.many<Reading> {\n where sensor == {s} or sensor == {s}\n}",
			want:  "there is no OR on this wire",
		},
		{
			name:  "single equals",
			decls: "export statement A(s: Sensor): firestore.many<Reading> {\n where sensor = {s}\n}",
			want:  `equality is spelled "==" here`,
		},
		{
			name:  "export disagrees with the name",
			decls: "statement A(s: Sensor): firestore.many<Reading> {\n where sensor == {s}\n}",
			want:  "has an exported name",
		},
		{
			name:  "unknown type",
			decls: "export statement A(s: Sensor): firestore.many<Missing> {\n where sensor == {s}\n}",
			want:  "no type Missing in this package carries firestore tags",
		},
		{
			name:  "ancestor is not a key",
			decls: "export statement A(p: string): firestore.many<Reading> {\n ancestor {p}\n}",
			want:  "must be datastore.Key",
		},
		{
			name:  "limit is not an integer",
			decls: "export statement A(s: Sensor, n: string): firestore.many<Reading> {\n where sensor == {s}\n limit {n}\n}",
			want:  "must be an integer",
		},
		{
			name: "duplicate statement",
			decls: "export statement A(s: Sensor): firestore.many<Reading> {\n where sensor == {s}\n}\n" +
				"export statement A(s: Sensor): firestore.many<Reading> {\n where sensor == {s}\n}",
			want: "is already declared",
		},
		{
			name:  "index names a renamed property",
			decls: "export statement A(s: Sensor): firestore.many<Reading> {\n where sensor == {s}\n index sensr\n}",
			want:  `Reading has no property "sensr"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := firestoreQueryErr(t, test.decls)
			if err == nil {
				t.Fatalf("got no error, want one containing %q", test.want)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %q does not contain %q", err, test.want)
			}
		})
	}
}

// A declaration is a use of its result type, so a package whose only Firestore
// use is a .tb.firestore file still gets a codec to decode into.
func TestFirestoreDeclarationCountsAsUsage(t *testing.T) {
	dir := firestoreQueryModule(t, queryPackage(queryReading), `
export statement BySensor(sensor: Sensor): firestore.many<Reading> {
  where sensor == {sensor}
}
`)
	opts := generator.DefaultOptions()
	plan, err := generator.AnalyzeFirestoreEntitiesWithOptions(dir, opts)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(plan.Entities) == 0 {
		t.Fatal("a package whose only use is a declaration got no codec")
	}
	code, err := generator.EmitFirestoreEntities(plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if !strings.Contains(string(code), "func (v *Reading) DecodeEntity(") {
		t.Errorf("the declared result type got no decoder\n%s", code)
	}
}

func TestFirestoreQueryGenerationIsDeterministic(t *testing.T) {
	decls := `
export statement BySensor(sensor: Sensor): firestore.many<Reading> {
  where sensor == {sensor}
}
`
	dir := firestoreQueryModule(t, queryPackage(queryReading), decls)
	g := &generator.Generator{Options: generator.DefaultOptions()}

	var first string
	for i := range 3 {
		code, err := g.EmitFirestoreQueriesFor(dir)
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

// Every shape a transaction can serve gets a twin, emitted from the declaration
// rather than from a discovered call, for the reason the key builder is.
func TestFirestoreQueryTransactionTwin(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement Page(sensor: Sensor): firestore.batch<Reading> {
  where sensor == {sensor}
}

export statement Total(sensor: Sensor): firestore.count<Reading> {
  where sensor == {sensor}
}

export statement Ids(sensor: Sensor): firestore.keys<Reading> {
  where sensor == {sensor}
}
`)
	for _, want := range []string{
		"func PageTx(ctx context.Context, tx *firestorebind.Tx, sensor Sensor) (firestorebind.Page[Reading], error) {",
		"firestorebind.QueryPageTx[Reading](ctx, tx, q)",
		"func TotalTx(ctx context.Context, tx *firestorebind.Tx, sensor Sensor) (int64, error) {",
		"firestorebind.CountTx(ctx, tx, q)",
		"func IdsTx(ctx context.Context, tx *firestorebind.Tx, sensor Sensor) (firestorebind.KeyPage, error) {",
		"firestorebind.QueryKeysPageTx(ctx, tx, q)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code is missing %q\n%s", want, code)
		}
	}
}

// The iterator gets no twin: a range inside a transaction issues an unbounded
// number of round trips inside something that has to commit.
func TestFirestoreQueryIteratorHasNoTransactionTwin(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement Every(sensor: Sensor): firestore.many<Reading> {
  where sensor == {sensor}
}
`)
	if strings.Contains(code, "func EveryTx(") {
		t.Errorf("the iterator shape got a transaction twin\n%s", code)
	}
}

// A twin takes the declaration's own visibility, since an unexported statement
// has no caller outside the package to reach either form.
func TestFirestoreQueryTwinFollowsVisibility(t *testing.T) {
	code := generateFirestoreQuery(t, `
statement page(sensor: Sensor): firestore.batch<Reading> {
  where sensor == {sensor}
}
`)
	if !strings.Contains(code, "func pageTx(ctx context.Context,") {
		t.Errorf("an unexported statement got an exported twin\n%s", code)
	}
}

// Both forms build the query through one emitter, so they cannot drift apart.
func TestFirestoreQueryFormsAgree(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement Page(sensor: Sensor, n: int): firestore.batch<Reading> {
  where sensor == {sensor}
  order at desc
  limit {n}
}
`)
	// The filter, the ordering and the bound each appear once per form.
	for _, line := range []string{
		`q = q.Filter("sensor", datastore.Equal, datastore.String(string(sensor)))`,
		`q = q.OrderDesc("at")`,
		"q = q.Limit(int32(n))",
	} {
		if got := strings.Count(code, line); got != 2 {
			t.Errorf("%q appears %d times, want 2 (one per form)\n%s", line, got, code)
		}
	}
}

// A projection is bandwidth, not a different type: the result stays the bound
// type and what was not selected arrives as its zero value.
func TestFirestoreQueryProjection(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement Times(sensor: Sensor): firestore.batch<Reading> {
  where sensor == {sensor}
  select at, celsius
}
`)
	if !strings.Contains(code, `q = q.Project("at", "celsius")`) {
		t.Errorf("the projection is missing\n%s", code)
	}
	if !strings.Contains(code, "(firestorebind.Page[Reading], error)") {
		t.Errorf("a projection changed the result type\n%s", code)
	}
	// The hazard is writing one back: Store and Update replace the whole
	// entity, so the unselected zero values would erase real data.
	if !strings.Contains(code, "Do not write one back") {
		t.Errorf("the write-back hazard is not documented\n%s", code)
	}
}

// A projection on an array property returns one result per element rather than
// one per entity, which the generator can see from the field's own type.
func TestFirestoreQueryProjectionOnArrayIsCalledOut(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement Tagged(s: Sensor): firestore.batch<Reading> {
  where sensor == {s}
  select tags
}
`)
	if !strings.Contains(code, "one result per") {
		t.Errorf("the array multiplication is not called out\n%s", code)
	}

	// A projection on a scalar says nothing about elements, so the note is not
	// emitted where it would be wrong.
	scalar := generateFirestoreQuery(t, `
export statement Times(s: Sensor): firestore.batch<Reading> {
  where sensor == {s}
  select at
}
`)
	if strings.Contains(scalar, "one result per") {
		t.Errorf("a scalar projection got the array note\n%s", scalar)
	}
}

// Cursors are how a caller resumes, which an offset cannot do without paying
// for every entity it steps over.
func TestFirestoreQueryCursors(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement From(c: datastore.Cursor, e: datastore.Cursor): firestore.batch<Reading> {
  order at
  start {c}
  end {e}
}
`)
	for _, want := range []string{
		"c datastore.Cursor, e datastore.Cursor",
		"q = q.Start(c)",
		"q = q.End(e)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code is missing %q\n%s", want, code)
		}
	}
}

func TestFirestoreQueryProjectionErrors(t *testing.T) {
	tests := []struct {
		name  string
		decls string
		want  string
	}{
		{
			name:  "select a renamed property",
			decls: "export statement A(s: Sensor): firestore.batch<Reading> {\n where sensor == {s}\n select att\n}",
			want:  `Reading has no property "att"`,
		},
		{
			// A projection reads from an index, and an excluded property is in
			// none, so it can never come back.
			name:  "select an unindexed property",
			decls: "export statement A(s: Sensor): firestore.batch<Reading> {\n where sensor == {s}\n select body\n}",
			want:  "is tagged noindex",
		},
		{
			name:  "select on a count",
			decls: "export statement A(s: Sensor): firestore.count<Reading> {\n where sensor == {s}\n select at\n}",
			want:  "counts, so a select clause has nothing to return",
		},
		{
			name:  "select on a keys query",
			decls: "export statement A(s: Sensor): firestore.keys<Reading> {\n where sensor == {s}\n select at\n}",
			want:  "already a projection on the key",
		},
		{
			name:  "distinct does not lead the ordering",
			decls: "export statement A(s: Sensor): firestore.batch<Reading> {\n where sensor == {s}\n distinct celsius\n order at, celsius\n}",
			want:  "have to lead it in the same order",
		},
		{
			name:  "distinct names more than the ordering",
			decls: "export statement A(s: Sensor): firestore.batch<Reading> {\n where sensor == {s}\n distinct celsius, at\n order celsius\n}",
			want:  "have to lead the ordering",
		},
		{
			name:  "cursor is not a Cursor",
			decls: "export statement A(c: string): firestore.batch<Reading> {\n order at\n start {c}\n}",
			want:  "must be datastore.Cursor",
		},
		{
			name:  "cursor on a count",
			decls: "export statement A(c: datastore.Cursor): firestore.count<Reading> {\n start {c}\n}",
			want:  "counts, so there is no batch to resume",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := firestoreQueryErr(t, test.decls)
			if err == nil {
				t.Fatalf("got no error, want one containing %q", test.want)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %q does not contain %q", err, test.want)
			}
		})
	}
}

// distinct leading the ordering is accepted, which is the rule's other half.
func TestFirestoreQueryDistinctLeadingTheOrdering(t *testing.T) {
	code := generateFirestoreQuery(t, `
export statement A(s: Sensor): firestore.batch<Reading> {
  where sensor == {s}
  distinct celsius
  order celsius, at
}
`)
	if !strings.Contains(code, `q = q.DistinctOn("celsius")`) {
		t.Errorf("the distinct clause is missing\n%s", code)
	}
}
