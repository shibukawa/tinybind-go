// Package htmlupdate serves HTML templates that can update themselves in place.
//
// One URL answers two ways. Without the render header a request gets the
// ordinary complete document, so a browser without the runtime, a crawler, and
// curl are all unaffected. With it, the response carries only the boundaries
// whose markup actually changed.
//
// The transport concerns live here rather than in htmlbind, because that
// package stays free of net/http so generated template code keeps working on
// TinyGo and WebAssembly targets.
package htmlupdate

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Version is the wire contract this package speaks.
const Version = htmlbind.ProtocolVersion

// DefaultHeaderPrefix names the request and response headers.
const DefaultHeaderPrefix = "X-Tinybind"

// DefaultPathPrefix is the URL namespace holding every framework-owned
// endpoint. Keeping them under one prefix means a deployment can route, cache,
// or protect the whole surface with one rule.
const DefaultPathPrefix = "/_tb"

// DefaultDataAttributePrefix names the attributes the protocol puts in a
// document. It matches the generator's own default, because the two have to
// agree: one writes the attributes and the other reads them.
const DefaultDataAttributePrefix = "tb"

// DefaultGlobalName is what the browser runtime is installed under.
const DefaultGlobalName = "tinybind"

// Mode is the rendering a request asked for.
type Mode int

const (
	// ModeDocument is the complete HTML document. It is what a request without
	// a usable render header gets, including one from an incompatible client.
	ModeDocument Mode = iota
	// ModeNavigation returns only the changed boundaries of the same route.
	ModeNavigation
	// ModeLive returns the deliveries of the same route's live boundaries, on a
	// response held open for as long as the subscriptions live.
	//
	// It is its own mode rather than a navigation held open because the two
	// differ in duration and in termination: a navigation ends when the route has
	// been described, a live response ends when every source finishes or when the
	// server reaches a lifetime bound. Sharing one name meant a deployment could
	// not route, time out, or bound them separately, and a served-mode log could
	// not tell an hours-long subscription from ordinary navigation traffic.
	ModeLive
)

const (
	modeNavigation = "navigation"
	modeLive       = "live"
)

