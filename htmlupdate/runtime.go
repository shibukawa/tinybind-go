package htmlupdate

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
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
func RuntimeSource() []byte { return []byte(runtimeSource) }

// Asset is one static file this package requires a page to load.
//
// The module decides what the bytes are and what identifies them; the caller
// decides where they are served, under what name, and with what cache policy.
type Asset struct {
	// Source is the file's content.
	Source []byte
	// Version is the content digest. Two builds with the same digest are the
	// same file, and a changed digest is a changed URL.
	Version string
	// ContentType is the media type the file must be served as.
	ContentType string
	// FileName is the name this package would serve it under, which a caller
	// serving it elsewhere may ignore.
	FileName string
}

// RuntimeAsset is the browser runtime as a static asset, for a caller that
// serves its own files.
func (o Options) RuntimeAsset() Asset {
	return Asset{
		Source:      RuntimeSource(),
		Version:     runtimeETag,
		ContentType: runtimeContentType,
		FileName:    o.runtimeFileName() + "." + runtimeETag + ".js",
	}
}

const runtimeContentType = "text/javascript; charset=utf-8"

// DefaultRuntimeFileName names the served runtime file.
const DefaultRuntimeFileName = "tinybind"

func (o Options) runtimeFileName() string {
	if o.RuntimeFileName == "" {
		return DefaultRuntimeFileName
	}
	return o.RuntimeFileName
}

func (o Options) serveRuntime() bool { return !o.CallerOwnsRuntime }

// RuntimeConfig is what the browser runtime reads to learn its own names.
//
// It is exported because a framework merging the runtime into its own asset
// builds the same object and passes it to the factory directly, rather than
// reproducing the field names from the JavaScript.
type RuntimeConfig struct {
	// Prefix is the URL namespace of every framework-owned endpoint.
	Prefix string `json:"prefix"`
	// Build is the identity of the binary that rendered the page.
	Build string `json:"build"`
	// Attr is the data-attribute prefix, which names the instance attribute the
	// runtime locates by and the preserve and ignore markers authors write.
	Attr string `json:"attr"`
	// Header is the header namespace the render, manifest, and build headers
	// are derived from.
	Header string `json:"header"`
	// Global is the name the runtime instance is installed under. Empty
	// installs nothing, which is what a caller using only the factory wants.
	Global string `json:"global"`
}

// RuntimeConfig is the configuration matching these options, so the server and
// the browser cannot disagree about a name.
func (o Options) RuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Prefix: o.pathPrefix(),
		Build:  o.buildID(),
		Attr:   o.dataAttributePrefix(),
		Header: o.prefix(),
		Global: o.globalName(),
	}
}

// RuntimePath is the URL the browser runtime is served from. The version
// segment makes the response immutable, which is why the handler may set a long
// max-age.
func (o Options) RuntimePath() string {
	return o.pathPrefix() + "/runtime/" + o.runtimeFileName() + "." + runtimeETag + ".js"
}

// RuntimeHandler serves the browser runtime.
//
// A caller owning the runtime serves its own asset and never calls this; see
// Options.CallerOwnsRuntime and RuntimeSource.
func (o Options) RuntimeHandler() http.Handler {
	modified := time.Time{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", runtimeContentType)
		w.Header().Set("ETag", `"`+runtimeETag+`"`)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeContent(w, r, o.runtimeFileName()+".js", modified, strings.NewReader(runtimeSource))
	})
}

// ScriptTag is the element loading the runtime, ready to place at the end of a
// document body.
//
// The caller injects it, because this milestone has no document shell
// bootstrap. The tag carries the whole runtime configuration, so one shared
// asset works for any set of names without being rebuilt.
//
// A caller owning the runtime gets an empty string: a tag pointing at an asset
// this build does not serve is worse than no tag at all.
func (o Options) ScriptTag() string {
	if !o.serveRuntime() {
		return ""
	}
	encoded, err := json.Marshal(o.RuntimeConfig())
	if err != nil {
		// Every field is a string, so this cannot fail; a nil config would
		// silently disable updates, which is the one outcome worth a panic.
		panic("htmlupdate: cannot encode the runtime configuration: " + err.Error())
	}
	return `<script src="` + htmlAttrEscape(o.RuntimePath()) +
		`" data-config="` + htmlAttrEscape(string(encoded)) + `" defer></script>`
}

func htmlAttrEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", `"`, "&#34;", "'", "&#39;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(value)
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

// Mount registers every framework-owned endpoint under the configured path
// prefix. One call keeps the whole surface routable, cacheable, and protectable
// by a single rule.
//
// The runtime asset is registered only when this build serves it; see
// Options.CallerOwnsRuntime.
func (o Options) Mount(router Router, registry *Registry) {
	if o.serveRuntime() {
		router.Handle("GET "+o.pathPrefix()+"/runtime/", o.RuntimeHandler())
	}
	if registry != nil {
		router.Handle("GET "+o.pathPrefix()+"/redraw/", o.RedrawHandler(registry))
	}
}
