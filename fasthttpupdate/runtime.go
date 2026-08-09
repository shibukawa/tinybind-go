package fasthttpupdate

import (
	"github.com/shibukawa/tinybind-go/internal/updatecore"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The runtime asset is the same bytes under the same URL on either transport:
// its digest is content-derived, so a deployment that serves one backend's
// asset and negotiates against the other still agrees about the path.

// RuntimeVersion is the content identity of the browser runtime. It appears in
// the runtime path so a new build gets a new URL.
func RuntimeVersion() string { return updatecore.RuntimeVersion() }

// RuntimeSource is the browser runtime this package implements, for a framework
// merging it into its own asset rather than loading a second one.
func RuntimeSource() []byte { return updatecore.RuntimeSource() }

// RuntimeAsset is the browser runtime as a static asset, for a caller that
// serves its own files.
func (o Options) RuntimeAsset() Asset { return o.core().RuntimeAsset() }

// RuntimeConfig is the configuration matching these options, so the server and
// the browser cannot disagree about a name. It carries no CSRF token; use
// RuntimeConfigFor to add one.
func (o Options) RuntimeConfig() RuntimeConfig { return o.core().RuntimeConfig() }

// RuntimeConfigFor is RuntimeConfig carrying this session's CSRF token, so the
// runtime sends it on every request it issues.
func (o Options) RuntimeConfigFor(csrfToken string) RuntimeConfig {
	return o.core().RuntimeConfigFor(csrfToken)
}

// RuntimePath is the URL the browser runtime is served from. The version
// segment makes the response immutable, which is why the handler may set a long
// max-age.
func (o Options) RuntimePath() string { return o.core().RuntimePath() }

// ScriptTag is the element loading the runtime, ready to place at the end of a
// document body. A caller owning the runtime gets an empty string.
func (o Options) ScriptTag() string { return o.core().ScriptTag() }

// ScriptTagFor is ScriptTag carrying this session's CSRF token, which is what a
// handler that renders forms uses.
func (o Options) ScriptTagFor(csrfToken string) string { return o.core().ScriptTagFor(csrfToken) }

// RuntimeHandler serves the browser runtime.
//
// A caller owning the runtime serves its own asset and never calls this; see
// Options.CallerOwnsRuntime and RuntimeSource.
func (o Options) RuntimeHandler() fasthttp.RequestHandler {
	asset := o.RuntimeAsset()
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.SetContentType(asset.ContentType)
		ctx.Response.Header.Set("ETag", `"`+asset.Version+`"`)
		ctx.Response.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
		// The asset is addressed by a digest of its own content, so the only
		// conditional request that can arrive is one already holding it.
		if match := string(ctx.Request.Header.Peek("If-None-Match")); match != "" &&
			updatecore.MatchesETag(match, `"`+asset.Version+`"`) {
			ctx.SetStatusCode(304)
			return
		}
		ctx.SetStatusCode(200)
		_, _ = ctx.Write(asset.Source)
	}
}

// Router is what Mount installs on.
//
// It names the one method Mount uses, so a framework with its own router passes
// it directly instead of losing the call. The forked router satisfies it.
type Router interface {
	Handle(method, path string, handler fasthttp.RequestHandler)
}

// Mount registers every endpoint this package owns under the configured path
// prefix, which is the runtime asset and nothing else.
//
// It takes no registry, because a redraw is not an endpoint this package
// mounts: the caller answers one from its own handler with Options.Redraw, at
// whatever URL it chooses.
//
// The asset is registered only when this build serves it; see
// Options.ServeRuntime.
func (o Options) Mount(router Router) {
	if o.core().ServesRuntime() {
		router.Handle("GET", o.core().PathNamespace()+"/runtime/{filepath:*}", o.RuntimeHandler())
	}
}
