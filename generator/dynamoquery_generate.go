package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// defaultDynamoQueryOut is the generated query output file.
const defaultDynamoQueryOut = "dynamoquery_gen.go"

// discoverDynamoQueryFiles lists the query declaration sources in dir.
func discoverDynamoQueryFiles(dir, pattern string) ([]string, error) {
	pattern = templatePattern(pattern, DefaultDynamoTemplatePattern)
	if _, err := filepath.Match(pattern, ""); err != nil {
		return nil, fmt.Errorf("invalid DynamoDB query pattern %q: %w", pattern, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match, err := filepath.Match(pattern, entry.Name())
		if err != nil {
			return nil, err // the pattern was validated above
		}
		if match {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

// declaredDynamoItemTypes lists the item types the query declarations of dir
// decode into. It is the item pass's view of the declarations: which types are
// bound, without the checking the query pass does.
func declaredDynamoItemTypes(dir string, opts Options) ([]string, error) {
	if opts.featureDisabled(FeatureItemCodec) {
		return nil, nil
	}
	files, err := discoverDynamoQueryFiles(dir, opts.DynamoTemplatePattern)
	if err != nil || len(files) == 0 {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		decls, err := parseDynamoQueries(path, source)
		if err != nil {
			return nil, err
		}
		for _, decl := range decls {
			if !seen[decl.ItemType] {
				seen[decl.ItemType] = true
				out = append(out, decl.ItemType)
			}
		}
	}
	return out, nil
}

// dynamoQueryPlans reads and checks every declaration of the package.
func (g *Generator) dynamoQueryPlans(load *packageLoad) (string, []DynamoQueryPlan, error) {
	if g.Options.featureDisabled(FeatureItemCodec) {
		return "", nil, nil
	}
	files, err := discoverDynamoQueryFiles(load.dir, g.Options.DynamoTemplatePattern)
	if err != nil || len(files) == 0 {
		return "", nil, err
	}
	items, err := analyzeDynamoItems(load, g.Options)
	if err != nil {
		return "", nil, err
	}
	var decls []DynamoQueryDecl
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		parsed, err := parseDynamoQueries(path, source)
		if err != nil {
			return "", nil, err
		}
		for i := range parsed {
			parsed[i].SourcePath = path
		}
		decls = append(decls, parsed...)
	}
	plans, err := planDynamoQueries(decls, items.Items)
	if err != nil {
		return "", nil, err
	}
	return items.Package, plans, nil
}

// GenerateDynamoQueries analyzes dir and writes the generated query functions.
// It returns "" when the package declares none.
func (g *Generator) GenerateDynamoQueries(dir, outDir, outName string) (string, error) {
	return g.generateDynamoQueries(newPackageLoad(dir), outDir, outName)
}

func (g *Generator) generateDynamoQueries(load *packageLoad, outDir, outName string) (string, error) {
	pkg, plans, err := g.dynamoQueryPlans(load)
	if err != nil || len(plans) == 0 {
		return "", err
	}
	src, err := EmitDynamoQueries(pkg, plans, g.Options.dynamoQueryOptions())
	if err != nil || len(src) == 0 {
		return "", err
	}
	if outDir == "" {
		outDir = load.dir
	}
	if outName == "" {
		outName = defaultDynamoQueryOut
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, outName)
	if err := os.WriteFile(path, src, 0o644); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

// dynamoQueryArtifacts emits one artifact per declaration source, so a package
// with two query files generates two, as every other per-source mode does.
func (g *Generator) dynamoQueryArtifacts(load *packageLoad) ([]Artifact, error) {
	pkg, plans, err := g.dynamoQueryPlans(load)
	if err != nil || len(plans) == 0 {
		return nil, err
	}
	grouped := map[string][]DynamoQueryPlan{}
	var order []string
	for _, plan := range plans {
		source := plan.Decl.SourcePath
		if _, seen := grouped[source]; !seen {
			order = append(order, source)
		}
		grouped[source] = append(grouped[source], plan)
	}
	sort.Strings(order)
	artifacts := make([]Artifact, 0, len(order))
	for _, source := range order {
		code, err := EmitDynamoQueries(pkg, grouped[source], g.Options.dynamoQueryOptions())
		if err != nil {
			return nil, err
		}
		if len(code) == 0 {
			continue
		}
		artifacts = append(artifacts, Artifact{
			SourcePath:  source,
			Kind:        ArtifactDynamoQuery,
			OutputBase:  artifactBase(source),
			PackageName: pkg,
			GoSource:    code,
		})
	}
	return artifacts, nil
}

// EmitDynamoQueriesFor analyzes dir and returns the generated query source
// without writing it, which is what a check needs.
func (g *Generator) EmitDynamoQueriesFor(dir string) ([]byte, error) {
	pkg, plans, err := g.dynamoQueryPlans(newPackageLoad(dir))
	if err != nil || len(plans) == 0 {
		return nil, err
	}
	return EmitDynamoQueries(pkg, plans, g.Options.dynamoQueryOptions())
}
