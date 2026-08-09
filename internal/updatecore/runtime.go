package updatecore

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"strings"
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
		ContentType: RuntimeContentType,
		FileName:    o.RuntimeBaseName() + "." + runtimeETag + ".js",
	}
}

const RuntimeContentType = "text/javascript; charset=utf-8"

// DefaultRuntimeFileName names the served runtime file.
const DefaultRuntimeFileName = "tinybind"

func (o Options) RuntimeBaseName() string {
	if o.RuntimeFileName == "" {
		return DefaultRuntimeFileName
	}
	return o.RuntimeFileName
}

func (o Options) ServesRuntime() bool { return o.ServeRuntime && !o.CallerOwnsRuntime }

// RuntimeConfig is what the browser runtime reads to learn its own names.
//
// It is exported because a framework merging the runtime into its own asset
// builds the same object and passes it to the factory directly, rather than
// reproducing the field names from the JavaScript.
type RuntimeConfig struct {
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
	// CSRFHeader is the header the runtime puts the token in. It is not derived
	// from Header, because X-CSRF-Token is a name every framework already
	// recognizes rather than one this module owns.
	CSRFHeader string `json:"csrfHeader,omitempty"`
	// CSRF is the session's token. It is empty for a deployment that turned the
	// token off, and then the runtime sends no header, so a page without one is
	// byte-identical to what it was before this existed.
	//
	// A token in a data attribute is readable by script, which is the same
	// exposure the hidden field in every form already has. It is not what
	// protects against XSS: script that runs in the page can act as the user
	// whether or not it can read this.
	CSRF string `json:"csrf,omitempty"`
}

// RuntimeConfig is the configuration matching these options, so the server and
// the browser cannot disagree about a name.
//
// It carries no CSRF token: a token belongs to a session and these options
// belong to the process. Use RuntimeConfigFor to add one.
func (o Options) RuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Build:      o.Build(),
		Attr:       o.AttributePrefix(),
		Header:     o.HeaderNamespace(),
		Global:     o.Global(),
		CSRFHeader: o.CSRFHeader(),
	}
}

// RuntimeConfigFor is RuntimeConfig carrying this session's CSRF token, so the
// runtime sends it on every request it issues.
//
// The header is the channel for anything the runtime fetches; the hidden field
// generated into each form is the channel for a submission with no script. They
// carry the same value, which is what a header carrying exactly one value
// requires of the token: one per session.
func (o Options) RuntimeConfigFor(csrfToken string) RuntimeConfig {
	config := o.RuntimeConfig()
	config.CSRF = csrfToken
	return config
}

// DefaultCSRFHeaderName is where the runtime puts the token. Unlike the render
// and manifest headers it does not follow HeaderPrefix, because this one is a
// name middleware already looks for rather than a namespace this module owns.
const DefaultCSRFHeaderName = "X-CSRF-Token"

func (o Options) CSRFHeader() string {
	if o.CSRFHeaderName == "" {
		return DefaultCSRFHeaderName
	}
	return o.CSRFHeaderName
}

// RuntimePath is the URL the browser runtime is served from. The version
// segment makes the response immutable, which is why the handler may set a long
// max-age.
func (o Options) RuntimePath() string {
	return o.PathNamespace() + "/runtime/" + o.RuntimeBaseName() + "." + runtimeETag + ".js"
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
func (o Options) ScriptTag() string { return o.scriptTag(o.RuntimeConfig()) }

// ScriptTagFor is ScriptTag carrying this session's CSRF token, which is what a
// handler that renders forms uses.
func (o Options) ScriptTagFor(csrfToken string) string {
	return o.scriptTag(o.RuntimeConfigFor(csrfToken))
}

func (o Options) scriptTag(config RuntimeConfig) string {
	if !o.ServesRuntime() {
		return ""
	}
	encoded, err := json.Marshal(config)
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
