package configpath_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configpath"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveExplicitPathSuccess(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "app.toml")
	writeFile(t, cfg, "x = 1\n")

	// Directory search would find a different file; explicit must win.
	userRoot := filepath.Join(dir, "user")
	sysRoot := filepath.Join(dir, "system")
	other := filepath.Join(userRoot, "acme", "mytool", "app.toml")
	writeFile(t, other, "other = true\n")

	search := configpath.FixedRootsSearch{UserRoot: userRoot, SystemRoot: sysRoot}
	path, found, err := configpath.ResolveWithSearch("acme", "mytool", "app.toml", cfg, search)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !found {
		t.Fatal("expected found")
	}
	if path != filepath.Clean(cfg) {
		t.Fatalf("path=%q want %q", path, filepath.Clean(cfg))
	}
}

func TestResolveExplicitPathMissingNoFallback(t *testing.T) {
	dir := t.TempDir()
	userRoot := filepath.Join(dir, "user")
	sysRoot := filepath.Join(dir, "system")
	// Search would succeed if fallback were allowed.
	fallback := filepath.Join(userRoot, "acme", "mytool", "app.toml")
	writeFile(t, fallback, "from=user\n")

	missing := filepath.Join(dir, "does-not-exist.toml")
	search := configpath.FixedRootsSearch{UserRoot: userRoot, SystemRoot: sysRoot}
	path, found, err := configpath.ResolveWithSearch("acme", "mytool", "app.toml", missing, search)
	if err == nil {
		t.Fatal("expected error for missing --config-path")
	}
	if found || path != "" {
		t.Fatalf("must not fall back: path=%q found=%v", path, found)
	}
	if !strings.Contains(err.Error(), "missing or unreadable") {
		t.Fatalf("error=%v", err)
	}
	// Sanity: same search without explicit finds user file.
	path, found, err = configpath.ResolveWithSearch("acme", "mytool", "app.toml", "", search)
	if err != nil || !found || path != fallback {
		t.Fatalf("fallback candidate path=%q found=%v err=%v", path, found, err)
	}
}

func TestResolveUserWinsOverSystem(t *testing.T) {
	dir := t.TempDir()
	userRoot := filepath.Join(dir, "user")
	sysRoot := filepath.Join(dir, "system")
	userFile := filepath.Join(userRoot, "acme", "mytool", "app.toml")
	sysFile := filepath.Join(sysRoot, "acme", "mytool", "app.toml")
	writeFile(t, userFile, "from=user\n")
	writeFile(t, sysFile, "from=system\n")

	search := configpath.FixedRootsSearch{UserRoot: userRoot, SystemRoot: sysRoot}
	path, found, err := configpath.ResolveWithSearch("acme", "mytool", "app.toml", "", search)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected found")
	}
	if path != userFile {
		t.Fatalf("path=%q want user %q", path, userFile)
	}
	// Ensure we did not return system path.
	if path == sysFile {
		t.Fatal("must not prefer system when user has file")
	}
}

func TestResolveSystemOnlyWhenUserLacks(t *testing.T) {
	dir := t.TempDir()
	userRoot := filepath.Join(dir, "user")
	sysRoot := filepath.Join(dir, "system")
	sysFile := filepath.Join(sysRoot, "acme", "mytool", "app.toml")
	writeFile(t, sysFile, "from=system\n")
	// user dir exists but without the file
	if err := os.MkdirAll(filepath.Join(userRoot, "acme", "mytool"), 0o755); err != nil {
		t.Fatal(err)
	}

	search := configpath.FixedRootsSearch{UserRoot: userRoot, SystemRoot: sysRoot}
	path, found, err := configpath.ResolveWithSearch("acme", "mytool", "app.toml", "", search)
	if err != nil {
		t.Fatal(err)
	}
	if !found || path != sysFile {
		t.Fatalf("path=%q found=%v want system %q", path, found, sysFile)
	}
}

