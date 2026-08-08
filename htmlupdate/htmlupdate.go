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
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

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
	// ModeRedraw returns one registered component's subtree, addressed by the
	// kind and instance headers rather than by the URL path.
	//
	// It is a request mode so a caller can answer a redraw at any URL it likes.
	// Usually that is the page the component sits on, where the redraw inherits
	// the page's own authorization rather than needing a second path pattern kept
	// in step with the one protecting the page — two rules that must agree and
	// that nothing forces to agree.
	ModeRedraw
	// ModeSequence returns the static half of one fragment, addressed by a
	// digest of its own content.
	//
	// It is the one response in this package that is not per user: a sequence
	// derives from the template rather than from a request, so it is the only
	// one that can be public, immutable, and held by a shared cache.
	ModeSequence
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
	// CallerOwnsRuntime says the caller ships its own browser runtime, so this
	// package serves and references none: Mount registers no asset route and
	// ScriptTag returns nothing.
	//
	// Usually that runtime is this one, merged into a larger asset from
	// RuntimeSource. Two runtimes on one document would mean two boundary id
	// spaces and two build identities, so a framework that already has one takes
	// this rather than adding a second.
	CallerOwnsRuntime bool
	// ServeRuntime asks this package to serve the reference browser runtime at a
	// content-hashed URL and to write the script tag that loads it.
	//
	// It is off by default, which is the whole of the difference from earlier
	// versions: the browser half belongs to the caller, so serving one is
	// something a deployment asks for rather than something it inherits.
	//
	// Exactly one of this and CallerOwnsRuntime must be set, and Validate says so
	// at startup. A build that set neither would compile and then serve pages
	// that silently stop updating, which is the worst failure shape available
	// here: nothing fails, the page is just quietly dead.
	ServeRuntime bool
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
	// SequenceCacheControl overrides the cache policy of a sequence response.
	// Empty uses DefaultSequenceCacheControl, which keeps it forever because the
	// address is a digest of the body.
	SequenceCacheControl string
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
// sequenceHeader names the request header a client sets to say it can walk a
// sequence tree, so a response may send values instead of markup.
//
// It is a capability rather than a list of held addresses: the choice between a
// fragment and its values is a heuristic, since values a client cannot resolve
// cost it one fetch and a fragment where values would have done costs a few
// bytes. Neither is wrong, so no per-address bookkeeping travels.
func (o Options) sequenceHeader() string { return o.prefix() + "-Sequences" }

// wantsSequences reports whether this request said it can walk sequences.
func (o Options) wantsSequences(r *http.Request) bool {
	return r != nil && r.Header.Get(o.sequenceHeader()) != ""
}

// operationBody writes one operation, in whichever half the client can use.
func (o Options) operationBody(operation delta.Operation, sequences bool) deltaOperation {
	body := deltaOperation{
		Kind: operation.Kind, ID: operation.InstanceID,
		Boundaries: operation.Boundaries,
	}
	// Values replace the markup only when they are smaller. A fragment of two
	// elements costs more as an address plus its values than as the markup
	// itself, because the address is per-operation overhead and there is almost
	// no static text to save; a list row is exactly that shape, and its parent —
	// a hundred hole frames — is the opposite one. Choosing per fragment is what
	// keeps the split from ever being a loss.
	if sequences && operation.Sequence != "" && valuesAreSmaller(operation) {
		body.Seq, body.Values = operation.Sequence, operation.Values
		return body
	}
	body.HTML = operation.HTML
	return body
}

func valuesAreSmaller(operation delta.Operation) bool {
	size := len(operation.Sequence)
	for _, value := range operation.Values {
		size += len(value)
	}
	return size < len(operation.HTML)
}

func (o Options) renderOptions(caller []htmlbind.Option) []htmlbind.Option {
	owned := []htmlbind.Option{
		htmlbind.WithBoundaryPrefix(o.dataAttributePrefix()),
		// Seeding every validator with the build identity is what keeps two
		// builds from producing comparable digests. Negotiate already answers a
		// build mismatch with a complete document before any validator is read,
		// so this matters where the build header was dropped in transit.
		htmlbind.WithValidatorTag(o.buildID()),
	}
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
	// Version is whatever the client wrote after "v=" in the render header, or
	// zero when it wrote none. This package neither defines it nor compares it:
	// the browser client belongs to the caller, so the caller owns its wire
	// version and what a mismatch means. It is carried so a caller that does
	// version its wire can read it, and it is echoed back on the response.
	//
	// The compatibility axis this package still operates is the build identity,
	// whose value Options.BuildID already makes the caller's.
	Version int
	// Known holds the validators the client already has. It is empty on a
	// client's first update, which simply yields a larger delta.
	Known delta.Manifest
}

