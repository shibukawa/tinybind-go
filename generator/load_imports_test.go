package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// The entities package as a caller writes it: a write call, a read call, and a
// type that carries neither codec yet, which is the state every first run is in.
const importCheckSource = `package entities

import (
	"context"

	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

type Note struct {
	ID     string ` + "`firestore:\"-,name\"`" + `
	Author string ` + "`firestore:\"author\"`" + `
}

func Save(ctx context.Context, note Note) (datastore.Key, error) {
	return firestorebind.Store(ctx, note)
}

func Read(ctx context.Context, key datastore.Key) (Note, error) {
	return firestorebind.Load[Note](ctx, key)
}
`

// A module that names no dependency, so firestorebind resolves to nothing.
func unresolvedModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tempmod\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOFLAGS", "-mod=readonly")
	t.Setenv("GOPROXY", "off")
	return dir
}

// A package whose firestorebind import resolves to nothing discovers no call
// site at all, and used to be generated anyway: the read half arrived from the
// declaration file, which is parsed from disk and needs no types, and the write
// half did not, so a successful run emitted a decoder and no encoder. That reads
// as a missing discovery rule for the write patterns rather than as a package
// the generator could not see, so the run has to fail instead.
func TestUnresolvedImportIsReportedRatherThanGeneratedAround(t *testing.T) {
	dir := unresolvedModule(t, map[string]string{
		"note.go": importCheckSource,
		"queries.tb.firestore": `
export statement ByAuthor(author: String): firestore.many<Note> {
  where author == {author}
}
`,
	})
	_, err := generator.AnalyzeFirestoreEntitiesWithOptions(dir, generator.DefaultOptions())
	if err == nil {
		t.Fatal("a package whose firestorebind import resolved to nothing was analyzed as if it had")
	}
	for _, want := range []string{"github.com/shibukawa/tinybind-go/firestorebind", "go mod tidy"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// The same package with no declaration file discovered nothing and reported no
// entities, which reads as a package with no Firestore use rather than as one
// that could not be read.
func TestUnresolvedImportIsReportedWithNoDeclaration(t *testing.T) {
	dir := unresolvedModule(t, map[string]string{"note.go": importCheckSource})
	_, err := generator.AnalyzeFirestoreEntitiesWithOptions(dir, generator.DefaultOptions())
	if err == nil {
		t.Fatal("a package whose firestorebind import resolved to nothing reported no entities instead of an error")
	}
	if !strings.Contains(err.Error(), "github.com/shibukawa/tinybind-go/firestorebind") {
		t.Errorf("error %q does not name the import that failed", err)
	}
}

// The check must not catch the type errors that every first run has: before
// generation the bound type satisfies neither codec interface, and that is the
// state the generator exists to fix.
func TestMissingCodecMethodsAreNotAnUnresolvedImport(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "note.go"), []byte(importCheckSource), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzeFirestoreEntitiesWithOptions(dir, generator.DefaultOptions())
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(plan.Entities) != 1 {
		t.Fatalf("got %d entities, want 1", len(plan.Entities))
	}
	// The write call directs the encoder and the read call the decoder, which is
	// what usage direction promises and what the silent load hid.
	want := generator.FirestoreEncode | generator.FirestoreDecode | generator.FirestoreKey
	if got := plan.Entities[0].Usage; got != want {
		t.Errorf("usage %d, want %d; Store directs the encoder and Load the decoder", got, want)
	}
}
