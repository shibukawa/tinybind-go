package htmlupdate

import (
	"context"
	"net/http"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/internal/updatecore"
)

// The streaming half. Every entry here decides what it can while it still owns
// the request, opens the response, and writes records; nothing is handed back
// to the caller half-open.
//
// That shape is not a net/http preference. fasthttp writes a streamed body from
// a callback that runs after the handler returned, so a stream a handler holds
// across statements has no transcription there. One authored form that works on
// both is worth more than an entry point that reads slightly better on one, and
// the callback also closes two failures the held form allowed: a producer that
// forgets to close writes a truncated stream, and a write error it discards is
// invisible.

// DeltaStream is an open record stream a producer writes boundary completions
// to as they settle.
//
// It is the same type on both transports, deliberately. A wrapper renaming so
// much as one method would make the two handler bodies differ by more than
// their signature line, and the source transform rewrites signatures and
// argument lists — not method names.
type DeltaStream = updatecore.DeltaStream

// ManifestEntry is what a client stores for one instance beside its markup: the
// validator of the region's own bytes, the digest of its nested boundary ids,
// and the boundary enclosing it.
type ManifestEntry = updatecore.ManifestEntry

// StreamPlan is what a stream entry decided before it committed. A caller sees
// one only when it drives the delivery itself.
type StreamPlan = updatecore.StreamPlan

// WriteStream opens a record stream, runs fn against it, and closes it.
//
// The stream is closed whether or not fn returns an error, so a producer cannot
// leave a client holding a truncated response. fn's error is reported in band
// through the terminator, because the response committed when the head record
// went out and the status can no longer change.
//
// head is the merged head of the composition, written as the first record so a
// stylesheet lands before the markup that needs it. The response headers are the
// caller's: take them from [Options.StreamHeaders] before calling.
func (o Options) WriteStream(w http.ResponseWriter, r *http.Request, head []string, fn func(*DeltaStream) error) {
	o.writeStream(w, updatecore.StreamPlan{
		Mode: ModeNavigation, Version: o.Negotiate(r).Version, Head: head,
	}, fn)
}

// WriteLiveStream is WriteStream for a delivery stream: the same records on the
// same framing, in the live mode rather than the navigation one.
//
// The difference a caller sees is the ending. A navigation stream closes final,
// having described the route; a live stream closes done when every source
// finished, or retry when the server closed a healthy response at a lifetime
// bound. A client keys its retry policy on that rather than on the fact that
// the stream ended.
func (o Options) WriteLiveStream(w http.ResponseWriter, r *http.Request, head []string, fn func(*DeltaStream) error) {
	o.writeStream(w, updatecore.StreamPlan{
		Mode: ModeLive, Live: true, Version: o.Negotiate(r).Version, Head: head,
	}, fn)
}

func (o Options) writeStream(w http.ResponseWriter, plan updatecore.StreamPlan, fn func(*DeltaStream) error) {
	stream := o.core().OpenStream(w, plan)
	err := fn(stream)
	if err != nil {
		stream.Fail(err.Error())
	}
	if cerr := stream.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		updatecore.ReportStreamError(err)
	}
}

// Render answers one request with either a complete document or a delta.
//
// It always sets Vary, because a cache that served a delta body to a document
// request would hand a browser a page of JSON. The caller keeps every other
// response concern, as elsewhere in this module.
//
// It buffers, so it holds nothing open and reports every failure as an ordinary
// error. A live request reaching it is answered with the document, which is the
// same fallback every unrecognized condition takes and leaves the client with a
// working page rather than an error.
func (o Options) Render(w http.ResponseWriter, r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	return o.core().Render(w, reader(r), wrappers, leaf, options)
}

// RenderStream answers a navigation with a record stream instead of one
// buffered body, so each region applies as soon as it is written.
//
// Everything that could change the status is decided before the first record,
// because writing it commits the response. After that a failure can only be
// reported in band.
func (o Options) RenderStream(w http.ResponseWriter, r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	plan, err := o.core().PlanBufferedStream(reader(r), wrappers, leaf, options)
	if err != nil {
		return err
	}
	if !plan.Streams() {
		return o.core().RenderDocument(w, plan, wrappers, leaf)
	}
	return o.core().RunBufferedStream(o.core().OpenStream(w, plan), plan)
}

// RenderStreamAsync answers a navigation with a record stream that also carries
// await boundaries as they settle.
//
// Each region reaches the browser with its fallback in place and is replaced
// when its dependency finishes, so a slow one delays only itself. A chain with
// no await boundary produces exactly what RenderStream does.
//
// This entry serves the document and navigation modes. A live request reaching
// it is answered as a navigation and terminated final, so a client that opened a
// live connection to a route this caller does not serve live learns so at once
// instead of holding a connection that will never deliver.
func (o Options) RenderStreamAsync(ctx context.Context, w http.ResponseWriter, r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	return o.renderStream(ctx, w, r, false, wrappers, leaf, options)
}

// RenderLiveStream answers a live request by holding the response open for as
// long as the composition's subscriptions live.
//
// It is the only entry that keeps subscriptions open. Everything else that
// reaches a live request answers it as a navigation and terminates, so a client
// learns at once that this route delivers nothing.
func (o Options) RenderLiveStream(ctx context.Context, w http.ResponseWriter, r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	return o.renderStream(ctx, w, r, true, wrappers, leaf, options)
}

func (o Options) renderStream(ctx context.Context, w http.ResponseWriter, r *http.Request, serveLive bool, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options []htmlbind.Option) error {
	plan, err := o.core().PlanStream(reader(r), serveLive, wrappers, leaf, options)
	if err != nil {
		return err
	}
	if !plan.Streams() {
		return o.core().RenderDocument(w, plan, wrappers, leaf)
	}
	return o.core().RunStream(ctx, o.core().OpenStream(w, plan), plan, wrappers, leaf)
}

// SetStreamErrorHandler installs the destination for stream failures raised
// after the response committed.
//
// A stream that has written its head record cannot answer with a status, so an
// error from a producer has nowhere to go but a log. It is installed once for
// the process and covers both transports.
func SetStreamErrorHandler(fn func(error)) { updatecore.SetStreamErrorHandler(fn) }
