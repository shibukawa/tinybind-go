package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// defaultFirestoreQueryOut is the generated query output file.
const defaultFirestoreQueryOut = "firestorequery_gen.go"

// discoverFirestoreQueryFiles lists the query declaration sources in dir.
func discoverFirestoreQueryFiles(dir, pattern string) ([]string, error) {
	pattern = templatePattern(pattern, DefaultFirestoreTemplatePattern)
	if _, err := filepath.Match(pattern, ""); err != nil {
		return nil, fmt.Errorf("invalid Firestore query pattern %q: %w", pattern, err)
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

// declaredFirestoreEntityTypes lists the entity types the query declarations of
// dir decode into. It is the codec pass's view of the declarations: which types
// are bound, without the checking the query pass does.
func declaredFirestoreEntityTypes(dir string, opts Options) ([]string, error) {
	if opts.featureDisabled(FeatureEntityCodec) {
		return nil, nil
	}
	files, err := discoverFirestoreQueryFiles(dir, opts.FirestoreTemplatePattern)
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
		decls, err := parseFirestoreQueries(path, source)
		if err != nil {
			return nil, err
		}
		for _, decl := range decls {
			if !seen[decl.EntityType] {
				seen[decl.EntityType] = true
				out = append(out, decl.EntityType)
			}
		}
	}
	return out, nil
}

// firestoreQueryPlans reads and checks every declaration of the package.
func (g *Generator) firestoreQueryPlans(load *packageLoad) (string, []FirestoreQueryPlan, error) {
	if g.Options.featureDisabled(FeatureEntityCodec) {
		return "", nil, nil
	}
	files, err := discoverFirestoreQueryFiles(load.dir, g.Options.FirestoreTemplatePattern)
	if err != nil || len(files) == 0 {
		return "", nil, err
	}
	entities, err := analyzeFirestoreEntities(load, g.Options)
	if err != nil {
		return "", nil, err
	}
	var decls []FirestoreQueryDecl
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		parsed, err := parseFirestoreQueries(path, source)
		if err != nil {
			return "", nil, err
		}
		for i := range parsed {
			parsed[i].SourcePath = path
		}
		decls = append(decls, parsed...)
	}
	plans, err := planFirestoreQueries(decls, entities.Entities)
	if err != nil {
		return "", nil, err
	}
	return entities.Package, plans, nil
}

// GenerateFirestoreQueries analyzes dir and writes the generated query
// functions. It returns "" when the package declares none.
func (g *Generator) GenerateFirestoreQueries(dir, outDir, outName string) (string, error) {
	return g.generateFirestoreQueries(newPackageLoad(dir), outDir, outName)
}

func (g *Generator) generateFirestoreQueries(load *packageLoad, outDir, outName string) (string, error) {
	pkg, plans, err := g.firestoreQueryPlans(load)
	if err != nil || len(plans) == 0 {
		return "", err
	}
	src, err := EmitFirestoreQueries(pkg, plans)
	if err != nil || len(src) == 0 {
		return "", err
	}
	if outDir == "" {
		outDir = load.dir
	}
	if outName == "" {
		outName = defaultFirestoreQueryOut
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

// EmitFirestoreQueriesFor analyzes dir and returns the generated query source
// without writing it, which is what a check needs.
func (g *Generator) EmitFirestoreQueriesFor(dir string) ([]byte, error) {
	pkg, plans, err := g.firestoreQueryPlans(newPackageLoad(dir))
	if err != nil || len(plans) == 0 {
		return nil, err
	}
	return EmitFirestoreQueries(pkg, plans)
}