// Options configure one set of update endpoints.
type Options struct {
	// Key authenticates validators. Two renders that are to be compared must
	// use the same key; rotating it forces complete documents, which is the
	// intended effect of a rotation.
	//
	// An unkeyed digest of low entropy content lets anyone confirm a guess by
	// comparing digests, so a deployment serving non-public pages must set it.
	Key []byte
	// HeaderPrefix overrides the header namespace. Empty uses
	// DefaultHeaderPrefix. The runtime reads it from its configuration, so
	// overriding it needs no rebuilt runtime.
	HeaderPrefix string
	// DataAttributePrefix overrides the attribute namespace. Empty uses
	// DefaultDataAttributePrefix.
	//
	// It must match the generator's own DataAttributePrefix, because that is
	// what wrote the instance attributes into the markup this runtime reads.
	// It also names the preserve and ignore markers an application author
	// writes by hand, which is why a framework needs to own it: those are the
	// author's surface, not a wire detail.
	DataAttributePrefix string
	// GlobalName overrides the name the browser runtime is installed under.
	// Empty uses DefaultGlobalName.
	//
	// A framework sets it so its users call the framework's own name rather
	// than a dependency's.
	GlobalName string
	// RuntimeFileName overrides the base name of the served runtime asset.
	// Empty uses DefaultRuntimeFileName. The content digest and the .js suffix
	// are appended either way, so the URL stays immutably cacheable.
	RuntimeFileName string
	// CallerOwnsRuntime stops this package serving or referencing a browser
	// runtime. Mount registers no asset route and ScriptTag returns nothing.
	//
	// Set it when the caller ships its own runtime — usually ours, merged into
	// a larger asset from RuntimeSource. Two runtimes on one document would
	// mean two boundary id spaces and two build identities, so a framework that
	// already has one takes this rather than adding a second.
	CallerOwnsRuntime bool
	// CSRFFieldName is the hidden field generated forms carry. Empty uses
	// DefaultCSRFFieldName.
	//
	// It must match the generator's own CSRFFieldName, because that is what
	// wrote the field into the markup this reads back: one writes it and the
	// other reads it, and nothing links the two at compile time.
	CSRFFieldName string
	// CSRFHeaderName overrides the header the browser runtime puts the CSRF
	// token in. Empty uses DefaultCSRFHeaderName.
	//
	// It does not follow HeaderPrefix on purpose: the render, manifest, and
	// build headers name this protocol, and this one names a convention every
	// framework's middleware already looks for.
	CSRFHeaderName string
	// PathPrefix overrides the URL namespace of every framework endpoint.
	// Empty uses DefaultPathPrefix. Unlike the header names, the runtime learns
	// this one at load time, so overriding it needs no rebuilt runtime.
	PathPrefix string
	// BuildID overrides the identity of the running binary. Empty uses
	// BuildID(), which is the version control revision the binary was stamped
	// with, or a per-process value when the tree was dirty or unstamped.
	//
	// A page rendered by a different build has client state this binary cannot
	// vouch for, so it is served a complete document instead of a delta.
	BuildID string
	// MaxManifestBytes caps the manifest header a request may carry. Zero uses
	// DefaultMaxManifestBytes. An oversized manifest is ignored rather than
	// rejected, so the response is a larger delta instead of an error.
	MaxManifestBytes int
	// MaxQueryBytes caps the arguments a redraw may carry. Zero uses
	// DefaultMaxQueryBytes. Unlike an oversized manifest an oversized query is
	// rejected, because the arguments are the request rather than a hint.
	MaxQueryBytes int
	// RedrawCacheControl overrides the cache policy of a redraw response. Empty
	// uses DefaultRedrawCacheControl.
	//
	// A caller relaxing it takes responsibility for what a redraw renders: the
	// arguments come from the browser, so the component authorizes its own
	// inputs, and a cache keyed on the URL alone would serve one user's render
	// to another.
	RedrawCacheControl string
	// StreamContentType overrides the media type of a streamed delta. Empty
	// uses DefaultStreamContentType.
	//
	// This one names the wire format rather than a limit, so overriding it is a
	// framing choice a client has to agree with, not a tuning knob.
	StreamContentType string
	// OnFailure receives every request an endpoint of this package could not
	// answer, and writes the response for it. Nil writes the plain-text
	// response WriteFailure writes.
	//
	// This package owns the endpoint, so it has to write something; it does
	// not have to decide what a failure looks like. A caller with problem
	// responses, its own error pages, a request-scoped logger, or a tracer
	// takes the whole Failure and answers however it answers everything else.
	//
	// A hook must write a response, exactly as a handler must. Delegating to
	// WriteFailure after logging is the cheapest way to keep the default body.
	OnFailure func(w http.ResponseWriter, r *http.Request, failure Failure)
}

// DefaultMaxManifestBytes bounds the validators a request may carry. Beyond it
// the hints are dropped, which costs bytes in the response instead of risking a
// proxy rejecting the request.
const DefaultMaxManifestBytes = 8 << 10

func (o Options) renderHeader() string { return o.prefix() + "-Render" }

func (o Options) manifestHeader() string { return o.prefix() + "-Manifest" }

func (o Options) buildHeader() string { return o.prefix() + "-Build" }

func (o Options) prefix() string {
	if o.HeaderPrefix == "" {
		return DefaultHeaderPrefix
	}
	return o.HeaderPrefix
}

// pathPrefix returns the endpoint namespace without a trailing slash.
func (o Options) pathPrefix() string {
	if o.PathPrefix == "" {
		return DefaultPathPrefix
	}
	return "/" + strings.Trim(o.PathPrefix, "/")
}

func (o Options) maxManifestBytes() int {
	if o.MaxManifestBytes == 0 {
		return DefaultMaxManifestBytes
	}
	return o.MaxManifestBytes
}

// renderOptions carries the naming these options configure into every htmlbind
// entry this package drives, so the placeholder element, the boundary
// identifiers, and the instance attributes are one naming system rather than
// two. Caller options follow, so a caller can still override.
func (o Options) renderOptions(caller []htmlbind.Option) []htmlbind.Option {
	owned := []htmlbind.Option{htmlbind.WithBoundaryPrefix(o.dataAttributePrefix())}
	return append(owned, caller...)
}

func (o Options) dataAttributePrefix() string {
	if o.DataAttributePrefix == "" {
		return DefaultDataAttributePrefix
	}
	return o.DataAttributePrefix
}

func (o Options) globalName() string {
	if o.GlobalName == "" {
		return DefaultGlobalName
	}
	return o.GlobalName
}

