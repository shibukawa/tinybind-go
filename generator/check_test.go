package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

func TestParseCheckTag_CoreRules(t *testing.T) {
	c, err := generator.ParseCheckTag("required,min=1,max=10,minlen=2,maxlen=5,email,uuid,date,time,datetime,pattern=^[a-z]+$", "string")
	// min/max invalid on string — parse should fail type check
	if err == nil {
		t.Fatalf("expected type error for min on string, got %+v", c)
	}

	c, err = generator.ParseCheckTag("required,minlen=1,maxlen=64,email,pattern=^[a-z]+$", "string")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Required || c.MinLen == nil || *c.MinLen != 1 || !c.Email || c.Pattern != "^[a-z]+$" {
		t.Fatalf("%+v", c)
	}

	c, err = generator.ParseCheckTag("min=1,max=150", "int")
	if err != nil {
		t.Fatal(err)
	}
	if c.Min == nil || *c.Min != 1 || c.Max == nil || *c.Max != 150 {
		t.Fatalf("%+v", c)
	}
}

// default and enum moved out of the check tag, so the old spellings have to
// fail loudly rather than parse into something nobody reads.
func TestParseCheckTag_RejectsMovedRules(t *testing.T) {
	err := mustCheckTagError(t, "min=1,default=-1", "int")
	if !strings.Contains(err.Error(), `default:"-1"`) {
		t.Fatalf("error must point at the default tag, got: %v", err)
	}

	// The suggested spelling has to be the new separator, not the old one.
	err = mustCheckTagError(t, "required,enum=asc|desc", "string")
	if !strings.Contains(err.Error(), `enum:"asc,desc"`) {
		t.Fatalf("error must point at the enum tag, got: %v", err)
	}
}

func mustCheckTagError(t *testing.T, raw, kind string) error {
	t.Helper()
	c, err := generator.ParseCheckTag(raw, kind)
	if err == nil {
		t.Fatalf("expected error for check tag %q, got %+v", raw, c)
	}
	return err
}

func TestParseDefaultTag(t *testing.T) {
	d, err := generator.ParseDefaultTag("-1", "int")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Set || d.Value != "-1" {
		t.Fatalf("%+v", d)
	}

	// An empty default is a real empty-string default, not an absent tag.
	d, err = generator.ParseDefaultTag("", "string")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Set || d.Value != "" {
		t.Fatalf("%+v", d)
	}

	for _, tc := range []struct{ raw, kind string }{
		{"nope", "int"},
		{"maybe", "bool"},
		{"x", "float64"},
		{"anything", "file"},
		{"anything", generator.KindStruct},
		{"anything", generator.KindSlice},
		{"anything", generator.KindRestAny},
	} {
		if _, err := generator.ParseDefaultTag(tc.raw, tc.kind); err == nil {
			t.Fatalf("expected error for default %q on %s", tc.raw, tc.kind)
		}
	}
}

func TestParseCheckTag_RejectsInvalid(t *testing.T) {
	cases := []struct {
		raw, kind string
	}{
		{"unknown", "string"},
		{"min=x", "int"},
		{"min=1", "string"},
		{"minlen=1", "int"},
		{"email", "int"},
		{"pattern=(", "string"},
	}
	for _, tc := range cases {
		if _, err := generator.ParseCheckTag(tc.raw, tc.kind); err == nil {
			t.Fatalf("expected error for %q on %s", tc.raw, tc.kind)
		}
	}
}

func TestParseCheckTag_PatternLastWithComma(t *testing.T) {
	c, err := generator.ParseCheckTag("required,pattern=a,b", "string")
	if err != nil {
		t.Fatal(err)
	}
	if c.Pattern != "a,b" || !c.Required {
		t.Fatalf("%+v", c)
	}
}

func TestEmit_ValidateThenDefaultOrder(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

type Sentinel struct {
	N int ` + "`query:\"n\" check:\"min=1\" default:\"-1\"`" + `
	Name string ` + "`query:\"name\" check:\"required,minlen=1\"`" + `
	Code string ` + "`query:\"code\" check:\"pattern=^[A-Z]{3}$\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(code)
	if strings.Contains(s, "reflect") {
		t.Fatal("reflect in generated code")
	}
	// validate before defaults
	vi := strings.Index(s, "httpbind.Validation(checkFields")
	di := strings.Index(s, "out.N = -1")
	if vi < 0 || di < 0 || vi > di {
		t.Fatalf("expected validate before default; vi=%d di=%d\n%s", vi, di, s)
	}
	if !strings.Contains(s, "presentN") || !strings.Contains(s, "must be >= 1") {
		t.Fatalf("missing min/presence:\n%s", s)
	}
	if !strings.Contains(s, "regexp.MustCompile") || !strings.Contains(s, "checkPatternSentinelCode") {
		t.Fatalf("missing pattern var:\n%s", s)
	}
}

func TestAnalyzePackage_InvalidCheckFails(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample
type Bad struct {
	X string ` + "`check:\"min=1\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "t.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	if _, err := generator.AnalyzePackage(dir); err == nil {
		t.Fatal("expected analyze error")
	}
}
