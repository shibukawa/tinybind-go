// Package updatecore is the transport-free half of the partial-update surface.
//
// The entries here read a request and write nothing through it, so what they
// need from a transport is a Reader and not a request type. Both HTTP runtimes
// shell this package: htmlupdate over *http.Request, and the fasthttp runtime
// over *fasthttp.RequestCtx. One implementation answers both, which is what
// keeps a wire contract from being agreed twice.
//
// Nothing here is imported by an application. Each shell redeclares Options and
// Response so its entries can be methods, and converts; the declarations are
// identical by construction, because a drifted field stops the conversion from
// compiling.
package updatecore

import (
	"context"
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
	// StreamContentType overrides the media type of a streamed delta. Empty
	// uses DefaultStreamContentType.
	//
	// This one names the wire format rather than a limit, so overriding it is a
	// framing choice a client has to agree with, not a tuning knob.
	StreamContentType string
	// OnFailure receives every request an endpoint of this package could not
	// answer. It observes rather than answers.
	//
	// Every entry returns the response it computed, with its Failure field set,
	// so a caller with its own error pages substitutes them by sending something
	// else instead of what it was handed. This hook is for the log line and the
	// span, which a caller wants on every refusal whether or not it changes the
	// answer — and which are otherwise lost, since a status alone cannot say
	// whether a page was stale or a render failed.
	//
	// It takes the request's context rather than the request. A log line and a
	// span both want the trace, the deadline, and whatever the caller's own
	// middleware put there, and none of them want the transport; taking the
	// narrower value is also what lets this field mean the same thing on a
	// backend whose request type is not *http.Request.
	OnFailure func(ctx context.Context, failure Failure)
}

// DefaultMaxManifestBytes bounds the validators a request may carry. Beyond it
// the hints are dropped, which costs bytes in the response instead of risking a
// proxy rejecting the request.
const DefaultMaxManifestBytes = 8 << 10

func (o Options) RenderHeader() string { return o.HeaderNamespace() + "-Render" }

func (o Options) ManifestHeader() string { return o.HeaderNamespace() + "-Manifest" }

func (o Options) BuildHeader() string { return o.HeaderNamespace() + "-Build" }

func (o Options) HeaderNamespace() string {
	if o.HeaderPrefix == "" {
		return DefaultHeaderPrefix
	}
	return o.HeaderPrefix
}

// PathNamespace returns the endpoint namespace without a trailing slash.
func (o Options) PathNamespace() string {
	if o.PathPrefix == "" {
		return DefaultPathPrefix
	}
	return "/" + strings.Trim(o.PathPrefix, "/")
}

func (o Options) ManifestLimit() int {
	if o.MaxManifestBytes == 0 {
		return DefaultMaxManifestBytes
	}
	return o.MaxManifestBytes
}

// RenderOptions carries the naming these options configure into every htmlbind
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
func (o Options) SequenceHeader() string { return o.HeaderNamespace() + "-Sequences" }

// WantsSequences reports whether this request said it can walk sequences.
func (o Options) WantsSequences(r Reader) bool {
	return r != nil && r.Header(o.SequenceHeader()) != ""
}

// OperationBody writes one operation, in whichever half the client can use.
func (o Options) OperationBody(operation delta.Operation, sequences bool) DeltaOperation {
	body := DeltaOperation{
		Kind: operation.Kind, ID: operation.InstanceID,
		Boundaries: operation.Boundaries,
	}
	if SendsValues(sequences, operation) {
		body.Seq, body.Values = operation.Sequence, operation.Values
		return body
	}
	body.HTML = operation.HTML
	return body
}

// SendsValues decides which half of a fragment travels, for every path that
// sends one.
//
// Values replace the markup only when they are smaller. A fragment of two
// elements costs more as an address plus its values than as the markup itself,
// because the address is per-operation overhead and there is almost no static
// text to save; a list row is exactly that shape, and its parent — a hundred
// hole frames — is the opposite one. Choosing per fragment is what keeps the
// split from ever being a loss.
//
// It is one function because the buffered path applied the size test and the
// streamed path did not, so the claim held on one path and not on its sibling —
// and the streamed path is the one every navigation goes through. That is the
// third defect of this shape: a rule applied on one path and not the other. A
// predicate with one home is what stops there being a fourth.
func SendsValues(sequences bool, operation delta.Operation) bool {
	if !sequences || operation.Sequence == "" {
		return false
	}
	size := len(operation.Sequence)
	for _, value := range operation.Values {
		size += len(value)
	}
	return size < len(operation.HTML)
}

