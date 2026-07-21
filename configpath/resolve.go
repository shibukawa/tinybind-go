// Package configpath resolves a single config file path for configbind.
// Explicit --config-path wins; otherwise configdir user then system is searched exclusively.
package configpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shibukawa/configdir"
)

// ProcessKey is the stable process-level key for the --config-path value in cliparser results.
// It is not a Bind-prefix overlay key.
const ProcessKey = "config-path"

// FileSearch finds a config file under vendor/tool directories (user first, then system).
// Production uses ConfigDirSearch; tests may inject fixed roots.
type FileSearch interface {
	// Find returns the full path to fileName if present under vendor/tool layout.
	Find(vendor, tool, fileName string) (fullPath string, found bool)
}

// ConfigDirSearch uses github.com/shibukawa/configdir (user then system, exclusive).
type ConfigDirSearch struct{}

// Find implements FileSearch using configdir.QueryFolderContainsFile.
func (ConfigDirSearch) Find(vendor, tool, fileName string) (string, bool) {
	cd := configdir.New(vendor, tool)
	folder := cd.QueryFolderContainsFile(fileName)
	if folder == nil {
		return "", false
	}
	return filepath.Join(folder.Path, fileName), true
}

// FixedRootsSearch searches userRoot then systemRoot with vendor/tool segments (for tests).
type FixedRootsSearch struct {
	UserRoot   string
	SystemRoot string
}

// Find implements FileSearch with explicit roots, user preferred over system.
func (s FixedRootsSearch) Find(vendor, tool, fileName string) (string, bool) {
	for _, root := range []string{s.UserRoot, s.SystemRoot} {
		if root == "" {
			continue
		}
		p := joinVendorToolFile(root, vendor, tool, fileName)
		if fileReadable(p) {
			return p, true
		}
	}
	return "", false
}

func joinVendorToolFile(root, vendor, tool, fileName string) string {
	if vendor != "" {
		return filepath.Join(root, vendor, tool, fileName)
	}
	return filepath.Join(root, tool, fileName)
}

// Resolve chooses one config file path.
//
// If explicitPath is non-empty, that path is the only candidate: missing or unreadable
// returns an error (no directory search fallback). On success found is true.
//
// If explicitPath is empty, vendor, tool, and fileName are required; directory search
// prefers user over system and returns at most one path. When neither has the file,
// found is false and err is nil (TOML layer may be skipped).
func Resolve(vendor, tool, fileName, explicitPath string) (path string, found bool, err error) {
	return ResolveWithSearch(vendor, tool, fileName, explicitPath, ConfigDirSearch{})
}

// ResolveWithSearch is like Resolve but uses the provided FileSearch for directory lookup.
func ResolveWithSearch(vendor, tool, fileName, explicitPath string, search FileSearch) (path string, found bool, err error) {
	explicitPath = strings.TrimSpace(explicitPath)
	if explicitPath != "" {
		return resolveExplicit(explicitPath)
	}
	vendor = strings.TrimSpace(vendor)
	tool = strings.TrimSpace(tool)
	fileName = strings.TrimSpace(fileName)
	if vendor == "" {
		return "", false, fmt.Errorf("configpath: vendor name is required")
	}
	if tool == "" {
		return "", false, fmt.Errorf("configpath: tool name is required")
	}
	if fileName == "" {
		return "", false, fmt.Errorf("configpath: file name is required")
	}
	if search == nil {
		search = ConfigDirSearch{}
	}
	p, ok := search.Find(vendor, tool, fileName)
	if !ok {
		return "", false, nil
	}
	return p, true, nil
}

func resolveExplicit(explicitPath string) (string, bool, error) {
	// Clean for stable returned path; existence is checked on the cleaned form.
	path := filepath.Clean(explicitPath)
	if !fileReadable(path) {
		return "", false, fmt.Errorf("configpath: --config-path %q is missing or unreadable", explicitPath)
	}
	return path, true, nil
}

func fileReadable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close()
	// Reject directories: config path must be a file.
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	return true
}
