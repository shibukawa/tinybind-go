package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/httpbind-go/generator"
)

func TestParseCheckTag_CoreRules(t *testing.T) {
	c, err := generator.ParseCheckTag("required,min=1,max=10,minlen=2,maxlen=5,enum=a|b,default=-1,email,uuid,date,time,datetime,pattern=^[a-z]+$", "string")
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

	c, err = generator.ParseCheckTag("min=1,max=150,default=-1", "int")
	if err != nil {
		t.Fatal(err)
	}
	if c.Min == nil || *c.Min != 1 || !c.HasDefault || c.Default != "-1" {
		t.Fatalf("%+v", c)
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
		{"default=nope", "int"},
		{"enum=true|maybe", "bool"},
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
	src := `package sample

type Sentinel struct {
	N int ` + "`query:\"n\" check:\"min=1,default=-1\"`" + `
	Name string ` + "`query:\"name\" check:\"required,minlen=1\"`" + `
	Code string ` + "`query:\"code\" check:\"pattern=^[A-Z]{3}$\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
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
	vi := strings.Index(s, "httpbinder.Validation(checkFields")
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
	src := `package sample
type Bad struct {
	X string ` + "`check:\"min=1\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "t.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := generator.AnalyzePackage(dir); err == nil {
		t.Fatal("expected analyze error")
	}
}
