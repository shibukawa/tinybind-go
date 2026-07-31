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
// shape, so a generated Context wrapper is compiled rather than only matched.
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
// the wrapper bodies are well typed. The iterator form in particular returns
// its resolver error through a yield rather than a result.
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

// TestDynamoQueriesTakeNoTableParameter pins the table clause: the declaration
// names the table, so the generated signature does not.
func TestDynamoQueriesTakeNoTableParameter(t *testing.T) {
	dir := dynamoContextModule(t, nil)
	generated := generateDynamoQueries(t, dir, generator.DefaultOptions())

	if !bytes.Contains(generated, []byte(`const readingsSinceTable = "readings"`)) {
		t.Errorf("the declared table is not a constant:\n%s", generated)
	}
	if !bytes.Contains(generated, []byte("func ReadingsSince(ctx context.Context, c *dynamodb.Client, sensor string, from int64, opts ...dynamodb.QueryOption)")) {
		t.Errorf("the explicit signature still carries a table:\n%s", generated)
	}
	// Without the option, the Context surface is absent entirely.
	if bytes.Contains(generated, []byte("ReadingsSinceContext")) {
		t.Errorf("the Context API is opt-in, but a wrapper was generated:\n%s", generated)
	}
	compileGenerated(t, dir, generated)
}

func TestDynamoContextAPIGeneratesWrappers(t *testing.T) {
	dir := dynamoContextModule(t, nil)
	options := generator.DefaultOptions()
	options.DynamoContextAPI = true
	generated := generateDynamoQueries(t, dir, options)

	// Both surfaces exist, and the explicit one is unchanged.
	for _, want := range []string{
		"func ReadingsSince(ctx context.Context, c *dynamodb.Client, sensor string, from int64, opts ...dynamodb.QueryOption)",
		"func ReadingsSinceContext(ctx context.Context, sensor string, from int64, opts ...dynamodb.QueryOption)",
		"func ReadingsPageContext(ctx context.Context, sensor string, opts ...dynamodb.QueryOption)",
		"dynamobind.TableFromContext(ctx, readingsSinceTable)",
	} {
		if !bytes.Contains(generated, []byte(want)) {
			t.Errorf("missing %q:\n%s", want, generated)
		}
	}
	compileGenerated(t, dir, generated)
}

func TestDynamoContextOnlyAPIRenamesTheExplicitForm(t *testing.T) {
	dir := dynamoContextModule(t, nil)
	options := generator.DefaultOptions()
	options.DynamoContextOnlyAPI = true
	generated := generateDynamoQueries(t, dir, options)

	if !bytes.Contains(generated, []byte("func ReadingsSince(ctx context.Context, sensor string, from int64, opts ...dynamodb.QueryOption)")) {
		t.Errorf("the declared name does not carry the Context form:\n%s", generated)
	}
	if !bytes.Contains(generated, []byte("func _tinybindDynamoReadingsSince(ctx context.Context, c *dynamodb.Client,")) {
		t.Errorf("the client-taking form was not made unexported:\n%s", generated)
	}
	// The name the wrapper would have taken stays free in this mode.
	if bytes.Contains(generated, []byte("ReadingsSinceContext")) {
		t.Errorf("context-only mode still generated a Context suffix:\n%s", generated)
	}
	compileGenerated(t, dir, generated)
}

func TestDynamoClientResolverReplacesTheRuntime(t *testing.T) {
	resolver := `package dynactx

import (
	"context"

	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// Table maps a declared table name onto the one this deployment uses.
func Table(context.Context, string) (*dynamodb.Client, string, error) { return nil, "", nil }
`
	dir := dynamoContextModule(t, map[string]string{"dynactx/dynactx.go": resolver})
	options := generator.DefaultOptions()
	options.DynamoClientResolver = &generator.SymbolPattern{PackagePath: "fixture/dynactx", Name: "Table"}
	generated := generateDynamoQueries(t, dir, options)

	if !bytes.Contains(generated, []byte(`_tinybindresolver "fixture/dynactx"`)) {
		t.Errorf("the resolver package was not imported:\n%s", generated)
	}
	if !bytes.Contains(generated, []byte("_tinybindresolver.Table(ctx, readingsSinceTable)")) {
		t.Errorf("the framework resolver was not called:\n%s", generated)
	}
	if bytes.Contains(generated, []byte("dynamobind.TableFromContext")) {
		t.Errorf("the runtime resolver survived a configured one:\n%s", generated)
	}
	compileGenerated(t, dir, generated)
}

// TestDynamoContextNameCollision covers the one name the Context mode can take
// away from an author: a declaration already called <Other>Context.
func TestDynamoContextNameCollision(t *testing.T) {
	declarations := `export statement Readings(sensor: string): dynamo.many<Reading> {
  table readings
  key sensor = {sensor}
}

export statement ReadingsContext(sensor: string): dynamo.many<Reading> {
  table readings
  key sensor = {sensor}
}
`
	dir := dynamoContextModule(t, map[string]string{"readings.tb.dynamo": declarations})
	options := generator.DefaultOptions()
	options.DynamoContextAPI = true
	if _, err := generator.New(options).GenerateDynamoQueries(dir, dir, ""); err == nil {
		t.Fatal("expected a name collision error")
	} else if !bytes.Contains([]byte(err.Error()), []byte("already a declared statement")) {
		t.Fatalf("error does not name the collision: %v", err)
	}
}