func (o Options) RenderOptions(caller []htmlbind.Option) []htmlbind.Option {
	owned := []htmlbind.Option{
		htmlbind.WithBoundaryPrefix(o.AttributePrefix()),
		// Seeding every validator with the build identity is what keeps two
		// builds from producing comparable digests. Negotiate already answers a
		// build mismatch with a complete document before any validator is read,
		// so this matters where the build header was dropped in transit.
		htmlbind.WithValidatorTag(o.Build()),
	}
	return append(owned, caller...)
}

func (o Options) AttributePrefix() string {
	if o.DataAttributePrefix == "" {
		return DefaultDataAttributePrefix
	}
	return o.DataAttributePrefix
}

func (o Options) Global() string {
	if o.GlobalName == "" {
		return DefaultGlobalName
	}
	return o.GlobalName
}

func (o Options) QueryLimit() int {
	if o.MaxQueryBytes == 0 {
		return DefaultMaxQueryBytes
	}
	return o.MaxQueryBytes
}

// DefaultStreamContentType marks a delta delivered as a record stream. One JSON
// record per line, which is the framing the module already uses for streamed
// values. Options.StreamContentType overrides it.
const DefaultStreamContentType = "application/x-ndjson; charset=utf-8"

func (o Options) StreamMediaType() string {
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
func (o Options) Negotiate(r Reader) Negotiated {
	name, version, ok := parseRender(r.Header(o.RenderHeader()))
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
	if r.Method() != "GET" && r.Method() != "HEAD" {
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
	if r.Header(o.BuildHeader()) != o.Build() {
		return Negotiated{Mode: ModeDocument}
	}
	encoded := r.Header(o.ManifestHeader())
	if len(encoded) > o.ManifestLimit() {
		encoded = ""
	}
	return Negotiated{Mode: mode, Version: version, Known: DecodeManifest(encoded)}
}

// RenderToken is the value a response echoes for one mode.
//
// The version is the one the request carried, not one this package chose. A
// caller versioning its own wire sees its own number come back; a caller that
// versions nothing gets a bare mode name, because inventing a number here would
// be this package versioning a contract it no longer owns.
func RenderToken(mode Mode, version int) string {
	return modeName(mode) + versionSuffix(version)
}

// modeName is exhaustive on purpose, and a mode it does not know panics rather
// than resolving to something.
//
// The default arm this replaces returned navigation, so ModeSequence — added
// after the arm was written — echoed navigation on every sequence response. A
// client enforcing the echo, which is where a proxy-substituted body is
// detected, discarded every tree it fetched; an operation that had arrived as
// values then had no markup to fall back to, and the navigation degraded to a
// complete document. The only trace was that pages got bigger.
//
// A default arm turns a missing case into a wrong claim. The panic is
// unreachable — Negotiate resolves anything unrecognized to ModeDocument — so it
// is here to make the next mode a failure at its first test rather than a
// response quietly claiming to be something else.
func modeName(mode Mode) string {
	switch mode {
	case ModeDocument, ModeNavigation:
		return modeNavigation
	case ModeLive:
		return modeLive
	case ModeRedraw:
		return modeRedraw
	case ModeSequence:
		return modeSequence
	}
	panic("htmlupdate: no name for render mode " + strconv.Itoa(int(mode)))
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

// DeltaResponse is the JSON body of a navigation or action response.
//
// It carries no version field. Nothing compared the one it used to carry, and a
// field nothing compares is not a version but a constant every response asks the
// wire for and every client ignores. A caller versioning its own wire adds its
// own field beside this shape.
type DeltaResponse struct {
	Operations []DeltaOperation `json:"ops"`
	Manifest   []DeltaInstance  `json:"manifest,omitempty"`
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

type DeltaOperation struct {
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

type DeltaInstance struct {
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

// liveHeader names the response header saying whether this composition owns a
// live boundary.
func (o Options) LiveHeader() string { return o.HeaderNamespace() + "-Live" }

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
func (o Options) KindHeader() string { return o.HeaderNamespace() + "-Kind" }

func (o Options) InstanceHeader() string { return o.HeaderNamespace() + "-Instance" }
