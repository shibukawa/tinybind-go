package generator_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// A set is meant to be invisible in the output and visible only in what a run
// spends, so every test here checks both: the same bytes as loading each
// directory on its own, and the type checks that did not happen.

func TestLoadPackagesMatchesPerDirectoryLoading(t *testing.T) {
	_, dirs := writeConcurrentFixture(t, concurrentFixturePackages)
	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"
	runner := generator.New(options)

	// First, so the help backfill has happened and both runs read one tree.
	perDirectory := fingerprintAll(t, runner, dirs, false)

	before := generator.PackageLoadCount()
	set, err := generator.LoadPackages(context.Background(), dirs)
	if err != nil {
		t.Fatal(err)
	}
	if loads := generator.PackageLoadCount() - before; loads != 1 {
		t.Errorf("loading %d directories together type-checked %d times, want 1", len(dirs), loads)
	}
	if set.Len() != len(dirs) {
		t.Fatalf("the set covers %d of %d directories", set.Len(), len(dirs))
	}

	before = generator.PackageLoadCount()
	fromSet := fingerprintAllFrom(t, runner, dirs, set, false)
	if loads := generator.PackageLoadCount() - before; loads != 0 {
		t.Errorf("generating %d directories from a set type-checked %d times, want none", len(dirs), loads)
	}
	for i, dir := range dirs {
		if fromSet[i] != perDirectory[i] {
			t.Errorf("%s: generating from a set produced different output:\nper directory:\n%s\nfrom set:\n%s",
				dir, perDirectory[i], fromSet[i])
		}
	}
}

// TestLoadPackagesServesConcurrentGeneration is the pairing the two features
// exist for, and the exposure it adds: one load means the directories now share
// their dependencies' type objects instead of holding a copy each, and they read
// them from several goroutines at once. Run under -race this is that sharing
// held to the detector.
func TestLoadPackagesServesConcurrentGeneration(t *testing.T) {
	_, dirs := writeConcurrentFixture(t, concurrentFixturePackages)
	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"
	runner := generator.New(options)

	// The backfill happens here, so neither measured pass writes a source.
	fingerprintAll(t, runner, dirs, false)

	// The concurrent pass runs against a set nothing has read yet, because the
	// lazy state inside go/types - an interface's type set, a named type's
	// resolution - is computed on first use, and a pass over a warm set would
	// not be the first use. Each pass gets its own load for that reason.
	cold, err := generator.LoadPackages(context.Background(), dirs)
	if err != nil {
		t.Fatal(err)
	}
	concurrent := fingerprintAllFrom(t, runner, dirs, cold, true)

	baseline, err := generator.LoadPackages(context.Background(), dirs)
	if err != nil {
		t.Fatal(err)
	}
	sequential := fingerprintAllFrom(t, runner, dirs, baseline, false)
	for i, dir := range dirs {
		if concurrent[i] != sequential[i] {
			t.Errorf("%s: generating concurrently from one set produced different output:\nsequential:\n%s\nconcurrent:\n%s",
				dir, sequential[i], concurrent[i])
		}
	}
}

// TestLoadPackagesFallsBackForUncoveredDirectory covers the directory a set does
// not answer for, which is a tree that changed under it as much as it is one
// nobody asked to load. It has to generate, not fail.
func TestLoadPackagesFallsBackForUncoveredDirectory(t *testing.T) {
	_, dirs := writeConcurrentFixture(t, concurrentFixturePackages)
	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"
	runner := generator.New(options)

	perDirectory := fingerprintAll(t, runner, dirs, false)

	covered := dirs[:len(dirs)-1]
	set, err := generator.LoadPackages(context.Background(), covered)
	if err != nil {
		t.Fatal(err)
	}
	if set.Len() != len(covered) {
		t.Fatalf("the set covers %d of %d directories", set.Len(), len(covered))
	}

	before := generator.PackageLoadCount()
	fromSet := fingerprintAllFrom(t, runner, dirs, set, false)
	if loads := generator.PackageLoadCount() - before; loads != 1 {
		t.Errorf("one uncovered directory type-checked %d times, want 1", loads)
	}
	for i, dir := range dirs {
		if fromSet[i] != perDirectory[i] {
			t.Errorf("%s: the fallback produced different output:\nper directory:\n%s\nfrom set:\n%s",
				dir, perDirectory[i], fromSet[i])
		}
	}
}

// TestLoadPackagesReportsNothingForNoDirectories keeps the empty call from
// running go list at all, since a caller with nothing to generate is ordinary.
func TestLoadPackagesReportsNothingForNoDirectories(t *testing.T) {
	before := generator.PackageLoadCount()
	set, err := generator.LoadPackages(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if set.Len() != 0 {
		t.Errorf("an empty load covers %d directories", set.Len())
	}
	if loads := generator.PackageLoadCount() - before; loads != 0 {
		t.Errorf("an empty load type-checked %d times", loads)
	}
}

// TestLoadPackagesLeavesOutWhatItCannotUse pins the degradation the doc
// promises. A directory the batch cannot resolve - outside the module, holding
// no package, not there at all - is dropped from the set instead of failing the
// load, and its generation then produces exactly the diagnostic it produces
// with no set at all. That equality is the whole reason for dropping rather
// than reporting: the batch has no good place to say what is wrong with one
// directory, and the directory's own load does.
func TestLoadPackagesLeavesOutWhatItCannotUse(t *testing.T) {
	root, dirs := writeConcurrentFixture(t, 3)

	outside := t.TempDir()
	writeTempModule(t, outside)
	if err := os.WriteFile(filepath.Join(outside, "x.go"), []byte("package other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(root, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "nope")

	set, err := generator.LoadPackages(context.Background(),
		append(append([]string{}, dirs...), outside, empty, missing))
	if err != nil {
		t.Fatalf("a load holding three unusable directories failed outright: %v", err)
	}
	if set.Len() != len(dirs) {
		t.Errorf("the set covers %d directories, want the %d usable ones", set.Len(), len(dirs))
	}

	options := generator.DefaultOptions()
	options.SQLDialect = "postgresql"
	runner := generator.New(options)
	if _, err := artifactFingerprint(runner, dirs[0], set); err != nil {
		t.Errorf("a covered directory did not generate: %v", err)
	}

	// An empty directory is one the batch drops and a single load accepts, so
	// it is the sharpest test of the fallback being invisible: the set answers
	// for it by not answering, and the outcome is unchanged either way.
	fromSet, setErr := artifactFingerprint(runner, empty, set)
	alone, aloneErr := artifactFingerprint(runner, empty, nil)
	if (setErr == nil) != (aloneErr == nil) {
		t.Fatalf("the fallback changed whether generating succeeds: set=%v none=%v", setErr, aloneErr)
	}
	if setErr != nil && setErr.Error() != aloneErr.Error() {
		t.Errorf("the fallback changed the diagnostic:\nfrom set: %v\nno set:   %v", setErr, aloneErr)
	}
	if fromSet != alone {
		t.Errorf("the fallback changed the output:\nfrom set:\n%s\nno set:\n%s", fromSet, alone)
	}
}
