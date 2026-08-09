package updatecore

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
	"github.com/shibukawa/tinybind-go/internal/bindcore"
)

// The post-commit error destination is bindcore's, not a second one. A stream
// that has written its head record cannot answer with a status, and a
// deployment installing one handler expects it to cover every stream the module
// opens rather than the typed ones only.

// SetStreamErrorHandler installs the destination for stream failures raised
// after the response committed.
func SetStreamErrorHandler(fn func(error)) { bindcore.SetStreamErrorHandler(fn) }

// ReportStreamError sends a post-commit failure to the installed handler.
func ReportStreamError(err error) { bindcore.ReportStreamError(err) }

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
	// Boundaries names the holes in HTML. One that also arrives as an operation
	// is filled from it; one that does not is retained from the DOM the client
	// already has, which is what keeps the state inside it.
	Boundaries []string `json:"boundaries,omitempty"`
	// Children digests the nested boundary ids of the instance this record names,
	// and Parent names the boundary enclosing it. With the frame they are the
	// whole of a manifest entry, so a client rebuilding one from a stream returns
	// what the next request is compared against rather than two thirds of it.
	Children string `json:"children,omitempty"`
	Parent   string `json:"parent,omitempty"`
	// Seq and Values are the fragment split into its static and varying halves,
	// sent in place of HTML to a client that walks sequences.
	Seq    string   `json:"seq,omitempty"`
	Values []string `json:"values,omitempty"`
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

// StreamPlan is everything a stream entry decides before it commits.
//
// It exists because one transport cannot read its request after the response is
// open. fasthttp writes a streamed body from a callback that runs once the
// handler has returned, and forbids touching the request context from inside
// it, so everything the delivery needs has to be captured while the handler
// still owns the request. Planning first is what makes the same loop run under
// both transports rather than two loops agreeing.
type StreamPlan struct {
	// Mode is what was negotiated. ModeDocument means this request gets the
	// complete document and no stream is opened at all.
	Mode Mode
	// Version is the wire version the request claimed, echoed in the terminator.
	Version int
	// Head is the merged head of the composition, written as the first record so
	// a stylesheet lands before the markup that needs it.
	Head []string
	// Live says this stream holds subscriptions open, which decides whether the
	// terminator can mean "come back".
	Live bool
	// Sequences says the client can walk a sequence tree, so a fragment may
	// travel as an address and its values instead of as markup.
	Sequences bool
	// ExpectLive marks a navigation whose composition owns a live boundary, so
	// the client knows a live request is worth issuing.
	ExpectLive bool
	// Known is what the client already holds.
	Known delta.Manifest
	// Options are the render options resolved for this stream, caller's last.
	Options []htmlbind.Option
	// Buffered is the whole delta, for the entry that renders before it writes.
	// Nil for the streaming entries, which render as they go.
	Buffered *delta.Delta
}

// Streams reports whether this plan opens a stream at all.
func (p StreamPlan) Streams() bool { return p.Mode != ModeDocument }

