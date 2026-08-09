package httpbind

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
)

// StreamFormat is the negotiated on-the-wire format for Stream[T].
type StreamFormat = bindcore.StreamFormat

const (
	// StreamSSE is text/event-stream (data: <json>\n\n).
	StreamSSE = bindcore.StreamSSE
	// StreamNDJSON is application/x-ndjson (one JSON object per line).
	// Same family as JSONL / NDJSON; not a single JSON array document.
	StreamNDJSON = bindcore.StreamNDJSON
	// StreamJSONArray is application/json as one JSON array document:
	// [obj1,obj2,...] with items appended incrementally and closed by Close.
	StreamJSONArray = bindcore.StreamJSONArray
)

// Stream is a typed incremental response stream.
//
// Handler usage:
//
//	httpbind.WriteStream(w, r, func(s *httpbind.Stream[ChatEvent]) error {
//		if err := s.Write(ChatEvent{Type: "delta", Delta: "hi"}); err != nil {
//			return err
//		}
//		return s.Write(ChatEvent{Type: "done"})
//	})
//
// Format (SSE vs NDJSON vs JSON array) is chosen once by rule:stream-content-negotiation.
// Write may be called many times; headers and status are sent when the stream
// opens. WriteStream closes the stream, which is what writes the trailing ']'
// of the JSON array framing.
//
// The framing lives in one place shared with the fasthttp runtime, so the same
// events produce the same bytes on either transport.
//
// Events are encoded through the jsonbind codec registry: T must have a
// generated encoder (or one registered manually via jsonbind.RegisterEncode).
type Stream[T any] = bindcore.Stream[T]

// WriteStream opens a negotiated stream, runs fn against it, and closes it.
//
// It returns nothing. fn runs after the handler has returned on the fasthttp
// runtime, where an error cannot travel back to handler code, so neither
// surface offers one and the same handler source works on both.
//
// A failure to open — before any byte is committed — becomes an ordinary
// Problem response. Once the stream is open the status is already sent, so an
// error from fn reaches the handler installed with SetStreamErrorHandler
// instead. Close runs either way, which is what keeps a JSON array document
// terminated when fn fails halfway through it.
func WriteStream[T any](w http.ResponseWriter, r *http.Request, fn func(*Stream[T]) error) {
	s, err := NewStream[T](w, r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	ferr := fn(s)
	cerr := s.Close()
	if ferr == nil {
		ferr = cerr
	}
	if ferr != nil {
		reportStreamError(ferr)
	}
}

// SetStreamErrorHandler installs the destination for stream failures that
// happen after the response status has been sent. Passing nil discards them,
// which is the default: a runtime that logged on its own would be writing to a
// destination the caller did not choose.
//
// The handler is shared with the fasthttp runtime, so installing it once
// covers both.
func SetStreamErrorHandler(fn func(error)) { bindcore.SetStreamErrorHandler(fn) }

func reportStreamError(err error) { bindcore.ReportStreamError(err) }

// NewStream negotiates transport format from the request, writes response
// headers and 200 once, and returns a stream for incremental Write calls.
//
// Deprecated: use [WriteStream]. A caller-held stream has no fasthttp
// transcription, and it makes the trailing ']' of the JSON array framing
// depend on the caller remembering to defer Close.
func NewStream[T any](w http.ResponseWriter, r *http.Request) (*Stream[T], error) {
	if w == nil {
		return nil, BadRequest(Problem{Code: "stream", Message: "nil ResponseWriter"})
	}
	format := NegotiateStreamFormat(r)
	for _, h := range bindcore.StreamHeaders(format) {
		w.Header().Set(h.Name, h.Value)
	}
	w.WriteHeader(http.StatusOK)
	return bindcore.NewStream[T](w, format), nil
}

// NegotiateStreamFormat selects SSE, NDJSON, or JSON array using:
//  1. ?stream= query
//  2. Accept
//  3. User-Agent heuristics
//  4. default NDJSON
//
// Exported for tests and advanced callers.
func NegotiateStreamFormat(r *http.Request) StreamFormat {
	if r == nil {
		return StreamNDJSON
	}
	streamQuery := ""
	if r.URL != nil {
		streamQuery = r.URL.Query().Get("stream")
	}
	return bindcore.NegotiateStream(streamQuery, r.Header.Get("Accept"), r.Header.Get("User-Agent"))
}
