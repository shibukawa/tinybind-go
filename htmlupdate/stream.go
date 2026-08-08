package htmlupdate

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

// DefaultStreamContentType marks a delta delivered as a record stream. One JSON
// record per line, which is the framing the module already uses for streamed
// values. Options.StreamContentType overrides it.
const DefaultStreamContentType = "application/x-ndjson; charset=utf-8"

// record is one line of a streamed delta.
//
// Each operation carries its own manifest entry, because a trailing manifest
// cannot be written before the operations it describes. That is also why the
// stream ends with an explicit terminator: a client that stops receiving has no
// other way to tell a finished render from a truncated one.
// It carries no version field, for the reason deltaResponse carries none: the
// browser client belongs to the caller, so the caller owns its wire version.
type record struct {
	Record string `json:"r"`
	// head fields
	Head []string `json:"head,omitempty"`
	// Build identifies the binary that opened this stream. A client reconnecting
	// into a redeployed server would otherwise apply deliveries addressed at a
	// document it is no longer showing; reading it from the first record means a
	// client that consumes records without inspecting response headers can still
	// tell, and reload instead.
	Build string `json:"build,omitempty"`
	// operation fields
	Kind  string `json:"kind,omitempty"`
	ID    string `json:"id,omitempty"`
	HTML  string `json:"html,omitempty"`
	Frame string `json:"frame,omitempty"`
	// terminator and directive fields
	Navigate string `json:"navigate,omitempty"`
	Error    string `json:"error,omitempty"`
	// Reason names which of the endings this is, because a bare close cannot
	// say. See the end* constants.
	Reason string `json:"reason,omitempty"`
	// RetryMillis is the server's own hint for how long to wait before
	// reconnecting. A client's backoff can only react to a failure; the server
	// is the only party that knows it is shedding load or rolling a deploy, and
	// can spread the return before anything fails. Zero leaves the delay to the
	// client.
	RetryMillis int `json:"retryMs,omitempty"`
}

const (
	recordHead  = "head"
	recordOp    = "op"
	recordAwait = "await"
	recordEnd   = "end"
)

// The terminator reasons. A stream ends from its source, from a server bound,
// or from the client aborting; only the first two need a record, and the two of
// them mean opposite things to a client's retry policy.
//
// The first three end a document-side stream and answer "is more coming, and
// from where"; the last two end a live stream and answer "should I come back".
const (
	// endFinal is a navigation that produced everything it will ever produce.
	// The client must issue no live request.
	endFinal = "final"
	// endLivePending is a navigation whose composition owns live boundaries, so
	// a live request is expected. It is the handoff marker on the streamed path.
	endLivePending = "live_pending"
	// endFailed is a sequence that ended on an unrecovered failure. Nothing more
	// is coming and a committed fallback will not be replaced by this response.
	endFailed = "failed"
	// endDone is a live stream whose every source finished. The client stops and
	// does not reconnect.
	endDone = "done"
	// endRetry is a live stream the server closed healthy: a lifetime bound, a
	// shutdown, a rebalance. The client is expected to reconnect promptly, and
	// must not spend a backoff attempt on it, because a rollover is not a
	// failure and treating it as one stalls a working screen every time the
	// server rotates a connection.
	endRetry = "retry"
)

// DeltaStream is an open record stream a producer writes boundary completions
// to as they settle.
//
// It exists so the transport and the producer stay separate. A synchronous
// delta drives it today; an asynchronous render sequence drives it by calling
// Replace once per completion, which makes wiring one in a call rather than a
// redesign.
type DeltaStream struct {
	writer *recordWriter
	closed bool
	// ending is the reason Close writes. It is set at open time from the mode,
	// so a producer that only calls Close still terminates correctly, and moved
	// by ExpectLive or Retry when the producer knows better.
	ending string
}

// OpenStream commits the response and writes the head record.
//
// Everything that could change the status has to be decided before this call,
// because after it the status is fixed and a failure can only be reported in
// band through Fail.
func (o Options) OpenStream(w http.ResponseWriter, head []string) *DeltaStream {
	return o.openStream(w, ModeNavigation, 0, head)
}

// OpenLiveStream commits a delivery stream: the same records on the same
// framing, in the live mode rather than the navigation one.
//
// The difference a caller sees is the ending. A navigation stream closes final,
// having described the route; a live stream closes done when every source
// finished, or retry when the server closed a healthy response at a lifetime
// bound. A client keys its retry policy on that rather than on the fact that
// the stream ended.
func (o Options) OpenLiveStream(w http.ResponseWriter, head []string) *DeltaStream {
	return o.openStream(w, ModeLive, 0, head)
}

// openStream commits the response. version is the one the request claimed, so
// the echoed token carries the caller's own number back rather than one this
// package invented; zero writes a bare mode name.
func (o Options) openStream(w http.ResponseWriter, mode Mode, version int, head []string) *DeltaStream {
	w.Header().Set("Content-Type", o.streamContentType())
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(o.renderHeader(), renderToken(mode, version))
	ending := endFinal
	if mode == ModeLive {
		ending = endDone
	}
	stream := &DeltaStream{writer: newRecordWriter(w), ending: ending}
	stream.writer.write(record{Record: recordHead, Head: head, Build: o.buildID()})
	return stream
}

