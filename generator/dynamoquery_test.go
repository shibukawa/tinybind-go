package generator

import (
	"strings"
	"testing"
)

func TestParseDynamoQueries(t *testing.T) {
	source := `
// Readings for one sensor, newest page first.
export statement ReadingsBySensor(sensor: Sensor, from: int64): dynamo.many<Reading> {
  table readings
  key sensor = {sensor} and at > {from}
}

statement readingsAround(sensor: Sensor, low: int64, high: int64): dynamo.page<Reading> {
  table sensor-readings.v2
  key sensor = {sensor} and at between {low} and {high}
}

export statement ReadingsByPrefix(sensor: Sensor, prefix: string): dynamo.many<Reading> {
  table readings; key sensor = {sensor} and begins_with(at, {prefix})
}

export statement ReadingsForSensor(sensor: Sensor): dynamo.many<Reading> {
  key sensor = {sensor}
  table readings
}
`
	decls, err := parseDynamoQueries("readings.tb.dynamo", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(decls) != 4 {
		t.Fatalf("parsed %d declarations, want 4", len(decls))
	}

	first := decls[0]
	if !first.Exported || first.Name != "ReadingsBySensor" {
		t.Errorf("first declaration: %+v", first)
	}
	if first.Shape != DynamoMany || first.ItemType != "Reading" {
		t.Errorf("result type: %s<%s>", first.Shape, first.ItemType)
	}
	if first.Table != "readings" {
		t.Errorf("table: %q", first.Table)
	}
	if len(first.Params) != 2 || first.Params[0].Name != "sensor" || first.Params[0].Type != "Sensor" ||
		first.Params[1].Name != "from" || first.Params[1].Type != "int64" {
		t.Errorf("params: %+v", first.Params)
	}
	if len(first.Key) != 2 {
		t.Fatalf("key predicates: %+v", first.Key)
	}
	if first.Key[0].Attribute != "sensor" || first.Key[0].Op != DynamoEqual || first.Key[0].Params[0] != "sensor" {
		t.Errorf("partition predicate: %+v", first.Key[0])
	}
	if first.Key[1].Attribute != "at" || first.Key[1].Op != DynamoGreater || first.Key[1].Params[0] != "from" {
		t.Errorf("sort predicate: %+v", first.Key[1])
	}

	if decls[1].Exported || decls[1].Shape != DynamoPage {
		t.Errorf("unexported page declaration: %+v", decls[1])
	}
	// A table name may hold the hyphens and dots DynamoDB allows, which no Go
	// identifier does.
	if decls[1].Table != "sensor-readings.v2" {
		t.Errorf("table: %q", decls[1].Table)
	}
	if between := decls[1].Key[1]; between.Op != DynamoBetween || len(between.Params) != 2 ||
		between.Params[0] != "low" || between.Params[1] != "high" {
		t.Errorf("between: %+v", between)
	}
	if prefix := decls[2].Key[1]; prefix.Op != DynamoBeginsWith || prefix.Attribute != "at" || prefix.Params[0] != "prefix" {
		t.Errorf("begins_with: %+v", prefix)
	}
	if len(decls[3].Key) != 1 {
		t.Errorf("a partition-only key condition is valid: %+v", decls[3].Key)
	}
}

func TestParseDynamoQueryErrors(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "missing statement keyword",
			source: "export query Foo(): dynamo.many<Reading> { table readings; key a = {b} }",
			want:   `expected "statement" or "export statement"`,
		},
		{
			name:   "unknown result shape",
			source: "export statement Foo(): dynamo.one<Reading> { table readings; key a = {b} }",
			want:   `expected "dynamo.many<T>" or "dynamo.page<T>"`,
		},
		{
			name:   "no key clause",
			source: "export statement Foo(): dynamo.many<Reading> { table readings }",
			want:   "declares no key clause",
		},
		{
			name:   "no table clause",
			source: "export statement Foo(a: string): dynamo.many<Reading> { key a = {a} }",
			want:   `declares no table clause; write "table <name>" in its body`,
		},
		{
			name:   "two table clauses",
			source: "export statement Foo(a: string): dynamo.many<Reading> { table readings\n table others\n key a = {a} }",
			want:   "more than one table clause",
		},
		{
			name:   "a table name DynamoDB would reject",
			source: "export statement Foo(a: string): dynamo.many<Reading> { table ab; key a = {a} }",
			want:   "expected a table name",
		},
		{
			name:   "filter is not supported yet",
			source: "export statement Foo(a: string): dynamo.many<Reading> { table readings; key a = {a}\n filter b = {a} }",
			want:   "filter clause is not supported yet",
		},
		{
			name:   "two key clauses",
			source: "export statement Foo(a: string): dynamo.many<Reading> { table readings; key a = {a}\n key a = {a} }",
			want:   "more than one key clause",
		},
		{
			name:   "bad comparison",
			source: "export statement Foo(a: string): dynamo.many<Reading> { table readings; key a != {a} }",
			want:   "expected a comparison after a",
		},
		{
			name:   "unterminated",
			source: "export statement Foo(a: string): dynamo.many<Reading> { table readings; key a = {a}",
			want:   "unterminated statement Foo",
		},
		{
			name:   "export with an unexported name",
			source: "export statement foo(a: string): dynamo.many<Reading> { table readings; key a = {a} }",
			want:   "declared export but its name is unexported",
		},
		{
			name:   "exported name without export",
			source: "statement Foo(a: string): dynamo.many<Reading> { table readings; key a = {a} }",
			want:   `write "export statement Foo"`,
		},
		{
			name:   "placeholder without braces",
			source: "export statement Foo(a: string): dynamo.many<Reading> { table readings; key a = a }",
			want:   `expected "{"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseDynamoQueries("x.tb.dynamo", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error %q does not contain %q", err, test.want)
			}
		})
	}
}

// TestPlanDynamoQueryChecks covers rule:dynamo-query-checks. Each case must name
// the attribute or parameter at fault, since the message is the whole reason for
// checking at generation time.
func TestPlanDynamoQueryChecks(t *testing.T) {
	item := DynamoItemPlan{
		Name: "Reading",
		Fields: []DynamoFieldPlan{
			{Name: "Sensor", Attribute: "sensor", Key: "partition", Type: DynamoType{Kind: DynamoString, Go: "Sensor"}},
			{Name: "At", Attribute: "at", Key: "sort", Type: DynamoType{Kind: DynamoInt, Go: "int64", Bits: 64}},
			{Name: "Note", Attribute: "note", Type: DynamoType{Kind: DynamoString, Go: "string"}},
		},
	}
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "unknown item type",
			source: "export statement Q(a: string): dynamo.many<Missing> { table readings; key sensor = {a} }",
			want:   "no type Missing in this package carries dynamo tags",
		},
		{
			name:   "attribute the type does not have",
			source: "export statement Q(a: Sensor): dynamo.many<Reading> { table readings; key nope = {a} }",
			want:   `Reading has no attribute "nope"`,
		},
		{
			name:   "non-key attribute in the key clause",
			source: "export statement Q(a: Sensor, b: string): dynamo.many<Reading> { table readings; key sensor = {a} and note = {b} }",
			want:   "note is not a key of Reading; a key condition reaches sensor and at",
		},
		{
			name:   "partition key with an inequality",
			source: "export statement Q(a: Sensor): dynamo.many<Reading> { table readings; key sensor > {a} }",
			want:   `the partition key sensor takes "=" only`,
		},
		{
			name:   "sort key first",
			source: "export statement Q(a: int64): dynamo.many<Reading> { table readings; key at > {a} }",
			want:   "a key condition starts with the partition key sensor",
		},
		{
			name:   "two sort predicates",
			source: "export statement Q(a: Sensor, b: int64, c: int64): dynamo.many<Reading> { table readings; key sensor = {a} and at > {b} and at < {c} }",
			want:   "at most one sort key predicate",
		},
		{
			name:   "parameter type does not match the attribute",
			source: "export statement Q(a: string): dynamo.many<Reading> { table readings; key sensor = {a} }",
			want:   "parameter a is string, but attribute sensor is stored from Sensor",
		},
		{
			name:   "undeclared parameter",
			source: "export statement Q(a: Sensor): dynamo.many<Reading> { table readings; key sensor = {b} }",
			want:   "no parameter named b is declared",
		},
		{
			name:   "unused parameter",
			source: "export statement Q(a: Sensor, spare: int64): dynamo.many<Reading> { table readings; key sensor = {a} }",
			want:   "parameter spare is declared but never used",
		},
		{
			name:   "begins_with on a number",
			source: "export statement Q(a: Sensor, b: int64): dynamo.many<Reading> { table readings; key sensor = {a} and begins_with(at, {b}) }",
			want:   "begins_with reads a string attribute, and at is stored as N",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decls, err := parseDynamoQueries("q.tb.dynamo", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err = planDynamoQueries(decls, []DynamoItemPlan{item})
			if err == nil {
				t.Fatal("expected a generation error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error %q does not contain %q", err, test.want)
			}
		})
	}
}

// TestPlanDynamoQueryAliasesEveryName pins the reserved-word rule: an alias is
// emitted for every attribute, so no reserved-word list has to be carried.
func TestPlanDynamoQueryAliasesEveryName(t *testing.T) {
	item := DynamoItemPlan{
		Name: "Event",
		Fields: []DynamoFieldPlan{
			{Name: "Status", Attribute: "status", Key: "partition", Type: DynamoType{Kind: DynamoString, Go: "string"}},
			{Name: "Timestamp", Attribute: "timestamp", Key: "sort", Type: DynamoType{Kind: DynamoInt, Go: "int64", Bits: 64}},
		},
	}
	decls, err := parseDynamoQueries("q.tb.dynamo", []byte(
		"export statement Q(s: string, t: int64): dynamo.many<Event> { table events; key status = {s} and timestamp >= {t} }"))
	if err != nil {
		t.Fatal(err)
	}
	plans, err := planDynamoQueries(decls, []DynamoItemPlan{item})
	if err != nil {
		t.Fatal(err)
	}
	plan := plans[0]
	if plan.Expression != "#k0 = :v0 AND #k1 >= :v1" {
		t.Fatalf("expression: %q", plan.Expression)
	}
	if plan.Names["#k0"] != "status" || plan.Names["#k1"] != "timestamp" {
		t.Fatalf("names: %v", plan.Names)
	}
	// Both attribute names are DynamoDB reserved words, and neither reaches the
	// expression literally.
	if strings.Contains(plan.Expression, "status") || strings.Contains(plan.Expression, "timestamp") {
		t.Fatalf("a reserved word reached the expression: %q", plan.Expression)
	}
}

// TestDynamoQueryFileIsAGenerationInput proves an edited declaration invalidates
// the generation cache. A query file that the fingerprint ignored would leave a
// stale function in place after its declaration changed.
func TestDynamoQueryFileIsAGenerationInput(t *testing.T) {
	patterns := []string{DefaultHTMLTemplatePattern, DefaultSQLTemplatePattern, DefaultDynamoTemplatePattern}
	if !isGenerationInput("readings.tb.dynamo", patterns...) {
		t.Fatal("a .tb.dynamo file must be hashed as a generation input")
	}
	if isGenerationInput("readings_test.go", patterns...) {
		t.Fatal("a test file is not a generation input")
	}
}