func TestResolveNeitherFound(t *testing.T) {
	dir := t.TempDir()
	search := configpath.FixedRootsSearch{
		UserRoot:   filepath.Join(dir, "user"),
		SystemRoot: filepath.Join(dir, "system"),
	}
	path, found, err := configpath.ResolveWithSearch("acme", "mytool", "app.toml", "", search)
	if err != nil {
		t.Fatalf("not found should not error: %v", err)
	}
	if found || path != "" {
		t.Fatalf("path=%q found=%v", path, found)
	}
}

func TestResolveRequiresVendorToolFileName(t *testing.T) {
	search := configpath.FixedRootsSearch{}
	cases := []struct {
		vendor, tool, file string
		wantSub            string
	}{
		{"", "mytool", "app.toml", "vendor"},
		{"acme", "", "app.toml", "tool"},
		{"acme", "mytool", "", "file name"},
	}
	for _, tc := range cases {
		_, _, err := configpath.ResolveWithSearch(tc.vendor, tc.tool, tc.file, "", search)
		if err == nil {
			t.Fatalf("expected error for vendor=%q tool=%q file=%q", tc.vendor, tc.tool, tc.file)
		}
		if !strings.Contains(err.Error(), tc.wantSub) {
			t.Fatalf("error %q missing %q", err, tc.wantSub)
		}
	}
}

func TestConfigPathDefAndParseThenResolve(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "from-cli.toml")
	writeFile(t, cfg, "cli=1\n")

	// Directory has another file that must not be used when CLI path is set.
	userRoot := filepath.Join(dir, "user")
	writeFile(t, filepath.Join(userRoot, "acme", "mytool", "app.toml"), "dir=1\n")

	defs := []cliparser.Def{configpath.ConfigPathDef()}
	if defs[0].ConfigKey != configpath.ProcessKey || defs[0].Longs[0] != "config-path" {
		t.Fatalf("def=%+v", defs[0])
	}
	// Not a bind-style prefix key
	if strings.Contains(defs[0].ConfigKey, ".") {
		t.Fatalf("process key must not look like bind key: %q", defs[0].ConfigKey)
	}

	res, err := cliparser.Parse([]string{"--config-path", cfg, "rest"}, defs)
	if err != nil {
		t.Fatal(err)
	}
	explicit := configpath.ExplicitPathFromParse(res)
	if explicit != cfg {
		t.Fatalf("explicit=%q want %q (values=%v)", explicit, cfg, res.Values)
	}

	search := configpath.FixedRootsSearch{UserRoot: userRoot, SystemRoot: filepath.Join(dir, "system")}
	path, found, err := configpath.ResolveWithSearch("acme", "mytool", "app.toml", explicit, search)
	if err != nil || !found || path != filepath.Clean(cfg) {
		t.Fatalf("path=%q found=%v err=%v", path, found, err)
	}
}

func TestResolveExplicitDirectoryIsError(t *testing.T) {
	dir := t.TempDir()
	_, _, err := configpath.ResolveWithSearch("a", "b", "c.toml", dir, configpath.FixedRootsSearch{})
	if err == nil {
		t.Fatal("expected error when --config-path is a directory")
	}
}

func TestResolveDefaultUsesConfigDirSearch(t *testing.T) {
	// Ship path: Resolve → ConfigDirSearch (github.com/shibukawa/configdir).
	// Unlikely names should report not found without error.
	path, found, err := configpath.Resolve(
		"tinybind-go-test-vendor",
		"configpath-smoke-tool",
		"no-such-configbind-file-xyz.toml",
		"",
	)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if found || path != "" {
		t.Fatalf("unexpected path=%q found=%v", path, found)
	}

	// Explicit bad path still errors through the same public entry point.
	_, _, err = configpath.Resolve("v", "t", "f.toml", filepath.Join(t.TempDir(), "missing.toml"))
	if err == nil {
		t.Fatal("expected error")
	}
}
