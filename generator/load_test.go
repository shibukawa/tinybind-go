package generator_test

import (
	"context"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// TestGenerationTypeChecksThePackageOnce pins the cost model of a run: binder,
// configbind, route discovery and OpenAPI are four readings of one type-checked
// package, and the type check is what generation actually spends its time on.
func TestGenerationTypeChecksThePackageOnce(t *testing.T) {
	_, fixture := copyCustomFrameworkFixture(t)
	runner := generator.New(customFrameworkOptions(t))

	before := generator.PackageLoadCount()
	if _, err := runner.GeneratePackage(context.Background(), stampRequest(fixture)); err != nil {
		t.Fatal(err)
	}
	if loads := generator.PackageLoadCount() - before; loads != 1 {
		t.Fatalf("GeneratePackage type-checked %d times, want 1", loads)
	}

	before = generator.PackageLoadCount()
	if _, err := runner.GenerateArtifacts(context.Background(), generator.GenerateRequest{Dir: fixture, OpenAPI: true}); err != nil {
		t.Fatal(err)
	}
	if loads := generator.PackageLoadCount() - before; loads != 1 {
		t.Fatalf("GenerateArtifacts type-checked %d times, want 1", loads)
	}
}
