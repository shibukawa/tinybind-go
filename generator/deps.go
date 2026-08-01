package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// A reference hook transform reads authored files that no generation phase
// otherwise looks at: the image a template points at, and every module a
// TypeScript entry point imports. None of them is a hashed input, so without
// what follows, editing one would change nothing the fingerprint covers and the
// next run would skip while shipping stale output.
//
// They cannot join the fingerprint directly, because the fingerprint decides
// whether the run happens and the read set is only known once it has. So the
// run records what it read, and the next run verifies that record before
// trusting its own skip. Hashing every file under an asset directory instead
// would make an unrelated upload force a regeneration, which is why the record
// is of what was actually read.

// depsFileName holds the read set of the last run. It exists only when a
// transform reported reading something, so a project registering no hook gains
// no file.
const depsFileName = "tinybind_deps_gen.json"

// depsVersion is bumped when the record's shape changes, so an older file is
// ignored rather than misread. An ignored record regenerates.
const depsVersion = 1

type depsRecord struct {
	Version int `json:"version"`
	// Reads maps each file a transform reported reading to the hex SHA-256 of
	// its bytes at the time it was read.
	Reads map[string]string `json:"reads"`
}

// writeDeps records the read set beside the generated Go files, or removes a
// stale record when this run read nothing.
//
// A path is stored exactly as the transform reported it and re-read the same
// way, so a transform should report paths it can read again from the same
// working directory; an absolute path always can.
func writeDeps(outDir string, readSet []string) error {
	path := filepath.Join(outDir, depsFileName)
	if len(readSet) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	record := depsRecord{Version: depsVersion, Reads: make(map[string]string, len(readSet))}
	for _, name := range readSet {
		digest, err := fileDigest(name)
		if err != nil {
			// A file that cannot be read now cannot be compared later, so
			// recording nothing is what makes the next run regenerate instead
			// of trusting an unverifiable entry.
			return nil
		}
		record.Reads[name] = digest
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

// depsUnchanged reports whether every file the previous run read still hashes
// to what that run recorded.
//
// Anything it cannot fully verify reports false, so an unreadable, malformed,
// or older record regenerates rather than being trusted.
func depsUnchanged(outDir string) bool {
	source, err := os.ReadFile(filepath.Join(outDir, depsFileName))
	if err != nil {
		// No record means either no hook read anything or the file is gone. The
		// first is the ordinary case and must stay skippable; the second is
		// covered by the record being an output the run rewrites.
		return os.IsNotExist(err)
	}
	var record depsRecord
	if err := json.Unmarshal(source, &record); err != nil || record.Version != depsVersion {
		return false
	}
	for name, recorded := range record.Reads {
		digest, err := fileDigest(name)
		if err != nil || digest != recorded {
			return false
		}
	}
	return true
}

func fileDigest(path string) (string, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return contentHash(source), nil
}

// depsPath reports where the record lives, so it joins the paths a run declares
// rather than appearing behind it.
func depsPath(outDir string, readSet []string) string {
	if len(readSet) == 0 {
		return ""
	}
	path := filepath.Join(outDir, depsFileName)
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
