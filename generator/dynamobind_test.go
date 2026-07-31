package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// dynamoModule writes a temp module whose single source is src, and returns its
// directory.
func dynamoModule(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	return dir
}

func generateDynamo(t *testing.T, src string) string {
	t.Helper()
	dir := dynamoModule(t, src)
	plan, err := generator.AnalyzeDynamoItemsWithOptions(dir, generator.DefaultOptions())
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	code, err := generator.EmitDynamoItems(plan, true)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return string(code)
}

// dynamoSource composes a package that declares body and calls the dynamobind
// functions named in calls, so discovery has something to find.
func dynamoSource(body, calls string) string {
	return `package sample

import (
	"context"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

` + body + `

var _ = context.Background
var _ = dynamodb.S

func use(ctx context.Context, c *dynamodb.Client) {
` + calls + `
}
`
}

func TestDynamoUsageFollowsCallSites(t *testing.T) {
	tests := []struct {
		name    string
		call    string
		want    []string
		notWant []string
	}{
		{
			name:    "load emits the decoder, not the encoder",
			call:    `	_, _ = dynamobind.Load[Reading](ctx, c, "t", dynamodb.Key{})`,
			want:    []string{"func (v *Reading) DecodeItem("},
			notWant: []string{"EncodeItem"},
		},
		{
			name:    "store emits the encoder, not the decoder",
			call:    `	_ = dynamobind.Store(ctx, c, "t", Reading{})`,
			want:    []string{"func (v Reading) EncodeItem("},
			notWant: []string{"DecodeItem"},
		},
		{
			name:    "remove emits the key and its table",
			call:    `	_ = dynamobind.Remove(ctx, c, "t", Reading{})`,
			want:    []string{"func (v Reading) ItemKey(", "func ReadingTable("},
			notWant: []string{"EncodeItem", "DecodeItem"},
		},
		{
			name: "store returning emits both codecs",
			call: `	_, _, _ = dynamobind.StoreReturning(ctx, c, "t", Reading{})`,
			want: []string{"func (v Reading) EncodeItem(", "func (v *Reading) DecodeItem("},
		},
		{
			name: "storeall discovers the slice element",
			call: `	_, _ = dynamobind.StoreAll(ctx, c, "t", []Reading{})`,
			want: []string{"func (v Reading) EncodeItem("},
		},
	}
	body := "type Reading struct {\n\tSensor string `dynamo:\"sensor,partitionkey\"`\n\tAt int64 `dynamo:\"at\"`\n}"
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code := generateDynamo(t, dynamoSource(body, test.call))
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

// TestDynamoUsageReachesNestedTypes proves the closure: a nested struct gains
// the operations its parent needs, and never a table key of its own.
func TestDynamoUsageReachesNestedTypes(t *testing.T) {
	body := "type Profile struct {\n\tCity string `dynamo:\"city\"`\n}\n\n" +
		"type Reading struct {\n\tSensor string `dynamo:\"sensor,partitionkey\"`\n\tProfile Profile `dynamo:\"profile\"`\n}"
	code := generateDynamo(t, dynamoSource(body, `	_ = dynamobind.Store(ctx, c, "t", Reading{})`))
	for _, want := range []string{"func (v Reading) EncodeItem(", "func (v Profile) EncodeItem("} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q in:\n%s", want, code)
		}
	}
	for _, notWant := range []string{"func (v Profile) ItemKey(", "func ProfileTable(", "func (v *Profile) DecodeItem("} {
		if strings.Contains(code, notWant) {
			t.Errorf("nested type over-generated %q in:\n%s", notWant, code)
		}
	}
}

// TestDynamoKeyBuilderComesWithTheKeyTag pins the one operation that is not
// usage-directed. Load(ctx, c, table, v.ItemKey()) is the documented way to
// read an item, and using a method is not a discoverable call, so waiting for
// one would mean the method never existed to call.
func TestDynamoKeyBuilderComesWithTheKeyTag(t *testing.T) {
	body := "type Reading struct {\n\tSensor string `dynamo:\"sensor,partitionkey\"`\n}"
	code := generateDynamo(t, dynamoSource(body, `	_, _ = dynamobind.Load[Reading](ctx, c, "t", dynamodb.Key{})`))
	if !strings.Contains(code, "func (v Reading) ItemKey(") {
		t.Fatalf("a bound type with a partitionkey must get its key builder:\n%s", code)
	}
}