// Negotiate resolves how a request must be answered.
//
// Anything unrecognized resolves to ModeDocument rather than to an error: a
// stale client, a truncated header, and a proxy that dropped a header must all
// still produce a working page. That is a total function on the mode name
// rather than a version comparison, so it holds with no version at all.
func (o Options) Negotiate(r *http.Request) Negotiated {
	name, version, ok := parseRender(r.Header.Get(o.renderHeader()))
	if !ok {
		return Negotiated{Mode: ModeDocument}
	}
	var mode Mode
	switch name {
	case modeNavigation:
		mode = ModeNavigation
	case modeLive:
		mode = ModeLive
	case modeRedraw:
		mode = ModeRedraw
	case modeSequence:
		mode = ModeSequence
	default:
		return Negotiated{Mode: ModeDocument}
	}
	// A render request must stay side-effect free, which is also why it needs
	// no CSRF token: a GET that changes nothing cannot be forged into an
	// action. A non-GET arriving in this mode is a client error, not a delta.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return Negotiated{Mode: ModeDocument}
	}
	// A sequence is asked for by an address that digests its own content, so a
	// build mismatch cannot make it wrong: either this process has that exact
	// tree or it does not. Gating it on the build would forfeit the one thing
	// this response uniquely has, which is that a shared cache may hold it
	// across builds and across users.
	if mode == ModeSequence {
		return Negotiated{Mode: mode, Version: version}
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
//
// The version is the one the request carried, not one this package chose. A
// caller versioning its own wire sees its own number come back; a caller that
// versions nothing gets a bare mode name, because inventing a number here would
// be this package versioning a contract it no longer owns.
func renderToken(mode Mode, version int) string {
	return modeName(mode) + versionSuffix(version)
}

func modeName(mode Mode) string {
	switch mode {
	case ModeLive:
		return modeLive
	case ModeRedraw:
		return modeRedraw
	default:
		return modeNavigation
	}
}

// versionSuffix writes back what the request claimed, and nothing when it
// claimed nothing.
func versionSuffix(version int) string {
	if version == 0 {
		return ""
	}
	return ";v=" + strconv.Itoa(version)
}

// parseRender reads a render header of the form "navigation;v=1" or a bare
// "navigation".
//
// The version part is optional because it is the caller's field: a client that
// does not version its wire writes the mode alone, and a malformed version is
// read as none rather than as a reason to refuse. Refusing would cost the page
// its update for a field this package does not interpret.
func parseRender(value string) (mode string, version int, ok bool) {
	name, rest, found := strings.Cut(value, ";")
	name = strings.TrimSpace(name)
	if name == "" {
		return "", 0, false
	}
	if !found {
		return name, 0, true
	}
	digits, found := strings.CutPrefix(strings.TrimSpace(rest), "v=")
	if !found {
		return name, 0, true
	}
	version, err := strconv.Atoi(strings.TrimSpace(digits))
	if err != nil {
		return name, 0, true
	}
	return name, version, true
}

// DecodeManifest reads the compact validator list a client sends back. The
// encoding is "id:frame" or "id:frame:children" separated by commas, which stays
// inside one header and needs no escaping because every part is an opaque token.
//
// The third part is what lets a list say its rows moved without its parent being
// replaced. It is absent for a boundary containing no nested boundary, which is
// most of them, and a pair with only two parts still reads.
func DecodeManifest(encoded string) delta.Manifest {
	var manifest delta.Manifest
	for _, entry := range strings.Split(encoded, ",") {
		id, rest, found := strings.Cut(entry, ":")
		if !found || id == "" {
			continue
		}
		frame, rest, _ := strings.Cut(rest, ":")
		if frame == "" {
			continue
		}
		childrenValidator, parent, _ := strings.Cut(rest, ":")
		manifest.Instances = append(manifest.Instances, delta.Instance{
			ID: id, ParentID: parent, FrameValidator: frame, ChildrenValidator: childrenValidator,
		})
	}
	return manifest
}

// EncodeManifest renders the validator list a client sends back. It exists so a
// test, and any non-browser client, can produce exactly what the runtime does.
func EncodeManifest(manifest delta.Manifest) string {
	var out strings.Builder
	for _, instance := range manifest.Instances {
		if out.Len() > 0 {
			out.WriteByte(',')
		}
		out.WriteString(instance.ID)
		out.WriteByte(':')
		out.WriteString(instance.FrameValidator)
		if instance.ChildrenValidator != "" || instance.ParentID != "" {
			out.WriteByte(':')
			out.WriteString(instance.ChildrenValidator)
		}
		if instance.ParentID != "" {
			out.WriteByte(':')
			out.WriteString(instance.ParentID)
		}
	}
	return out.String()
}

// deltaResponse is the JSON body of a navigation or action response.
//
// It carries no version field. Nothing compared the one it used to carry, and a
// field nothing compares is not a version but a constant every response asks the
// wire for and every client ignores. A caller versioning its own wire adds its
// own field beside this shape.
type deltaResponse struct {
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

func encodeJSON(w http.ResponseWriter, body deltaResponse) error {
	return json.NewEncoder(w).Encode(body)
}

type deltaOperation struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	HTML string `json:"html"`
	// Boundaries names the nested boundaries appearing as holes in HTML.
	//
	// A hole whose id also carries an operation in this response is filled from
	// it; one that does not is a region the client already holds, and it moves
	// that live node in rather than recreating it — which is what keeps the
	// focus, the form values, and the media state inside it. Without the list a
	// missing fragment would be indistinguishable from a truncated response.
	Boundaries []string `json:"boundaries,omitempty"`
	// Seq addresses this fragment's static half and Values are the varying half
	// a client walks it with. They replace HTML for a client that said it can
	// walk sequences, because the statics then travel once per client instead of
	// once per render.
	Seq    string   `json:"seq,omitempty"`
	Values []string `json:"values,omitempty"`
}

type deltaInstance struct {
	ID    string `json:"id"`
	Frame string `json:"frame"`
	// Children digests the nested boundary ids, so a later request can say a
	// list reordered without its parent being replaced to express it. Absent for
	// a boundary containing no nested boundary, which is most of them.
	Children string `json:"children,omitempty"`
	// Parent names the enclosing boundary, so a region that disappears can be
	// attributed to the boundary that will report the survivors. Absent for an
	// outermost boundary.
	Parent string `json:"parent,omitempty"`
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
		return renderDelta(w, r, o, negotiated, wrappers, leaf)
	}
	// This entry buffers, so it cannot hold a delivery stream open. A live
	// request reaching it is answered with the document, which is the same
	// fallback every unrecognized condition takes and leaves the client with a
	// working page rather than an error.
	//
	// The document render collects so every boundary carries its instance
	// attribute; without them a later delta could not find its targets.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := delta.CollectChain(w, o.Key, wrappers, leaf, o.renderOptions(nil)...)
	return err
}

