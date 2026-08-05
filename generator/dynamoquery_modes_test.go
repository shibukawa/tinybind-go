package generator_test

import (
	"bytes"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// resolverPackage is a framework that carries every value it manages in one
// struct under one key, which is the shape the resolver option exists for: one
// lookup and one type assertion answer the DynamoDB Handle and everything else
// the framework holds.
const resolverPackage = `package pw

import (
	"context"

	"github.com/shibukawa/tinybind-go/dynamobind"
)

type Values struct {
	Dynamo dynamobind.Handle
	Tenant string
}

type key struct{}

func With(ctx context.Context, v *Values) context.Context {
	return context.WithValue(ctx, key{}, v)
}

func DynamoHandle(ctx context.Context) (dynamobind.Handle, error) {
	v, ok := ctx.Value(key{}).(*Values)
	if !ok {
		return dynamobind.Handle{}, dynamobind.ErrNoClient
	}
	return v.Dynamo, nil
}
`

// TestDynamoParameterAPIPutsTheHandleInTheSignature checks the mode the request
// asked for: the declared name is unchanged and only the signature moves.
func TestDynamoParameterAPIPutsTheHandleInTheSignature(t *testing.T) {
	dir := dynamoContextModule(t, nil)
	options := generator.DefaultOptions()
	options.DynamoParameterAPI = true
	generated := generateDynamoQueries(t, dir, options)

	for _, want := range []string{
		"func ReadingsSince(ctx context.Context, h dynamobind.Handle, sensor string, from int64, opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]",
		"func ReadingsPage(ctx context.Context, h dynamobind.Handle, sensor string, opts ...dynamodb.QueryOption) (dynamobind.Page[Reading], error)",
		"return dynamobind.QueryOn[Reading](ctx, h, readingsSinceTable, readingsSinceKeyCondition, opts...)",
		"return dynamobind.QueryPageOn[Reading](ctx, h, readingsPageTable, readingsPageKeyCondition, opts...)",
	} {
		if !bytes.Contains(generated, []byte(want)) {
			t.Errorf("missing %q:\n%s", want, generated)
		}
	}
	// The declaration still owns the table, and the mode changes the signature
	// rather than the name.
	if bytes.Contains(generated, []byte("ReadingsSinceOn(")) {
		t.Errorf("the declared name gained a suffix:\n%s", generated)
	}
	compileGenerated(t, dir, generated)
}

// TestDynamoHandleResolverReadsTheFrameworkValue checks the other half: the
// signature is the Context one, and the Handle comes from the framework's own
// Context value rather than from a second node dynamobind installs.
func TestDynamoHandleResolverReadsTheFrameworkValue(t *testing.T) {
	dir := dynamoContextModule(t, map[string]string{"pw/pw.go": resolverPackage})
	options := generator.DefaultOptions()
	options.DynamoHandleResolver = &generator.SymbolPattern{PackagePath: "fixture/pw", Name: "DynamoHandle"}
	generated := generateDynamoQueries(t, dir, options)

	for _, want := range []string{
		`_tinybindresolver "fixture/pw"`,
		"func ReadingsSince(ctx context.Context, sensor string, from int64, opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]",
		"h, err := _tinybindresolver.DynamoHandle(ctx)",
		"return dynamobind.QueryOn[Reading](ctx, h, readingsSinceTable, readingsSinceKeyCondition, opts...)",
	} {
		if !bytes.Contains(generated, []byte(want)) {
			t.Errorf("missing %q:\n%s", want, generated)
		}
	}
	// The resolver's error has to be reported in the shape the declaration
	// returns, which for an iterator means yielding it once.
	if !bytes.Contains(generated, []byte("yield(zero, err)")) {
		t.Errorf("the iterator form drops the resolver error:\n%s", generated)
	}
	if !bytes.Contains(generated, []byte("return dynamobind.Page[Reading]{}, err")) {
		t.Errorf("the page form drops the resolver error:\n%s", generated)
	}
	compileGenerated(t, dir, generated)
}

// TestDynamoParameterAPIWinsOverTheResolver pins the precedence: a signature
// already carrying the Handle resolves nothing, so no resolver import is
// emitted for a package that would never call it.
func TestDynamoParameterAPIWinsOverTheResolver(t *testing.T) {
	dir := dynamoContextModule(t, map[string]string{"pw/pw.go": resolverPackage})
	options := generator.DefaultOptions()
	options.DynamoParameterAPI = true
	options.DynamoHandleResolver = &generator.SymbolPattern{PackagePath: "fixture/pw", Name: "DynamoHandle"}
	generated := generateDynamoQueries(t, dir, options)

	if bytes.Contains(generated, []byte("_tinybindresolver")) {
		t.Errorf("parameter mode still imports the resolver:\n%s", generated)
	}
	if !bytes.Contains(generated, []byte("h dynamobind.Handle")) {
		t.Errorf("parameter mode did not take effect:\n%s", generated)
	}
	compileGenerated(t, dir, generated)
}

// TestDynamoDefaultModeIsUnchanged is the acceptance condition the whole change
// rests on: a run setting neither option emits what it emitted before either
// existed.
func TestDynamoDefaultModeIsUnchanged(t *testing.T) {
	dir := dynamoContextModule(t, nil)
	generated := generateDynamoQueries(t, dir, generator.DefaultOptions())

	for _, unwanted := range []string{"dynamobind.Handle", "QueryOn", "QueryPageOn", "_tinybindresolver"} {
		if bytes.Contains(generated, []byte(unwanted)) {
			t.Errorf("the default mode emitted %q:\n%s", unwanted, generated)
		}
	}
	if !bytes.Contains(generated, []byte("return dynamobind.Query[Reading](ctx, readingsSinceTable, readingsSinceKeyCondition, opts...)")) {
		t.Errorf("the default mode changed:\n%s", generated)
	}
}

// TestDynamoHandleResolverIsChecked keeps a mistyped resolver a generation
// error rather than an unbuildable generated file whose cause is one setting in
// a config the reader is not looking at.
func TestDynamoHandleResolverIsChecked(t *testing.T) {
	for name, resolver := range map[string]generator.SymbolPattern{
		"no package":  {Name: "DynamoHandle"},
		"unexported":  {PackagePath: "fixture/pw", Name: "dynamoHandle"},
		"not a name":  {PackagePath: "fixture/pw", Name: "Dynamo Handle"},
		"empty name":  {PackagePath: "fixture/pw", Name: ""},
		"punctuation": {PackagePath: "fixture/pw", Name: "pw.Handle"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := dynamoContextModule(t, nil)
			options := generator.DefaultOptions()
			options.DynamoHandleResolver = &resolver
			if _, err := generator.New(options).GenerateDynamoItems(dir, dir, ""); err != nil {
				t.Fatalf("generate items: %v", err)
			}
			if _, err := generator.New(options).GenerateDynamoQueries(dir, dir, ""); err == nil {
				t.Fatal("a resolver that cannot be called generated without an error")
			}
		})
	}
}