// TestDynamoNoKeyTagNoKeyBuilder is the other half: without a key tag there is
// no key to build, and no table to describe.
func TestDynamoNoKeyTagNoKeyBuilder(t *testing.T) {
	body := "type Reading struct {\n\tSensor string `dynamo:\"sensor\"`\n}"
	code := generateDynamo(t, dynamoSource(body, `	_ = dynamobind.Store(ctx, c, "t", Reading{})`))
	for _, notWant := range []string{"ItemKey", "ReadingTable", "dynamobind.Keyer"} {
		if strings.Contains(code, notWant) {
			t.Fatalf("unexpected %q without a key tag:\n%s", notWant, code)
		}
	}
}

func TestDynamoUnusedTypeEmitsNothing(t *testing.T) {
	body := "type Reading struct {\n\tSensor string `dynamo:\"sensor,partitionkey\"`\n}\n\n" +
		"type Unrelated struct {\n\tID string `dynamo:\"id\"`\n}"
	code := generateDynamo(t, dynamoSource(body, `	_ = dynamobind.Store(ctx, c, "t", Reading{})`))
	if strings.Contains(code, "Unrelated") {
		t.Fatalf("unused type emitted:\n%s", code)
	}
}

// TestDynamoGeneratedCodeHasNoReflect guards the property the whole generator
// exists for.
func TestDynamoGeneratedCodeHasNoReflect(t *testing.T) {
	body := "type Reading struct {\n\tSensor string `dynamo:\"sensor,partitionkey\"`\n\tTags []string `dynamo:\"tags,stringset\"`\n}"
	code := generateDynamo(t, dynamoSource(body, `	_ = dynamobind.Store(ctx, c, "t", Reading{})`))
	if strings.Contains(code, "reflect") {
		t.Fatalf("generated code references reflect:\n%s", code)
	}
}

func TestDynamoTableDefinitionCarriesKeyTypes(t *testing.T) {
	body := "type Reading struct {\n\tSensor []byte `dynamo:\"sensor,partitionkey\"`\n\tAt float64 `dynamo:\"at,sortkey\"`\n}"
	code := generateDynamo(t, dynamoSource(body, `	_ = dynamobind.Remove(ctx, c, "t", Reading{})`))
	for _, want := range []string{
		`PartitionKey: dynamodb.KeyAttribute{Name: "sensor", Type: dynamodb.TypeBinary}`,
		`SortKey:      &dynamodb.KeyAttribute{Name: "at", Type: dynamodb.TypeNumber}`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q in:\n%s", want, code)
		}
	}
}

func TestDynamoTableDefinitionIsDisableable(t *testing.T) {
	body := "type Reading struct {\n\tSensor string `dynamo:\"sensor,partitionkey\"`\n}"
	dir := dynamoModule(t, dynamoSource(body, `	_ = dynamobind.Remove(ctx, c, "t", Reading{})`))
	options := generator.DefaultOptions()
	options.DisableFeatures = []generator.Feature{generator.FeatureItemTable}
	plan, err := generator.AnalyzeDynamoItemsWithOptions(dir, options)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.EmitDynamoItems(plan, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(code), "ReadingTable") {
		t.Fatalf("disabled table definition emitted:\n%s", code)
	}
	if !strings.Contains(string(code), "func (v Reading) ItemKey(") {
		t.Fatalf("key builder should survive a disabled table definition:\n%s", code)
	}
}

