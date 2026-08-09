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
//
// What each entry reads from a request lives in internal/updatecore, so the
// second transport runtime answers the same wire contract rather than agreeing
// with this one. This package is the net/http half: it wraps a *http.Request in
// the reader that half takes, and owns everything that writes — the document
// and delta renders, the record streams, and the runtime asset handler.
package htmlupdate

import (
	"context"
	"net/http"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
	"github.com/shibukawa/tinybind-go/internal/updatecore"
)

// The wire contract and everything derived from it are declared once, in
// internal/updatecore, and aliased here. A caller sees one of each type
// whichever runtime it builds against, which is what keeps a Failure raised on
// one side inspectable on the other. Options and Response are the exceptions:
// each runtime redeclares them so its entries can be methods, and converts.

// Mode is the rendering a request asked for.
type Mode = updatecore.Mode

const (
	// ModeDocument is the complete HTML document. It is what a request without
	// a usable render header gets, including one from an incompatible client.
	ModeDocument = updatecore.ModeDocument
	// ModeNavigation returns only the changed boundaries of the same route.
	ModeNavigation = updatecore.ModeNavigation
	// ModeLive returns the deliveries of the same route's live boundaries, on a
	// response held open for as long as the subscriptions live.
	ModeLive = updatecore.ModeLive
	// ModeRedraw returns one registered component's subtree, addressed by the
	// kind and instance headers rather than by the URL path.
	ModeRedraw = updatecore.ModeRedraw
	// ModeSequence returns the static half of one fragment, addressed by a
	// digest of its own content.
	ModeSequence = updatecore.ModeSequence
)

// Negotiated is what a request asked for, after validation.
type Negotiated = updatecore.Negotiated

// Update is one region an action rewrote.
type Update = updatecore.Update

// Replace rewrites the region addressed by targetID with fragment.
func Replace(targetID string, fragment htmlbind.Fragment) Update {
	return updatecore.Replace(targetID, fragment)
}

// FailureKind names why an update endpoint could not answer.
type FailureKind = updatecore.FailureKind

const (
	// FailureMalformedRequest is a redraw that named no component.
	FailureMalformedRequest = updatecore.FailureMalformedRequest
	// FailureUnknownComponent is a kind this deployment does not publish.
	FailureUnknownComponent = updatecore.FailureUnknownComponent
	// FailureArgumentsTooLarge is a query past the configured bound.
	FailureArgumentsTooLarge = updatecore.FailureArgumentsTooLarge
	// FailureInvalidArguments is a query the generated decoder refused.
	FailureInvalidArguments = updatecore.FailureInvalidArguments
	// FailureRenderFailed is a component that could not render.
	FailureRenderFailed = updatecore.FailureRenderFailed
)

// Failure is one request an update endpoint could not answer. It satisfies
// error and unwraps to the cause, so it goes straight to a logger or a span.
type Failure = updatecore.Failure

// Reloadable is one component published as a redraw endpoint.
type Reloadable = updatecore.Reloadable

// Registry holds the components a deployment publishes for redraw.
type Registry = updatecore.Registry

// Asset is one static file this package requires a page to load.
type Asset = updatecore.Asset

// RuntimeConfig is what the browser runtime reads to learn its own names.
type RuntimeConfig = updatecore.RuntimeConfig

// QueryError is a redraw argument the generated decoder refused.
type QueryError = updatecore.QueryError

// The naming defaults. Each is shared with the generator, which writes what
// these read back.
const (
	// DefaultHeaderPrefix names the request and response headers.
	DefaultHeaderPrefix = updatecore.DefaultHeaderPrefix
	// DefaultPathPrefix is the URL namespace of the endpoints this package owns.
	DefaultPathPrefix = updatecore.DefaultPathPrefix
	// DefaultDataAttributePrefix names the attributes the protocol puts in a
	// document.
	DefaultDataAttributePrefix = updatecore.DefaultDataAttributePrefix
	// DefaultGlobalName is what the browser runtime is installed under.
	DefaultGlobalName = updatecore.DefaultGlobalName
	// DefaultRuntimeFileName names the served runtime file.
	DefaultRuntimeFileName = updatecore.DefaultRuntimeFileName
	// DefaultCSRFFieldName is the hidden field generated forms carry.
	DefaultCSRFFieldName = updatecore.DefaultCSRFFieldName
	// DefaultCSRFHeaderName is where the runtime puts the token.
	DefaultCSRFHeaderName = updatecore.DefaultCSRFHeaderName
	// DefaultMaxManifestBytes bounds the validators a request may carry.
	DefaultMaxManifestBytes = updatecore.DefaultMaxManifestBytes
	// DefaultMaxQueryBytes bounds the arguments a redraw may carry.
	DefaultMaxQueryBytes = updatecore.DefaultMaxQueryBytes
	// DefaultStreamContentType marks a delta delivered as a record stream.
	DefaultStreamContentType = updatecore.DefaultStreamContentType
)

// BuildID identifies the running binary, so anything that could change
// rendering invalidates client state.
var BuildID = updatecore.BuildID

// ErrCSRFMissing reports an unsafe request carrying no token at all.
var ErrCSRFMissing = updatecore.ErrCSRFMissing

// ErrCSRFMismatch reports a token that is not the session's.
var ErrCSRFMismatch = updatecore.ErrCSRFMismatch

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

// core is these options as the transport-free half reads them.
//
// It is a conversion rather than a copy, so the two declarations cannot drift:
// a field added on one side and not the other stops this line from compiling,
// which is the only guard a duplicated struct needs.
func (o Options) core() updatecore.Options { return updatecore.Options(o) }

// Negotiate resolves how a request must be answered.
//
// Anything unrecognized resolves to ModeDocument rather than to an error: a
// stale client, a truncated header, and a proxy that dropped a header must all
// still produce a working page.
func (o Options) Negotiate(r *http.Request) Negotiated { return o.core().Negotiate(reader(r)) }

// Validate reports a configuration this package cannot serve.
func (o Options) Validate() error { return o.core().Validate() }

// DecodeManifest reads the compact validator list a client sends back.
func DecodeManifest(encoded string) delta.Manifest { return updatecore.DecodeManifest(encoded) }

// EncodeManifest renders the validator list a client sends back. It exists so a
// test, and any non-browser client, can produce exactly what the runtime does.
func EncodeManifest(manifest delta.Manifest) string { return updatecore.EncodeManifest(manifest) }