// ExpectLive marks this stream's terminator as handing off to a live request,
// which a navigation does when the route it just described owns a live
// boundary.
//
// Without it a client either opens a speculative live request on every
// navigation — one full page execution per screen that will never deliver
// anything — or the caller hardcodes which routes are live.
func (s *DeltaStream) ExpectLive() {
	if s.ending == endFinal {
		s.ending = endLivePending
	}
}

// Replace writes one settled boundary and the validator it produced.
func (s *DeltaStream) Replace(instanceID, html, frame string) {
	s.writer.write(record{Record: recordOp, Kind: delta.OpReplace, ID: instanceID, HTML: html, Frame: frame})
}

// Unchanged restates a boundary's validator without markup, so the client can
// rebuild its whole manifest from what it received.
func (s *DeltaStream) Unchanged(instanceID, frame string) {
	s.writer.write(record{Record: recordOp, ID: instanceID, Frame: frame})
}

// Settled writes an await boundary that finished after the initial pass.
//
// It addresses a placeholder inside a region the client already installed,
// which is a different namespace from an instance id, so it is its own record
// kind rather than an operation with a surprising target.
func (s *DeltaStream) Settled(boundaryID string, html []byte) {
	s.writer.write(record{Record: recordAwait, ID: boundaryID, HTML: string(html)})
}

// Sent reports whether an instance already appeared, so a producer emitting
// completions out of order does not restate one it already wrote.
func (s *DeltaStream) Sent(instanceID string) bool {
	_, ok := s.writer.seen[instanceID]
	return ok
}

// Fail reports a failure that happened after the response committed. The status
// is already sent, so this is the only way to say so.
func (s *DeltaStream) Fail(message string) {
	s.writer.write(record{Record: recordEnd, Reason: endFailed, Error: message})
	s.closed = true
}

// Retry closes a healthy stream the server chose to end: a lifetime bound, a
// shutdown, a rebalance. The client reconnects promptly instead of backing off,
// because nothing failed.
//
// after is the server's own hint for how long to wait. Zero leaves the delay to
// the client, which is the right answer for an ordinary rollover; a server
// shedding load or rolling a deploy is the only party that knows to spread the
// return, so it is the only one that can fill this in.
//
// Calling it on a navigation stream is a mistake this package does not guard
// against, because a navigation has nothing to reconnect to; it is here for the
// live path, where a bare close cannot mean stop.
func (s *DeltaStream) Retry(after time.Duration) error {
	if !s.closed {
		s.writer.write(record{Record: recordEnd, Reason: endRetry, RetryMillis: int(after.Milliseconds())})
		s.closed = true
	}
	return s.writer.err
}

