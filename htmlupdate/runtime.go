package htmlupdate

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

//go:embed runtime.js
var runtimeSource string

// runtimeETag identifies the runtime bytes, so the asset URL can be immutably
// cacheable and a deploy still invalidates it.
var runtimeETag = func() string {
	sum := sha256.Sum256([]byte(runtimeSource))
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}()

// RuntimeVersion is the content identity of the browser runtime. It appears in
// the runtime path so a new build gets a new URL.
func RuntimeVersion() string { return runtimeETag }

// RuntimePath is the URL the browser runtime is served from. The version
// segment makes the response immutable, which is why the handler may set a long
// max-age.
func (o Options) RuntimePath() string {
	return o.pathPrefix() + "/runtime/tinybind." + runtimeETag + ".js"
}

// Mount registers every framework-owned endpoint under the configured path
// prefix. One call keeps the whole surface routable, cacheable, and protectable
// by a single rule.
func (o Options) Mount(mux *http.ServeMux, registry *Registry) {
	mux.Handle("GET "+o.pathPrefix()+"/runtime/", o.RuntimeHandler())
	if registry != nil {
		mux.Handle("GET "+o.pathPrefix()+"/redraw/", o.RedrawHandler(registry))
	}
}

// RuntimeHandler serves the browser runtime.
//
// The framework ships this asset rather than generating it, because the
// protocol it speaks is a framework constant. Serving it here keeps the first
// milestone free of the static asset pipeline.
func (o Options) RuntimeHandler() http.Handler {
	modified := time.Time{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("ETag", `"`+runtimeETag+`"`)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeContent(w, r, "tinybind.js", modified, strings.NewReader(runtimeSource))
	})
}

// ScriptTag is the element loading the runtime, ready to place at the end of a
// document body.
//
// The caller injects it, because this milestone has no document shell
// bootstrap. The tag carries the endpoint prefix, so one shared runtime asset
// works for any configured namespace without being rebuilt.
func (o Options) ScriptTag() string {
	return `<script src="` + htmlAttrEscape(o.RuntimePath()) +
		`" data-tinybind-prefix="` + htmlAttrEscape(o.pathPrefix()) +
		`" data-tinybind-build="` + htmlAttrEscape(o.buildID()) + `" defer></script>`
}

func htmlAttrEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", `"`, "&#34;", "'", "&#39;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(value)
}
