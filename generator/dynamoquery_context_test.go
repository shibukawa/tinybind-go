package generator_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// dynamoContextModule writes a one-type module with one declaration per result
// shape, so the generated code is compiled rather than only matched.
func dynamoContextModule(t *testing.T, extra map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module fixture\n\ngo 1.26\n\nrequire (\n\tgithub.com/shibukawa/tinybind-go v0.0.0\n\tgithub.com/shibukawa/tinygodriver v1.1.3\n)\n\nreplace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(moduleRoot) + "\n",
		"types.go": `package fixture

type Reading struct {
	Sensor string ` + "`dynamo:\"sensor,partitionkey\"`" + `
	At     int64  ` + "`dynamo:\"at,sortkey\"`" + `
	Note   string ` + "`dynamo:\"note\"`" + `
}
`,
		"readings.tb.dynamo": `export statement ReadingsSince(sensor: string, from: int64): dynamo.many<Reading> {
  table readings
  key sensor = {sensor} and at > {from}
}

export statement ReadingsPage(sensor: string): dynamo.page<Reading> {
  table readings
  key sensor = {sensor}
}
`,
	}
	for name, source := range extra {
		files[name] = source
	}
	for name, source := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tidyTempModule(t, dir)
	return dir
}

// compileGenerated builds the generated package, which is the only check that
// the emitted bodies are well typed.
func compileGenerated(t *testing.T, dir string, generated []byte) {
	t.Helper()
	command := exec.Command("go", "build", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated queries do not compile: %v\n%s\n%s", err, output, generated)
	}
}

func generateDynamoQueries(t *testing.T, dir string, options generator.Options) []byte {
	t.Helper()
	// The codec comes first, because a declaration is the only use of Reading
	// here: the query pass instantiates dynamobind.Query with it, and that does
	// not compile without the decoder the item pass writes.
	if _, err := generator.New(options).GenerateDynamoItems(dir, dir, ""); err != nil {
		t.Fatalf("generate items: %v", err)
	}
	path, err := generator.New(options).GenerateDynamoQueries(dir, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("no query file was generated")
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return generated
}

// TestDynamoQueriesTakeNeitherClientNorTable pins the whole point of the
// declaration: the table comes from the clause, the client from the Context,
// and the signature carries only what neither can supply.
func TestDynamoQueriesTakeNeitherClientNorTable(t *testing.T) {
	dir := dynamoContextModule(t, nil)
	generated := generateDynamoQueries(t, dir, generator.DefaultOptions())

	for _, want := range []string{
		`const readingsSinceTable = "readings"`,
		"func ReadingsSince(ctx context.Context, sensor string, from int64, opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]",
		"func ReadingsPage(ctx context.Context, sensor string, opts ...dynamodb.QueryOption) (dynamobind.Page[Reading], error)",
		"return dynamobind.Query[Reading](ctx, readingsSinceTable, readingsSinceKeyCondition, opts...)",
	} {
		if !bytes.Contains(generated, []byte(want)) {
			t.Errorf("missing %q:\n%s", want, generated)
		}
	}
	// There is one surface, so no suffixed variant and no client anywhere.
	for _, unwanted := range []string{"Context(ctx", "dynamodb.Client"} {
		if bytes.Contains(generated, []byte(unwanted)) {
			t.Errorf("generated code still mentions %q:\n%s", unwanted, generated)
		}
	}
	compileGenerated(t, dir, generated)
}
