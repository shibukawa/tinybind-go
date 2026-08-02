package generator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// writeUnparsableSourceFixture writes a package holding one valid file and one
// file the Go parser cannot read. An editor that has created a file but not yet
// written into it leaves exactly that, and it is the shape that used to take a
// generation run's process down: packages.Load still reports a syntax entry for
// the file it could not parse, that entry's position is token.NoPos, and a
// FileSet lookup of token.NoPos is nil.
func writeUnparsableSourceFixture(t *testing.T, valid string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	return dir
}

// TestAnalyzePackageSurvivesUnparsableSource covers the binder analysis, which
// every generation run reaches.
func TestAnalyzePackageSurvivesUnparsableSource(t *testing.T) {
	dir := writeUnparsableSourceFixture(t, `package sample

type Req struct {
	Page int `+"`query:\"page\"`"+`
}
`)
	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Types) != 1 || plan.Types[0].Name != "Req" {
		t.Fatalf("types: %+v", plan.Types)
	}
}

// TestAnalyzeConfigBindSurvivesUnparsableSource covers the same loop over the
// configuration artifact.
func TestAnalyzeConfigBindSurvivesUnparsableSource(t *testing.T) {
	dir := writeUnparsableSourceFixture(t, `package sample

import "github.com/shibukawa/tinybind-go/configbind"

type Config struct {
	Port int `+"`config:\"port\"`"+`
}

func Load() (*Config, error) {
	return configbind.Bind[Config]("sample")
}
`)
	name, specs, err := generator.AnalyzeConfigBind(dir)
	if err != nil {
		t.Fatal(err)
	}
	if name != "sample" {
		t.Fatalf("package name: %q", name)
	}
	if len(specs) != 1 {
		t.Fatalf("specs: %+v", specs)
	}
}

// TestAnalyzeDynamoItemsSurvivesUnparsableSource covers dynamoFileName, the
// third reading of the same syntax list.
func TestAnalyzeDynamoItemsSurvivesUnparsableSource(t *testing.T) {
	dir := writeUnparsableSourceFixture(t, `package sample

type Item struct {
	ID   string `+"`dynamo:\"id,partitionkey\"`"+`
	Name string `+"`dynamo:\"name\"`"+`
}
`)
	plan, err := generator.AnalyzeDynamoItems(dir)
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("nil plan")
	}
}