// Close writes the terminator. Without it the client treats the stream as
// truncated and discards its manifest, so a producer must always reach here.
//
// The terminator names which ending this is: a navigation says whether a live
// request should follow, and a live stream says whether the client should come
// back. A close with no reason would make a healthy lifetime rollover
// indistinguishable from a fault, so a client would back off on both and stall a
// working screen every time the server rotates a connection.
func (s *DeltaStream) Close() error {
	if !s.closed {
		s.writer.write(record{Record: recordEnd, Reason: s.ending})
		s.closed = true
	}
	return s.writer.err
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

// RenderLiveStream is RenderStreamAsync for a chain holding live sources: in the
// live mode it keeps every subscription open and writes each delivery as it
// arrives.
//
// The mode is what decides. A navigation asks what this route looks like now, so
// a live boundary settles in place and the response ends; a live request asks for
// the deliveries, so the response stays open. Serving both from one entry is what
// keeps the reconnect path and the render path the same code — the reason a
// reconnect needs no cursor, no event log, and no replay.
//
// Reconnecting after a dropped stream is the same request again. Nothing has to
// be resumed, because a live delivery carries the whole state of its region
// rather than an increment, so a missed one costs nothing and boundary ids are
// reproduced by position.
func (o Options) RenderLiveStream(ctx context.Context, w http.ResponseWriter, r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	return o.renderStream(ctx, w, r, true, wrappers, leaf, options)
}

// renderStream is the body behind both streaming entries. serveLive says whether
// this caller answers the live mode at all; the request says whether it asked to.
func (o Options) renderStream(ctx context.Context, w http.ResponseWriter, r *http.Request, serveLive bool, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options []htmlbind.Option) error {
	w.Header().Add("Vary", o.renderHeader())
	w.Header().Add("Vary", o.buildHeader())
	negotiated := o.Negotiate(r)
	o.markLive(w, wrappers, leaf)
	if negotiated.Mode == ModeDocument {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := delta.CollectChain(w, o.Key, wrappers, leaf, o.renderOptions(options)...)
		return err
	}
	live := serveLive && negotiated.Mode == ModeLive
	if live {
		// Only the live mode keeps subscriptions open. A navigation that did so
		// would never terminate, because a live source has no settle.
		options = append(options, htmlbind.WithLiveSubscriptions())
	}
	// The head is known before the first record, so a stylesheet a newly
	// reachable component brought is installed before its markup arrives.
	head, err := delta.DeltaStreamHead(wrappers, leaf, o.renderOptions(options)...)
	if err != nil {
		return err
	}
	var stream *DeltaStream
	if live {
		stream = o.openStream(w, ModeLive, negotiated.Version, head)
	} else {
		stream = o.openStream(w, ModeNavigation, negotiated.Version, head)
		if htmlbind.HasLiveBlock(wrappers, leaf) {
			stream.ExpectLive()
		}
	}
	for item, err := range delta.RenderDeltaStream(ctx, o.Key, negotiated.Known, wrappers, leaf, o.renderOptions(options)...) {
		if err != nil {
			// The response committed with the head record, so the status cannot
			// change and the failure has to travel in band.
			stream.Fail(err.Error())
			return stream.Close()
		}
		switch {
		case item.Completion != nil:
			stream.Settled(item.Completion.BoundaryID, item.Completion.HTML)
		case item.Operation != nil && item.Operation.HTML != "":
			stream.Replace(item.Operation.InstanceID, item.Operation.HTML, item.Frame)
		case item.Operation != nil && live:
			// A live client already holds this boundary from the document render,
			// and its manifest is not rebuilt from a delivery stream. Restating a
			// validator it is not going to replace buys nothing, so an unchanged
			// boundary costs no record here.
		case item.Operation != nil:
			stream.Unchanged(item.Operation.InstanceID, item.Frame)
		}
	}
	// A cancelled context ends the sequence silently, and on the live path that
	// is almost always the server going away: a shutdown, a rolling deploy, a
	// request the caller bounded. Closing it done would tell the client every
	// source finished and it should stop, so a deploy would leave every open
	// screen frozen until somebody reloaded. Closing it retry says what actually
	// happened.
	//
	// The client aborting its own request lands here too, and writing the record
	// costs nothing there: nobody is reading it.
	if live && ctx.Err() != nil {
		return stream.Retry(0)
	}
	return stream.Close()
}

// RenderStream answers a navigation with a record stream instead of one
// buffered body, so each region applies as soon as it is written.
//
// Everything that could change the status is decided before the first record,
// because writing it commits the response. After that a failure can only be
// reported in band.
func (o Options) RenderStream(w http.ResponseWriter, r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) error {
	w.Header().Add("Vary", o.renderHeader())
	w.Header().Add("Vary", o.buildHeader())
	negotiated := o.Negotiate(r)
	o.markLive(w, wrappers, leaf)
	if negotiated.Mode != ModeNavigation {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := delta.CollectChain(w, o.Key, wrappers, leaf, o.renderOptions(nil)...)
		return err
	}
	// Rendering happens before the first byte, so a failure here is still an
	// ordinary error the caller can turn into a status.
	diff, err := delta.RenderDelta(o.Key, negotiated.Known, wrappers, leaf, o.renderOptions(nil)...)
	if err != nil {
		return err
	}
	stream := o.openStream(w, ModeNavigation, negotiated.Version, diff.Head)
	if htmlbind.HasLiveBlock(wrappers, leaf) {
		stream.ExpectLive()
	}
	frames := map[string]string{}
	for _, instance := range diff.Manifest.Instances {
		frames[instance.ID] = instance.FrameValidator
	}
	for _, operation := range diff.Operations {
		stream.Replace(operation.InstanceID, operation.HTML, frames[operation.InstanceID])
	}
	for _, instance := range diff.Manifest.Instances {
		if stream.Sent(instance.ID) {
			continue
		}
		stream.Unchanged(instance.ID, instance.FrameValidator)
	}
	return stream.Close()
}

// recordWriter serializes records and flushes each one, so a boundary reaches
// the browser as soon as it is written rather than when the response ends.
type recordWriter struct {
	w       http.ResponseWriter
	encoder *json.Encoder
	flusher http.Flusher
	seen    map[string]struct{}
	err     error
}

func newRecordWriter(w http.ResponseWriter) *recordWriter {
	flusher, _ := w.(http.Flusher)
	return &recordWriter{w: w, encoder: json.NewEncoder(w), flusher: flusher, seen: map[string]struct{}{}}
}

func (rw *recordWriter) write(item record) {
	if rw.err != nil {
		return
	}
	if item.ID != "" {
		rw.seen[item.ID] = struct{}{}
	}
	if rw.err = rw.encoder.Encode(item); rw.err != nil {
		return
	}
	// A writer that cannot flush still produces correct output, only without
	// progressive delivery.
	if rw.flusher != nil {
		rw.flusher.Flush()
	}
}
