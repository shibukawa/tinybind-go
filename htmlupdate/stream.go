package htmlupdate

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// StreamContentType marks a delta delivered as a record stream. One JSON record
// per line, which is the framing the module already uses for streamed values.
const StreamContentType = "application/x-ndjson; charset=utf-8"

// record is one line of a streamed delta.
//
// Each operation carries its own manifest entry, because a trailing manifest
// cannot be written before the operations it describes. That is also why the
// stream ends with an explicit terminator: a client that stops receiving has no
// other way to tell a finished render from a truncated one.
type record struct {
	Record string `json:"r"`
	// head fields
	Version int      `json:"v,omitempty"`
	Head    []string `json:"head,omitempty"`
	// operation fields
	Kind  string `json:"kind,omitempty"`
	ID    string `json:"id,omitempty"`
	HTML  string `json:"html,omitempty"`
	Frame string `json:"frame,omitempty"`
	// terminator and directive fields
	Navigate string `json:"navigate,omitempty"`
	Error    string `json:"error,omitempty"`
}

const (
	recordHead  = "head"
	recordOp    = "op"
	recordAwait = "await"
	recordEnd   = "end"
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
}

// OpenStream commits the response and writes the head record.
//
// Everything that could change the status has to be decided before this call,
// because after it the status is fixed and a failure can only be reported in
// band through Fail.
func (o Options) OpenStream(w http.ResponseWriter, head []string) *DeltaStream {
	w.Header().Set("Content-Type", StreamContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(o.renderHeader(), modeNavigation+";v="+versionText)
	stream := &DeltaStream{writer: newRecordWriter(w)}
	stream.writer.write(record{Record: recordHead, Version: Version, Head: head})
	return stream
}

// Replace writes one settled boundary and the validator it produced.
func (s *DeltaStream) Replace(instanceID, html, frame string) {
	s.writer.write(record{Record: recordOp, Kind: htmlbind.OpReplace, ID: instanceID, HTML: html, Frame: frame})
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
	s.writer.write(record{Record: recordEnd, Error: message})
	s.closed = true
}

// Close writes the terminator. Without it the client treats the stream as
// truncated and discards its manifest, so a producer must always reach here.
func (s *DeltaStream) Close() error {
	if !s.closed {
		s.writer.write(record{Record: recordEnd})
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
func (o Options) RenderStreamAsync(ctx context.Context, w http.ResponseWriter, r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	w.Header().Add("Vary", o.renderHeader())
	w.Header().Add("Vary", o.buildHeader())
	negotiated := o.Negotiate(r)
	if negotiated.Mode != ModeNavigation {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := htmlbind.CollectChain(w, o.Key, wrappers, leaf, options...)
		return err
	}
	// The head is known before the first record, so a stylesheet a newly
	// reachable component brought is installed before its markup arrives.
	head, err := htmlbind.DeltaStreamHead(wrappers, leaf, options...)
	if err != nil {
		return err
	}
	stream := o.OpenStream(w, head)
	for item, err := range htmlbind.RenderDeltaStream(ctx, o.Key, negotiated.Known, wrappers, leaf, options...) {
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
		case item.Operation != nil:
			stream.Unchanged(item.Operation.InstanceID, item.Frame)
		}
	}
	return stream.Close()
}

// RenderLiveStream is RenderStreamAsync for a chain holding live sources: it
// keeps every subscription open and writes each delivery as it arrives.
//
// Reconnecting after a dropped stream is the same request again. Nothing has to
// be resumed, because a live delivery carries the whole state of its region
// rather than an increment, so a missed one costs nothing and boundary ids are
// reproduced by position.
func (o Options) RenderLiveStream(ctx context.Context, w http.ResponseWriter, r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	return o.RenderStreamAsync(ctx, w, r, wrappers, leaf, append(options, htmlbind.WithLiveSubscriptions())...)
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
	if negotiated.Mode != ModeNavigation {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := htmlbind.CollectChain(w, o.Key, wrappers, leaf)
		return err
	}
	// Rendering happens before the first byte, so a failure here is still an
	// ordinary error the caller can turn into a status.
	delta, err := htmlbind.RenderDelta(o.Key, negotiated.Known, wrappers, leaf)
	if err != nil {
		return err
	}
	stream := o.OpenStream(w, delta.Head)
	frames := map[string]string{}
	for _, instance := range delta.Manifest.Instances {
		frames[instance.ID] = instance.FrameValidator
	}
	for _, operation := range delta.Operations {
		stream.Replace(operation.InstanceID, operation.HTML, frames[operation.InstanceID])
	}
	for _, instance := range delta.Manifest.Instances {
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
