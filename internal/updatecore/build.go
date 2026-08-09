package updatecore

import (
	"crypto/rand"
	"encoding/base64"
	"runtime/debug"
	"sync"
)

// BuildID identifies the running binary.
//
// It is the third and last identity in this design, and it does the job the
// other two deliberately do not:
//
//   - the protocol version names the wire contract, and must stay stable across
//     builds or every deploy would make an already-loaded page incompatible
//   - a component kind names a component, and must stay stable so an unrelated
//     deploy does not invalidate its endpoint
//   - the build id names this binary, so anything that could change rendering
//     invalidates client state: a template, a Go function a template calls, the
//     render runtime itself, or a dependency
//
// A component kind cannot do this job. It hashes one component's own compiled
// plan, so it misses a change in a component that one calls, in an external
// function, and in the framework's own rendering.
//
// The value comes from the version control revision the binary was stamped
// with. A binary built from a dirty tree, or with no stamping at all, gets a
// value unique to the process instead: during development every restart should
// invalidate, and guessing otherwise would serve stale regions while editing.
var BuildID = sync.OnceValue(func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		var revision string
		var modified bool
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value == "true"
			}
		}
		if revision != "" && !modified {
			return revision[:min(len(revision), 16)]
		}
	}
	return processID()
})

// processID is the fallback identity: unique per process, so a development
// restart invalidates and a clean production build does not.
func processID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// A source of randomness that fails leaves nothing better than a
		// constant, and a constant is the unsafe direction, so fail loudly.
		panic("htmlupdate: cannot derive a build identity: " + err.Error())
	}
	return "dev-" + base64.RawURLEncoding.EncodeToString(raw[:])
}

// Build returns the configured identity, or the running binary's.
func (o Options) Build() string {
	if o.BuildID != "" {
		return o.BuildID
	}
	return BuildID()
}
