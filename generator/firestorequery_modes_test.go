package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// firestoreResolverPackage is the Firestore half of resolverPackage: one
// framework struct under one key answering every value the framework manages.
const firestoreResolverPackage = `package pw

import (
	"context"

	"github.com/shibukawa/tinybind-go/firestorebind"
)

type Values struct {
	Datastore firestorebind.Handle
	Tenant    string
}

type key struct{}

func With(ctx context.Context, v *Values) context.Context {
	return context.WithValue(ctx, key{}, v)
}

func DatastoreHandle(ctx context.Context) (firestorebind.Handle, error) {
	v, ok := ctx.Value(key{}).(*Values)
	if !ok {
		return firestorebind.Handle{}, firestorebind.ErrNoClient
	}
	return v.Datastore, nil
}
`

// firestoreModeDecls covers every result shape, so each one's resolver-error
// return is generated and compiled rather than only the common two.
const firestoreModeDecls = `
export statement BySensor(sensor: Sensor): firestore.many<Reading> {
  where sensor == {sensor}
}

export statement PageBySensor(sensor: Sensor): firestore.batch<Reading> {
  where sensor == {sensor}
}

export statement CountBySensor(sensor: Sensor): firestore.count<Reading> {
  where sensor == {sensor}
}

export statement KeysBySensor(sensor: Sensor): firestore.keys<Reading> {
  where sensor == {sensor}
}
`

// generateFirestoreModes writes the queries with options and returns the source,
// having compiled the package so the emitted bodies are known to be well typed.
func generateFirestoreModes(t *testing.T, options generator.Options, extra map[string]string) string {
	t.Helper()
	dir := firestoreQueryModule(t, queryPackage(queryReading), firestoreModeDecls)
	for name, source := range extra {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g := generator.New(options)
	// The codec comes first: a declaration is the only use of Reading here, and
	// the query pass instantiates the runtime with it.
	if _, err := g.GenerateFirestoreEntities(dir, dir, ""); err != nil {
		t.Fatalf("generate entities: %v", err)
	}
	path, err := g.GenerateFirestoreQueries(dir, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, dir, generated)
	return string(generated)
}

func TestFirestoreParameterAPIPutsTheHandleInTheSignature(t *testing.T) {
	options := generator.DefaultOptions()
	options.FirestoreParameterAPI = true
	code := generateFirestoreModes(t, options, nil)

	for _, want := range []string{
		"func BySensor(ctx context.Context, h firestorebind.Handle, sensor Sensor, opts ...datastore.ReadOption) iter.Seq2[Reading, error]",
		"return firestorebind.QueryOn[Reading](ctx, h, q, opts...)",
		"return firestorebind.QueryPageOn[Reading](ctx, h, q, opts...)",
		"return firestorebind.CountOn(ctx, h, q, opts...)",
		"return firestorebind.QueryKeysPageOn(ctx, h, q, opts...)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q:\n%s", want, code)
		}
	}
	// The transactional twin takes a *Tx, which already carries the tenancy, so
	// no mode adds a Handle to it.
	if strings.Contains(code, "tx *firestorebind.Tx, h firestorebind.Handle") ||
		strings.Contains(code, "h firestorebind.Handle, tx *firestorebind.Tx") {
		t.Errorf("the transaction twin gained a Handle:\n%s", code)
	}
}

func TestFirestoreHandleResolverReadsTheFrameworkValue(t *testing.T) {
	options := generator.DefaultOptions()
	options.FirestoreHandleResolver = &generator.SymbolPattern{PackagePath: "tempmod/pw", Name: "DatastoreHandle"}
	code := generateFirestoreModes(t, options, map[string]string{"pw/pw.go": firestoreResolverPackage})

	for _, want := range []string{
		`_tinybindresolver "tempmod/pw"`,
		"func BySensor(ctx context.Context, sensor Sensor, opts ...datastore.ReadOption) iter.Seq2[Reading, error]",
		"h, err := _tinybindresolver.DatastoreHandle(ctx)",
		"return firestorebind.QueryOn[Reading](ctx, h, q, opts...)",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q:\n%s", want, code)
		}
	}
	// Each shape reports the resolver error in the shape it returns.
	for _, want := range []string{
		"yield(zero, err)",
		"return firestorebind.Page[Reading]{}, err",
		"return 0, err",
		"return firestorebind.KeyPage{}, err",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("a result shape drops the resolver error, missing %q:\n%s", want, code)
		}
	}
}

func TestFirestoreDefaultModeIsUnchanged(t *testing.T) {
	code := generateFirestoreModes(t, generator.DefaultOptions(), nil)

	for _, unwanted := range []string{"firestorebind.Handle", "QueryOn", "CountOn", "_tinybindresolver"} {
		if strings.Contains(code, unwanted) {
			t.Errorf("the default mode emitted %q:\n%s", unwanted, code)
		}
	}
	if !strings.Contains(code, "return firestorebind.Query[Reading](ctx, q, opts...)") {
		t.Errorf("the default mode changed:\n%s", code)
	}
}