func TestDynamoCodecDisableEmitsNothing(t *testing.T) {
	body := "type Reading struct {\n\tSensor string `dynamo:\"sensor,partitionkey\"`\n}"
	dir := dynamoModule(t, dynamoSource(body, `	_ = dynamobind.Store(ctx, c, "t", Reading{})`))
	options := generator.DefaultOptions()
	options.DisableFeatures = []generator.Feature{generator.FeatureItemCodec}
	plan, err := generator.AnalyzeDynamoItemsWithOptions(dir, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 0 {
		t.Fatalf("disabled codec still planned %d item(s)", len(plan.Items))
	}
}

// TestDynamoGenerationErrors covers every check rule:dynamo-tag-options states.
// Each one must name the field, since the message is the whole point of failing
// at generation time rather than storing something surprising.
func TestDynamoGenerationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknown option",
			body: "type Reading struct {\n\tSensor string `dynamo:\"sensor,partitionkey\"`\n\tOops string `dynamo:\"oops,stringset2\"`\n}",
			want: `unknown dynamo tag option "stringset2"`,
		},
		{
			name: "duplicate attribute",
			body: "type Reading struct {\n\tSensor string `dynamo:\"sensor,partitionkey\"`\n\tOther string `dynamo:\"sensor\"`\n}",
			want: `both map to attribute "sensor"`,
		},
		{
			name: "two partition keys",
			body: "type Reading struct {\n\tA string `dynamo:\"a,partitionkey\"`\n\tB string `dynamo:\"b,partitionkey\"`\n}",
			want: "are both partitionkey",
		},
		{
			name: "two sort keys",
			body: "type Reading struct {\n\tP string `dynamo:\"p,partitionkey\"`\n\tA string `dynamo:\"a,sortkey\"`\n\tB string `dynamo:\"b,sortkey\"`\n}",
			want: "are both sortkey",
		},
		{
			name: "sort key without partition key",
			body: "type Reading struct {\n\tA string `dynamo:\"a,sortkey\"`\n}",
			want: "is sortkey without a partitionkey",
		},
		{
			name: "key of an unsupported attribute type",
			body: "type Reading struct {\n\tOn bool `dynamo:\"on,partitionkey\"`\n}",
			want: "key must be S, N or B",
		},
		{
			name: "unsupported field type",
			body: "type Reading struct {\n\tP string `dynamo:\"p,partitionkey\"`\n\tCh chan int `dynamo:\"ch\"`\n}",
			want: "has no DynamoDB attribute type",
		},
		{
			name: "map with a non-string key",
			body: "type Reading struct {\n\tP string `dynamo:\"p,partitionkey\"`\n\tM map[int]string `dynamo:\"m\"`\n}",
			want: "needs a string key",
		},
		{
			name: "set on a non-slice field",
			body: "type Reading struct {\n\tP string `dynamo:\"p,partitionkey\"`\n\tS string `dynamo:\"s,stringset\"`\n}",
			want: "cannot be a set",
		},
		{
			name: "set with the wrong element type",
			body: "type Reading struct {\n\tP string `dynamo:\"p,partitionkey\"`\n\tS []int `dynamo:\"s,stringset\"`\n}",
			want: "stringset needs string elements",
		},
		{
			name: "unixtime on a field that is not a time",
			body: "type Reading struct {\n\tP string `dynamo:\"p,partitionkey\"`\n\tN int64 `dynamo:\"n,unixtime\"`\n}",
			want: "unixtime applies to time.Time",
		},
		{
			name: "sdk tag without the tinybind tag",
			body: "type Reading struct {\n\tP string `dynamo:\"p,partitionkey\"`\n\tCity string `dynamodbav:\"city\"`\n}",
			want: `found tag "dynamodbav" but no "dynamo" tag`,
		},
		{
			name: "method collision",
			body: "type Reading struct {\n\tP string `dynamo:\"p,partitionkey\"`\n}\n\nfunc (r Reading) EncodeItem() dynamodb.Item { return nil }",
			want: "already declares EncodeItem",
		},
		{
			name: "nested type from another package",
			body: "type Reading struct {\n\tP string `dynamo:\"p,partitionkey\"`\n\tPage dynamodb.Page `dynamo:\"page\"`\n}",
			want: "declared in another package",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := dynamoModule(t, dynamoSource(test.body, `	_ = dynamobind.Store(ctx, c, "t", Reading{})`))
			_, err := generator.AnalyzeDynamoItemsWithOptions(dir, generator.DefaultOptions())
			if err == nil {
				t.Fatalf("expected an error naming the field")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error %q does not contain %q", err, test.want)
			}
		})
	}
}
