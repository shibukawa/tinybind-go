package generator_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// TestGenerateArtifactsWithRoutes pins the export contract: one artifact run
// hands back the route analysis it performed, with resolved routes positioned
// at their registration sites, so a caller needs no go/packages load and no
// options normalization of its own.
func TestGenerateArtifactsWithRoutes(t *testing.T) {
	runner := generator.New(generator.DefaultOptions())
	dir := filepath.Join("..", "testdata", "basic_handlefunc")

	artifacts, routes, err := runner.GenerateArtifactsWithRoutes(context.Background(), generator.GenerateRequest{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) == 0 {
		t.Fatal("expected artifacts beside the routes")
	}
	if routes == nil || len(routes.Routes) != 1 {
		t.Fatalf("routes: %+v", routes)
	}
	route := routes.Routes[0]
	if route.Method != "POST" || route.Path != "/users/{id}" {
		t.Fatalf("route: %s %s", route.Method, route.Path)
	}
	if filepath.Base(route.Site.File) != "main.go" || route.Site.Line <= 0 || route.Site.Column <= 0 {
		t.Fatalf("registration site: %+v", route.Site)
	}
}

// TestGenerateArtifactsWithRoutesReportsUnresolvedSites pins the other half of
// the table: a registration the analysis cannot resolve is returned as a
// positioned diagnostic rather than silently dropped.
func TestGenerateArtifactsWithRoutesReportsUnresolvedSites(t *testing.T) {
	runner := generator.New(generator.DefaultOptions())
	dir := filepath.Join("..", "testdata", "unsupported_string_concat")

	_, routes, err := runner.GenerateArtifactsWithRoutes(context.Background(), generator.GenerateRequest{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if routes == nil || len(routes.Routes) != 0 {
		t.Fatalf("expected no resolved routes, got %+v", routes)
	}
	if len(routes.Diagnostics) == 0 {
		t.Fatal("expected the dynamic pattern as a diagnostic")
	}
	diag := routes.Diagnostics[0]
	if diag.File == "" || diag.Line <= 0 {
		t.Fatalf("diagnostic carries no site: %+v", diag)
	}
}

// TestRunParsesRoutesOnce extends the one-type-check cost model to the parse:
// the OpenAPI phase and the route export read one cached result.
func TestRunParsesRoutesOnce(t *testing.T) {
	_, fixture := copyCustomFrameworkFixture(t)
	runner := generator.New(customFrameworkOptions(t))

	before := generator.RouteParseCount()
	result, err := runner.GeneratePackage(context.Background(), stampRequest(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if parses := generator.RouteParseCount() - before; parses != 1 {
		t.Fatalf("GeneratePackage parsed routes %d times, want 1", parses)
	}
	if result.Routes == nil {
		t.Fatal("GeneratePackage returned no route analysis")
	}

	before = generator.RouteParseCount()
	_, routes, err := runner.GenerateArtifactsWithRoutes(context.Background(), generator.GenerateRequest{Dir: fixture, OpenAPI: true})
	if err != nil {
		t.Fatal(err)
	}
	if parses := generator.RouteParseCount() - before; parses != 1 {
		t.Fatalf("GenerateArtifactsWithRoutes parsed routes %d times, want 1", parses)
	}
	if routes == nil {
		t.Fatal("GenerateArtifactsWithRoutes returned no route analysis")
	}
}