func (o Options) maxQueryBytes() int {
	if o.MaxQueryBytes == 0 {
		return DefaultMaxQueryBytes
	}
	return o.MaxQueryBytes
}

func (o Options) streamContentType() string {
	if o.StreamContentType == "" {
		return DefaultStreamContentType
	}
	return o.StreamContentType
}

// Negotiated is what a request asked for, after validation.
type Negotiated struct {
	Mode Mode
	// Version is the protocol version the client claims. It equals Version
	// whenever Mode is not ModeDocument.
	Version int
	// Known holds the validators the client already has. It is empty on a
	// client's first update, which simply yields a larger delta.
	Known htmlbind.Manifest
}

// Negotiate resolves how a request must be answered.
//
// Anything unrecognized resolves to ModeDocument rather than to an error: a
// stale client, a truncated header, a proxy that dropped a header, and a
// version bump must all still produce a working page.
func (o Options) Negotiate(r *http.Request) Negotiated {
	name, version, ok := parseRender(r.Header.Get(o.renderHeader()))
	if !ok || version != Version {
		return Negotiated{Mode: ModeDocument}
	}
	var mode Mode
	switch name {
	case modeNavigation:
		mode = ModeNavigation
	case modeLive:
		mode = ModeLive
	default:
		return Negotiated{Mode: ModeDocument}
	}
	// A render request must stay side-effect free, which is also why it needs
	// no CSRF token: a GET that changes nothing cannot be forged into an
	// action. A non-GET arriving in this mode is a client error, not a delta.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return Negotiated{Mode: ModeDocument}
	}
	// A page rendered by another build holds state this binary cannot vouch
	// for: a template it does not have, a function that behaves differently, a
	// runtime that renders differently. None of that is visible in a validator,
	// so the build is compared instead of guessed at.
	if r.Header.Get(o.buildHeader()) != o.buildID() {
		return Negotiated{Mode: ModeDocument}
	}
	encoded := r.Header.Get(o.manifestHeader())
	if len(encoded) > o.maxManifestBytes() {
		encoded = ""
	}
	return Negotiated{Mode: mode, Version: version, Known: DecodeManifest(encoded)}
}

// renderToken is the value a response echoes for one mode.
func renderToken(mode Mode) string {
	if mode == ModeLive {
		return modeLive + ";v=" + versionText
	}
	return modeNavigation + ";v=" + versionText
}

// parseRender reads a render header of the form "navigation;v=1".
func parseRender(value string) (mode string, version int, ok bool) {
	name, rest, found := strings.Cut(value, ";")
	if !found {
		return "", 0, false
	}
	digits, found := strings.CutPrefix(strings.TrimSpace(rest), "v=")
	if !found {
		return "", 0, false
	}
	version, err := strconv.Atoi(digits)
	if err != nil {
		return "", 0, false
	}
	return strings.TrimSpace(name), version, true
}

// DecodeManifest reads the compact validator list a client sends back. The
// encoding is "id:validator" pairs separated by commas, which stays inside one
// header and needs no escaping because both halves are opaque tokens.
func DecodeManifest(encoded string) htmlbind.Manifest {
	var manifest htmlbind.Manifest
	for _, pair := range strings.Split(encoded, ",") {
		id, validator, found := strings.Cut(pair, ":")
		if !found || id == "" || validator == "" {
			continue
		}
		manifest.Instances = append(manifest.Instances, htmlbind.Instance{
			ID: id, FrameValidator: validator,
		})
	}
	return manifest
}

// EncodeManifest renders the validator list a client sends back. It exists so a
// test, and any non-browser client, can produce exactly what the runtime does.
func EncodeManifest(manifest htmlbind.Manifest) string {
	var out strings.Builder
	for _, instance := range manifest.Instances {
		if out.Len() > 0 {
			out.WriteByte(',')
		}
		out.WriteString(instance.ID)
		out.WriteByte(':')
		out.WriteString(instance.FrameValidator)
	}
	return out.String()
}