// OpenStream writes the head record and returns the stream the records go to.
//
// Everything that could change the status has to be decided before this call,
// because after it the status is fixed and a failure can only be reported in
// band through Fail.
func (o Options) OpenStream(w io.Writer, plan StreamPlan) *DeltaStream {
	ending := endFinal
	if plan.Live {
		ending = endDone
	}
	stream := &DeltaStream{writer: newRecordWriter(w), ending: ending}
	stream.writer.write(record{Record: recordHead, Head: plan.Head, Build: o.Build()})
	if plan.ExpectLive {
		stream.ExpectLive()
	}
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

// Replace writes one settled boundary, the validator it produced, and the
// nested boundaries appearing as holes in its markup.
//
// A hole whose id also arrives as an operation on this stream is filled from it;
// one that does not is a region the client already holds and moves in. The list
// is what separates the two, since nothing in the markup does.
func (s *DeltaStream) Replace(instanceID, html string, entry ManifestEntry, boundaries ...string) {
	s.writer.write(record{
		Record: recordOp, Kind: delta.OpReplace, ID: instanceID, HTML: html,
		Frame: entry.Frame, Children: entry.Children, Parent: entry.Parent,
		Boundaries: boundaries,
	})
}

// ManifestEntry is what a client stores for one instance beside its markup: the
// validator of the region's own bytes, the digest of its nested boundary ids,
// and the boundary enclosing it. All three travel on every operation record,
// because a client rebuilding its manifest from a stream has no other source for
// them and the next request is compared against all three.
type ManifestEntry struct {
	Frame    string
	Children string
	Parent   string
}

// Children writes a boundary whose own markup is unchanged and whose nested
// boundaries are now these, in this order.
//
// It carries no markup, which is the point: appending one row to a list costs
// the list of ids rather than the list of holes. A client keeps what the list
// keeps, moving what moved, drops what it omits, and fills what arrives as its
// own operation in the same response.
func (s *DeltaStream) Children(instanceID string, entry ManifestEntry, boundaries ...string) {
	s.writer.write(record{
		Record: recordOp, Kind: delta.OpChildren, ID: instanceID,
		Frame: entry.Frame, Children: entry.Children, Parent: entry.Parent,
		Boundaries: boundaries,
	})
}

// ReplaceValues is Replace for a client that walks sequences: the fragment
// travels as the address of its static half and the values that fill it, so the
// statics cost one response per client rather than one per render.
func (s *DeltaStream) ReplaceValues(instanceID, sequence string, values []string, entry ManifestEntry, boundaries ...string) {
	s.writer.write(record{
		Record: recordOp, Kind: delta.OpReplace, ID: instanceID,
		Seq: sequence, Values: values,
		Frame: entry.Frame, Children: entry.Children, Parent: entry.Parent,
		Boundaries: boundaries,
	})
}

// Unchanged restates a boundary's validator without markup, so the client can
// rebuild its whole manifest from what it received.
func (s *DeltaStream) Unchanged(instanceID string, entry ManifestEntry) {
	s.writer.write(record{
		Record: recordOp, ID: instanceID,
		Frame: entry.Frame, Children: entry.Children, Parent: entry.Parent,
	})
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

// PlanStream decides everything a streamed render can decide before it writes.
//
// serveLive says whether the caller's entry holds subscriptions open. A live
// request reaching one that does not is planned as a navigation and terminated,
// so a client that opened a live connection to a route this caller does not
// serve live learns so at once instead of holding a connection that will never
// deliver.
//
// A failure here is an ordinary error: nothing is written yet, so the caller can
// still choose a status and serve an error page. That window exists on both
// transports, because this runs while the handler still owns the request.
func (o Options) PlanStream(r Reader, serveLive bool, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options []htmlbind.Option) (StreamPlan, error) {
	negotiated := o.Negotiate(r)
	plan := StreamPlan{
		Mode:      negotiated.Mode,
		Version:   negotiated.Version,
		Sequences: o.WantsSequences(r),
		Known:     negotiated.Known,
	}
	if !plan.Streams() {
		plan.Options = o.RenderOptions(options)
		return plan, nil
	}
	plan.Live = serveLive && negotiated.Mode == ModeLive
	if plan.Live {
		// Only the live mode keeps subscriptions open. A navigation that did so
		// would never terminate, because a live source has no settle.
		options = append(options, htmlbind.WithLiveSubscriptions())
	} else {
		plan.ExpectLive = htmlbind.HasLiveBlock(wrappers, leaf)
	}
	plan.Options = o.RenderOptions(options)
	// The head is known before the first record, so a stylesheet a newly
	// reachable component brought is installed before its markup arrives.
	head, err := delta.DeltaStreamHead(wrappers, leaf, plan.Options...)
	if err != nil {
		return StreamPlan{}, err
	}
	plan.Head = head
	return plan, nil
}

// RunStream writes the records of a planned stream and terminates it.
//
// It reads nothing from the request: the plan carries everything, which is what
// lets this run from a body stream writer on the transport that forbids
// touching the request there.
//
// ctx is the caller's cancellation, not the request's. On net/http those are
// usually the same value; on fasthttp they cannot be, because a RequestCtx is
// pooled and this outlives the handler that owned it.
func (o Options) RunStream(ctx context.Context, stream *DeltaStream, plan StreamPlan, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) error {
	for item, err := range delta.RenderDeltaStream(ctx, o.Key, plan.Known, wrappers, leaf, plan.Options...) {
		if err != nil {
			// The response committed with the head record, so the status cannot
			// change and the failure has to travel in band.
			stream.Fail(err.Error())
			return stream.Close()
		}
		if stream.writer.broken() {
			// Nobody is reading. On net/http the context is usually cancelled
			// too; on fasthttp this is the only signal there is, and continuing
			// would render a live subscription into a closed socket forever.
			return stream.writer.err
		}
		switch {
		case item.Completion != nil:
			stream.Settled(item.Completion.BoundaryID, item.Completion.HTML)
		case item.Operation == nil:
			// Nothing to write.
		case item.Operation.Kind == delta.OpChildren:
			// A children operation carries no markup by design, so it has to be
			// recognised by its kind. Deciding by whether markup is present put
			// it in the unchanged shape on a navigation — leaving an appended row
			// with nowhere to go — and dropped it entirely on the live path,
			// where an unchanged boundary is deliberately silent.
			stream.Children(item.Operation.InstanceID, entryOf(item), item.Operation.Boundaries...)
		case item.Operation.Kind == delta.OpReplace:
			if SendsValues(plan.Sequences, *item.Operation) {
				stream.ReplaceValues(item.Operation.InstanceID, item.Operation.Sequence,
					item.Operation.Values, entryOf(item), item.Operation.Boundaries...)
			} else {
				stream.Replace(item.Operation.InstanceID, item.Operation.HTML, entryOf(item),
					item.Operation.Boundaries...)
			}
		case plan.Live:
			// A live client already holds this boundary from the document render,
			// and its manifest is not rebuilt from a delivery stream. Restating a
			// validator it is not going to replace buys nothing, so an unchanged
			// boundary costs no record here.
		default:
			stream.Unchanged(item.Operation.InstanceID, entryOf(item))
		}
	}
	// A cancelled context ends the sequence silently, and on the live path that
	// is almost always the server going away: a shutdown, a rolling deploy, a
	// request the caller bounded. Closing it done would tell the client every
	// source finished and it should stop, so a deploy would leave every open
	// screen frozen until somebody reloaded. Closing it retry says what actually
	// happened.
	//
	// The client aborting its own request lands here too on net/http, and
	// writing the record costs nothing there: nobody is reading it. On fasthttp
	// a client that left is seen as a write failure above rather than here.
	if plan.Live && ctx.Err() != nil {
		return stream.Retry(0)
	}
	return stream.Close()
}

// PlanBufferedStream is PlanStream for the entry that renders the whole delta
// before it writes anything, so every failure it can have is pre-commit.
//
// It serves the navigation mode only. Anything else is planned as a document,
// because a buffered render cannot hold a delivery open.
func (o Options) PlanBufferedStream(r Reader, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options []htmlbind.Option) (StreamPlan, error) {
	negotiated := o.Negotiate(r)
	plan := StreamPlan{Version: negotiated.Version, Options: o.RenderOptions(options)}
	if negotiated.Mode != ModeNavigation {
		return plan, nil
	}
	plan.Mode = ModeNavigation
	// Rendering happens before the first byte, so a failure here is still an
	// ordinary error the caller can turn into a status.
	diff, err := delta.RenderDelta(o.Key, negotiated.Known, wrappers, leaf, plan.Options...)
	if err != nil {
		return StreamPlan{}, err
	}
	plan.Buffered = &diff
	plan.Head = diff.Head
	plan.ExpectLive = htmlbind.HasLiveBlock(wrappers, leaf)
	return plan, nil
}

// RunBufferedStream writes an already-rendered delta as records.
func (o Options) RunBufferedStream(stream *DeltaStream, plan StreamPlan) error {
	diff := plan.Buffered
	if diff == nil {
		return stream.Close()
	}
	entries := map[string]ManifestEntry{}
	for _, instance := range diff.Manifest.Instances {
		entries[instance.ID] = ManifestEntry{
			Frame: instance.FrameValidator, Children: instance.ChildrenValidator,
			Parent: instance.ParentID,
		}
	}
	for _, operation := range diff.Operations {
		// By kind, not by whether markup happens to be present: writing a
		// children operation as a replace would have emptied the region rather
		// than reordering it, since a children operation carries no markup.
		if operation.Kind == delta.OpChildren {
			stream.Children(operation.InstanceID, entries[operation.InstanceID], operation.Boundaries...)
			continue
		}
		stream.Replace(operation.InstanceID, operation.HTML, entries[operation.InstanceID], operation.Boundaries...)
	}
	for _, instance := range diff.Manifest.Instances {
		if stream.Sent(instance.ID) {
			continue
		}
		stream.Unchanged(instance.ID, entries[instance.ID])
	}
	return stream.Close()
}

// RenderDocument writes the complete document, which is what a request with no
// usable render header gets and what every stream entry falls back to.
//
// The document render collects so every boundary carries its instance
// attribute; without them a later delta could not find its targets.
func (o Options) RenderDocument(w io.Writer, plan StreamPlan, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) error {
	_, err := delta.CollectChain(w, o.Key, wrappers, leaf, plan.Options...)
	return err
}

// entryOf is the manifest entry a streamed record carries beside its operation.
func entryOf(item delta.DeltaRecord) ManifestEntry {
	return ManifestEntry{Frame: item.Frame, Children: item.Children, Parent: item.Parent}
}

// recordWriter serializes records and flushes each one, so a boundary reaches
// the browser as soon as it is written rather than when the response ends.
//
// It takes an io.Writer and duck-types the flush, because the two transports
// present progressive delivery differently and neither presents it as an
// interface the other has: net/http asserts http.Flusher on the response
// writer, and fasthttp hands a *bufio.Writer to its body stream writer. Both
// spell it Flush; only the return type differs.
type recordWriter struct {
	w       io.Writer
	encoder *json.Encoder
	flush   func()
	seen    map[string]struct{}
	err     error
}

func newRecordWriter(w io.Writer) *recordWriter {
	rw := &recordWriter{w: w, encoder: json.NewEncoder(w), seen: map[string]struct{}{}}
	switch f := w.(type) {
	case interface{ Flush() }:
		rw.flush = f.Flush
	case interface{ Flush() error }:
		// A flush error is the connection going away, which the next write
		// reports anyway; recording it here would report it twice.
		rw.flush = func() { _ = f.Flush() }
	}
	return rw
}

// broken reports that writing has already failed, which is the only signal
// either transport gives that nobody is reading.
//
// net/http cancels the request context when the client disconnects and fasthttp
// does not — its Done channel closes on server shutdown alone — so a write
// failure is the one disconnect signal both transports have. A render that
// ignored it would keep producing for a client that left, which on a live
// stream means forever.
func (rw *recordWriter) broken() bool { return rw.err != nil }

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
	if rw.flush != nil {
		rw.flush()
	}
}

// Render answers one request with either a complete document or a delta.
//
// It buffers: the delta path encodes one JSON body and the document path writes
// the collected chain, so nothing is held open and every failure is an ordinary
// error. That is why this entry needs no inversion on either transport — the
// report that grouped it with the streaming entries was reading the name rather
// than the writes.
func (o Options) Render(w io.Writer, r Reader, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options []htmlbind.Option) error {
	negotiated := o.Negotiate(r)
	if negotiated.Mode == ModeNavigation {
		return o.renderDelta(w, r, negotiated, wrappers, leaf, options)
	}
	// This entry buffers, so it cannot hold a delivery stream open. A live
	// request reaching it is answered with the document, which is the same
	// fallback every unrecognized condition takes and leaves the client with a
	// working page rather than an error.
	//
	// The document render collects so every boundary carries its instance
	// attribute; without them a later delta could not find its targets.
	_, err := delta.CollectChain(w, o.Key, wrappers, leaf, o.RenderOptions(options)...)
	return err
}

// renderDelta writes the buffered navigation delta, and the handoff marker for a
// chain that owns a live boundary.
//
// A live request re-executes the route, its layouts, and its page, so a client
// that cannot tell a live page from a static one pays a full page execution per
// screen that never had a live boundary. The marker is therefore a cost control
// rather than tidiness, and it travels in the body because a delta reuses the
// document shell and never sees this response's head.
func (o Options) renderDelta(w io.Writer, r Reader, negotiated Negotiated, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options []htmlbind.Option) error {
	sequences := o.WantsSequences(r)
	diff, err := delta.RenderDelta(o.Key, negotiated.Known, wrappers, leaf, o.RenderOptions(options)...)
	if err != nil {
		// Nothing has been written yet, so the caller can still choose a status
		// and serve an ordinary error page.
		return err
	}
	body := DeltaResponse{}
	for _, operation := range diff.Operations {
		body.Operations = append(body.Operations, o.OperationBody(operation, sequences))
	}
	for _, instance := range diff.Manifest.Instances {
		body.Manifest = append(body.Manifest, DeltaInstance{
			ID: instance.ID, Frame: instance.FrameValidator,
			Children: instance.ChildrenValidator, Parent: instance.ParentID,
		})
	}
	body.Head = diff.Head
	// A navigation can arrive at a route whose composition owns a live boundary,
	// and the client reused its document shell, so this body is the only place
	// that can tell it so.
	body.Live = htmlbind.HasLiveBlock(wrappers, leaf)
	return json.NewEncoder(w).Encode(body)
}
