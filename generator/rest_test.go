package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

func TestAnalyze_PayloadRestMap(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import "encoding/json"

type Patch struct {
	Name  string         ` + "`payload:\"name\"`" + `
	Extra map[string]any ` + "`payload:\"*\"`" + `
}

type PatchRaw struct {
	Name  string                     ` + "`payload:\"name\"`" + `
	Extra map[string]json.RawMessage ` + "`payload:\"*\"`" + `
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
	var foundAny, foundRaw bool
	for _, tp := range plan.Types {
		for _, f := range tp.Fields {
			if f.Name != "Extra" {
				continue
			}
			if f.Wire != "*" || f.Source != generator.SourcePayload {
				t.Fatalf("%s Extra wire/source: %q %q", tp.Name, f.Wire, f.Source)
			}
			switch f.Kind {
			case generator.KindRestAny:
				foundAny = true
			case generator.KindRestRaw:
				foundRaw = true
			default:
				t.Fatalf("unexpected kind %q", f.Kind)
			}
		}
	}
	if !foundAny || !foundRaw {
		t.Fatalf("rest fields not planned: any=%v raw=%v plan=%+v", foundAny, foundRaw, plan)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(code)
	for _, n := range []string{"RestJSONAny", "RestJSONMember", "RestFormAny", `"name"`} {
		if !strings.Contains(s, n) {
			t.Fatalf("missing %q in:\n%s", n, s)
		}
	}
}

func TestAnalyze_PayloadRestRejectsMultiple(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample
type Bad struct {
	A map[string]any ` + "`payload:\"*\"`" + `
	B map[string]any ` + "`payload:\"*\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "t.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	_, err := generator.AnalyzePackage(dir)
	if err == nil {
		t.Fatal("expected error for multiple rest fields")
	}
	if !strings.Contains(err.Error(), "at most one") {
		t.Fatalf("error: %v", err)
	}
}

func TestAnalyze_PayloadRestRejectsWrongType(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample
type Bad struct {
	Extra string ` + "`payload:\"*\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "t.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	_, err := generator.AnalyzePackage(dir)
	if err == nil {
		t.Fatal("expected error for non-map rest")
	}
	if !strings.Contains(err.Error(), "map[string]") {
		t.Fatalf("error: %v", err)
	}
}

func TestAnalyze_PayloadRestRejectsInputStar(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample
type Bad struct {
	Extra map[string]any ` + "`input:\"*\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "t.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	_, err := generator.AnalyzePackage(dir)
	if err == nil {
		t.Fatal("expected error for input:\"*\"")
	}
}