// deltaResponse is the JSON body of a navigation or action response.
type deltaResponse struct {
	Version    int              `json:"v"`
	Operations []deltaOperation `json:"ops"`
	Manifest   []deltaInstance  `json:"manifest,omitempty"`
	// Head is the merged head of the new composition, sent so the client can
	// install what a newly reachable component contributed before its markup
	// lands and flashes unstyled.
	Head []string `json:"head,omitempty"`
	// Navigate asks the browser to leave the page, which an action uses when
	// it changed where the user belongs.
	Navigate string `json:"navigate,omitempty"`
	// Live says the composition this response describes owns a live boundary, so
	// a client that applied it should open a live request. It is the handoff
	// marker of rule:stream-termination-marker on the buffered path.
	//
	// Absent means no live boundary, and a page that has none is what it was
	// before this field existed: a client that reads no marker issues no
	// speculative request and costs the server no page execution.
	Live bool `json:"live,omitempty"`
}

// versionText is the protocol version as it appears in a header value.
var versionText = strconv.Itoa(Version)

func encodeJSON(w http.ResponseWriter, body deltaResponse) error {
	return json.NewEncoder(w).Encode(body)
}

type deltaOperation struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	HTML string `json:"html"`
}

type deltaInstance struct {
	ID    string `json:"id"`
	Frame string `json:"frame"`
}

// Render answers one request with either a complete document or a delta.
//
// It always sets Vary, because a cache that served a delta body to a document
// request would hand a browser a page of JSON. The caller keeps every other
// response concern, as elsewhere in this module.
func (o Options) Render(w http.ResponseWriter, r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) error {
	w.Header().Add("Vary", o.renderHeader())
	w.Header().Add("Vary", o.buildHeader())
	negotiated := o.Negotiate(r)
	o.markLive(w, wrappers, leaf)
	if negotiated.Mode == ModeNavigation {
		return renderDelta(w, o, negotiated, wrappers, leaf)
	}
	// This entry buffers, so it cannot hold a delivery stream open. A live
	// request reaching it is answered with the document, which is the same
	// fallback every unrecognized condition takes and leaves the client with a
	// working page rather than an error.
	//
	// The document render collects so every boundary carries its instance
	// attribute; without them a later delta could not find its targets.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := htmlbind.CollectChain(w, o.Key, wrappers, leaf, o.renderOptions(nil)...)
	return err
}

// liveHeader names the response header saying whether this composition owns a
// live boundary.
func (o Options) liveHeader() string { return o.prefix() + "-Live" }

// headHeader names the response header carrying a redraw's head contribution.
// Every other mode has a body a head field fits into; a redraw's body is the
// component's markup, and wrapping it would cost the plain-fragment property
// that makes the endpoint testable with curl.
func (o Options) headHeader() string { return o.prefix() + "-Head" }

// markLive writes the handoff marker for a chain that owns a live boundary, so
// a client knows whether a live request is worth issuing at all.
//
// A live request re-executes the route, its layouts, and its page, so a client
// that cannot tell a live page from a static one pays a full page execution per
// screen that never had a live boundary. The marker is therefore a cost control
// rather than tidiness, and it is written on every mode: a browser loading the
// document reads the header, and a client that arrived by delta reads the body
// field, because a delta reuses the shell and never sees this response's head.
//
// Nothing is written when the chain owns no live boundary, so a page that had
// none is byte-identical to what it was before the marker existed.
func (o Options) markLive(w http.ResponseWriter, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) {
	if htmlbind.HasLiveBlock(wrappers, leaf) {
		w.Header().Set(o.liveHeader(), "1")
	}
}

func renderDelta(w http.ResponseWriter, o Options, negotiated Negotiated, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) error {
	delta, err := htmlbind.RenderDelta(o.Key, negotiated.Known, wrappers, leaf, o.renderOptions(nil)...)
	if err != nil {
		// Nothing has been written yet, so the caller can still choose a status
		// and serve an ordinary error page.
		return err
	}
	body := deltaResponse{Version: Version}
	for _, operation := range delta.Operations {
		body.Operations = append(body.Operations, deltaOperation{
			Kind: operation.Kind, ID: operation.InstanceID, HTML: operation.HTML,
		})
	}
	for _, instance := range delta.Manifest.Instances {
		body.Manifest = append(body.Manifest, deltaInstance{ID: instance.ID, Frame: instance.FrameValidator})
	}
	body.Head = delta.Head
	// A navigation can arrive at a route whose composition owns a live boundary,
	// and the client reused its document shell, so this body is the only place
	// that can tell it so.
	body.Live = htmlbind.HasLiveBlock(wrappers, leaf)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// A delta carries per-document validators, so it is never shareable.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(o.renderHeader(), renderToken(ModeNavigation))
	return encodeJSON(w, body)
}