// liveHeader names the response header saying whether this composition owns a
// live boundary.
func (o Options) liveHeader() string { return o.prefix() + "-Live" }

// kindHeader and instanceHeader name the component a redraw addresses.
//
// They are headers rather than path segments so a redraw can be answered at any
// URL, which is what lets a caller serve one from the page the component sits
// on. There the redraw inherits the page's own authorization; on a reserved path
// it needs a second path pattern kept in step with the first, and nothing forces
// two such rules to agree.
//
// They are headers rather than query parameters because the generated decoder
// treats an unknown parameter name as an error, so a query-carried kind and
// instance would reserve two names an author could then not declare.
func (o Options) kindHeader() string { return o.prefix() + "-Kind" }

func (o Options) instanceHeader() string { return o.prefix() + "-Instance" }

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

func renderDelta(w http.ResponseWriter, r *http.Request, o Options, negotiated Negotiated, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) error {
	sequences := o.wantsSequences(r)
	diff, err := delta.RenderDelta(o.Key, negotiated.Known, wrappers, leaf, o.renderOptions(nil)...)
	if err != nil {
		// Nothing has been written yet, so the caller can still choose a status
		// and serve an ordinary error page.
		return err
	}
	body := deltaResponse{}
	for _, operation := range diff.Operations {
		body.Operations = append(body.Operations, o.operationBody(operation, sequences))
	}
	for _, instance := range diff.Manifest.Instances {
		body.Manifest = append(body.Manifest, deltaInstance{
			ID: instance.ID, Frame: instance.FrameValidator,
			Children: instance.ChildrenValidator, Parent: instance.ParentID,
		})
	}
	body.Head = diff.Head
	// A navigation can arrive at a route whose composition owns a live boundary,
	// and the client reused its document shell, so this body is the only place
	// that can tell it so.
	body.Live = htmlbind.HasLiveBlock(wrappers, leaf)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// A delta carries per-document validators, so it is never shareable.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(o.renderHeader(), renderToken(ModeNavigation, negotiated.Version))
	return encodeJSON(w, body)
}
