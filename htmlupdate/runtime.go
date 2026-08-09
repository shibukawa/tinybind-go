package htmlupdate

import (
	"net/http"
	"strings"
	"time"

	"github.com/shibukawa/tinybind-go/internal/updatecore"
)

// The runtime asset is transport-free everywhere except here: its bytes, its
// digest, its URL, and the script tag carrying its configuration are all
// computed in internal/updatecore, and this file is the net/http way to serve
// them. A second backend serves the same Asset from its own handler.

// RuntimeVersion is the content identity of the browser runtime. It appears in
// the runtime path so a new build gets a new URL.
func RuntimeVersion() string { return updatecore.RuntimeVersion() }

// RuntimeSource is the browser runtime this package implements.
//
// The bytes are the point. A framework that already ships a runtime cannot put
// two on one document — that would be two boundary id spaces, two build
// identities, and two script tags with nothing deciding which owns a region —
// so it merges ours into its own asset. Without readable bytes, merging means
// keeping a copy, and a copy is not a version-pinned dependency: it drifts on
// upgrade with nothing in the build failing, and a drifted browser runtime is a
// silently dead page rather than a compile error.
//
// The bytes carry no naming choice. The runtime reads its attribute prefix,
// header namespace, endpoint prefix, and installed name from the configuration
// it is given, so one asset serves every deployment and merging it needs no
// build step. RuntimeConfig produces that configuration; the file installs
// createPartialUpdateRuntime for a caller constructing an instance directly.
func RuntimeSource() []byte { return updatecore.RuntimeSource() }

// RuntimeAsset is the browser runtime as a static asset, for a caller that
// serves its own files.
func (o Options) RuntimeAsset() Asset { return o.core().RuntimeAsset() }

// RuntimeConfig is the configuration matching these options, so the server and
// the browser cannot disagree about a name.
//
// It carries no CSRF token: a token belongs to a session and these options
// belong to the process. Use RuntimeConfigFor to add one.
func (o Options) RuntimeConfig() RuntimeConfig { return o.core().RuntimeConfig() }

// RuntimeConfigFor is RuntimeConfig carrying this session's CSRF token, so the
// runtime sends it on every request it issues.
//
// The header is the channel for anything the runtime fetches; the hidden field
// generated into each form is the channel for a submission with no script. They
// carry the same value, which is what a header carrying exactly one value
// requires of the token: one per session.
func (o Options) RuntimeConfigFor(csrfToken string) RuntimeConfig {
	return o.core().RuntimeConfigFor(csrfToken)
}

// RuntimePath is the URL the browser runtime is served from. The version
// segment makes the response immutable, which is why the handler may set a long
// max-age.
func (o Options) RuntimePath() string { return o.core().RuntimePath() }

// ScriptTag is the element loading the runtime, ready to place at the end of a
// document body.
//
// The caller injects it, because this milestone has no document shell
// bootstrap. The tag carries the whole runtime configuration, so one shared
// asset works for any set of names without being rebuilt.
//
// A caller owning the runtime gets an empty string: a tag pointing at an asset
// this build does not serve is worse than no tag at all.
func (o Options) ScriptTag() string { return o.core().ScriptTag() }

// ScriptTagFor is ScriptTag carrying this session's CSRF token, which is what a
// handler that renders forms uses.
func (o Options) ScriptTagFor(csrfToken string) string { return o.core().ScriptTagFor(csrfToken) }

// RuntimeHandler serves the browser runtime.
//
// A caller owning the runtime serves its own asset and never calls this; see
// Options.CallerOwnsRuntime and RuntimeSource.
func (o Options) RuntimeHandler() http.Handler {
	modified := time.Time{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", updatecore.RuntimeContentType)
		w.Header().Set("ETag", `"`+updatecore.RuntimeVersion()+`"`)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeContent(w, r, o.core().RuntimeBaseName()+".js", modified, strings.NewReader(string(updatecore.RuntimeSource())))
	})
}

// Router is what Mount installs on.
//
// It names the one method Mount uses, so a framework with its own router passes
// it directly instead of losing the call. *http.ServeMux satisfies it, which is
// what keeps every existing call site compiling: naming a concrete type here
// made the convenience uncallable from exactly the callers who most wanted the
// whole surface installed by one rule.
type Router interface {
	Handle(pattern string, handler http.Handler)
}

// Mount registers every endpoint this package owns under the configured path
// prefix, which is now the runtime asset and nothing else.
//
// It takes no registry, because a redraw is no longer an endpoint this package
// mounts: the caller answers one from its own handler with Options.Redraw, at
// whatever URL it chooses. That is the whole point of the change — an endpoint
// the caller routes, protects, and logs should have an address the caller picked.
//
// The asset is registered only when this build serves it; see
// Options.ServeRuntime.
func (o Options) Mount(router Router) {
	if o.core().ServesRuntime() {
		router.Handle("GET "+o.core().PathNamespace()+"/runtime/", o.RuntimeHandler())
	}
}
