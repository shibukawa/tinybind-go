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

// Mode is the rendering a request asked for.
type Mode int

const (
	// ModeDocument is the complete HTML document. It is what a request without
	// a usable render header gets, including one from an incompatible client.
	ModeDocument Mode = iota
	// ModeNavigation returns only the changed boundaries of the same route.
	ModeNavigation
)

const modeNavigation = "navigation"

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
	// DefaultHeaderPrefix. A deployment overriding it needs a browser runtime
	// built for the same prefix, because the runtime hardcodes the names.
	HeaderPrefix string
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
	mode, version, ok := parseRender(r.Header.Get(o.renderHeader()))
	if !ok || mode != modeNavigation || version != Version {
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
	return Negotiated{Mode: ModeNavigation, Version: version, Known: DecodeManifest(encoded)}
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
	if negotiated.Mode == ModeNavigation {
		return renderDelta(w, o, negotiated, wrappers, leaf)
	}
	// The document render collects so every boundary carries its instance
	// attribute; without them a later delta could not find its targets.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := htmlbind.CollectChain(w, o.Key, wrappers, leaf)
	return err
}

func renderDelta(w http.ResponseWriter, o Options, negotiated Negotiated, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) error {
	delta, err := htmlbind.RenderDelta(o.Key, negotiated.Known, wrappers, leaf)
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// A delta carries per-document validators, so it is never shareable.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(o.renderHeader(), modeNavigation+";v="+versionText)
	return encodeJSON(w, body)
}
