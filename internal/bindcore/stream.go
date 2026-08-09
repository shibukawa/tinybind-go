package bindcore

import (
	"errors"
	"io"
	"strings"
	"sync/atomic"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// The stream lives here whole. Its framing is the wire contract — SSE prefixes,
// the array's commas and trailing bracket, the NDJSON line endings — and two
// transports emitting that from two implementations would be two chances to
// disagree. Each surface aliases this type rather than reimplementing it.
//
// Nothing here names a transport: the stream writes to an io.Writer and flushes
// through whichever Flush method the writer happens to have, which is satisfied
// by an http.ResponseWriter and by the *bufio.Writer fasthttp hands a body
// stream writer.

// StreamFormat is the negotiated on-the-wire format for Stream[T].
type StreamFormat string

const (
	// StreamSSE is text/event-stream (data: <json>\n\n).
	StreamSSE StreamFormat = "sse"
	// StreamNDJSON is application/x-ndjson (one JSON object per line).
	// Same family as JSONL / NDJSON; not a single JSON array document.
	StreamNDJSON StreamFormat = "ndjson"
	// StreamJSONArray is application/json as one JSON array document:
	// [obj1,obj2,...] with items appended incrementally and closed by Close.
	StreamJSONArray StreamFormat = "json-array"
)

var (
	ErrNilStream    = errors.New("httpbind: nil stream")
	ErrStreamClosed = errors.New("httpbind: stream closed")
)

var (
	ssePrefix     = []byte("data: ")
	sseNewline    = []byte("\n")
	arrayOpen     = []byte("[")
	arrayComma    = []byte(",")
	arrayClose    = []byte("]")
	arrayEmptyDoc = []byte("[]")
)

// streamErrorHandler receives failures raised after a stream has committed its
// status, where no response code is left to carry them. It lives here so a
// process installing one covers both transports, the same reason the multipart
// limit does.
var streamErrorHandler atomic.Pointer[func(error)]

// SetStreamErrorHandler installs the destination for post-commit stream
// failures. Passing nil discards them, which is the default: a runtime logging
// on its own would write to a destination the caller did not choose.
func SetStreamErrorHandler(fn func(error)) {
	if fn == nil {
		streamErrorHandler.Store(nil)
		return
	}
	streamErrorHandler.Store(&fn)
}

// ReportStreamError hands err to the installed handler, if any.
func ReportStreamError(err error) {
	if fn := streamErrorHandler.Load(); fn != nil {
		(*fn)(err)
	}
}

// StreamHeader is one response header a format requires.
type StreamHeader struct{ Name, Value string }

// StreamHeaders lists the headers a format's response opens with, in order, so
// both surfaces send the same set.
func StreamHeaders(format StreamFormat) []StreamHeader {
	switch format {
	case StreamSSE:
		return []StreamHeader{
			{"Content-Type", "text/event-stream; charset=utf-8"},
			{"Cache-Control", "no-cache"},
			{"Connection", "keep-alive"},
			// Disable proxy buffering when supported (nginx etc.).
			{"X-Accel-Buffering", "no"},
		}
	case StreamJSONArray:
		return []StreamHeader{
			{"Content-Type", "application/json; charset=utf-8"},
			{"Cache-Control", "no-cache"},
		}
	default: // NDJSON / JSONL
		return []StreamHeader{
			{"Content-Type", "application/x-ndjson; charset=utf-8"},
			{"Cache-Control", "no-cache"},
		}
	}
}

// Stream is a typed incremental response stream over any writer.
type Stream[T any] struct {
	w       io.Writer
	format  StreamFormat
	closed  bool
	started bool // JSON array: '[' already written
}

// NewStream builds a stream over w. The caller has already sent the headers
// StreamHeaders lists and the 200 status.
func NewStream[T any](w io.Writer, format StreamFormat) *Stream[T] {
	return &Stream[T]{w: w, format: format}
}

// Format returns the negotiated stream format (sse | ndjson | json-array).
func (s *Stream[T]) Format() StreamFormat {
	if s == nil {
		return ""
	}
	return s.format
}

// Write encodes one event in the negotiated format.
// Callable many times; does not re-send HTTP status or headers.
//
// Generated encoders terminate each event with a single '\n', which supplies
// the NDJSON line ending and the first newline of the SSE frame.
func (s *Stream[T]) Write(v T) error {
	if s == nil {
		return Internal(ErrNilStream)
	}
	if s.closed {
		return Internal(ErrStreamClosed)
	}
	switch s.format {
	case StreamSSE:
		if _, err := s.w.Write(ssePrefix); err != nil {
			return err
		}
		if err := jsonbind.EncodeJSON(s.w, v); err != nil {
			return err
		}
		if _, err := s.w.Write(sseNewline); err != nil {
			return err
		}
	case StreamJSONArray:
		sep := arrayComma
		if !s.started {
			sep = arrayOpen
			s.started = true
		}
		if _, err := s.w.Write(sep); err != nil {
			return err
		}
		if err := jsonbind.EncodeJSON(s.w, v); err != nil {
			return err
		}
	default: // NDJSON: the encoder's trailing '\n' ends the line
		if err := jsonbind.EncodeJSON(s.w, v); err != nil {
			return err
		}
	}
	flush(s.w)
	return nil
}

// Close marks the stream finished. Idempotent.
// For JSON array format, Close writes the trailing ']' (or "[]" if no Write).
// SSE and NDJSON do not require a special trailer.
func (s *Stream[T]) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	if s.format == StreamJSONArray {
		var err error
		if !s.started {
			_, err = s.w.Write(arrayEmptyDoc)
		} else {
			_, err = s.w.Write(arrayClose)
		}
		if err != nil {
			return err
		}
		flush(s.w)
	}
	return nil
}

// flush pushes buffered bytes toward the client when the writer can. The two
// shapes cover http.Flusher and *bufio.Writer without naming either.
func flush(w io.Writer) {
	if f, ok := w.(interface{ Flush() }); ok {
		f.Flush()
		return
	}
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}

// NegotiateStream selects SSE, NDJSON, or JSON array using:
//  1. the ?stream= query value
//  2. Accept
//  3. User-Agent heuristics
//  4. default NDJSON
//
// It takes the three values as strings so each transport reads them its own
// way and the decision itself stays in one place.
//
// Note: NDJSON/JSONL (line-delimited objects) is distinct from JSON array
// (a single [...] document). application/json selects the array form;
// application/x-ndjson / application/jsonl select NDJSON.
func NegotiateStream(streamQuery, accept, userAgent string) StreamFormat {
	// 1) stream query parameter
	if q := strings.TrimSpace(streamQuery); q != "" {
		switch strings.ToLower(q) {
		case "sse", "event-stream", "events", "eventstream":
			return StreamSSE
		case "ndjson", "jsonl", "nd", "lines":
			return StreamNDJSON
		case "json", "array", "json-array", "jsonarray":
			return StreamJSONArray
		}
	}

	// 2) Accept — first matching media type wins (left to right).
	if accept != "" {
		for part := range strings.SplitSeq(accept, ",") {
			media := strings.TrimSpace(strings.Split(part, ";")[0])
			media = strings.ToLower(media)
			switch media {
			case "text/event-stream":
				return StreamSSE
			case "application/x-ndjson", "application/ndjson", "application/jsonl":
				return StreamNDJSON
			case "application/json":
				// Full JSON array document (not JSONL).
				return StreamJSONArray
			}
		}
	}

	// 3) User-Agent
	ua := strings.ToLower(userAgent)
	if ua != "" {
		if isBrowserUA(ua) {
			return StreamSSE
		}
		if strings.Contains(ua, "curl") || strings.Contains(ua, "wget") || strings.Contains(ua, "httpie") {
			return StreamNDJSON
		}
	}

	// 4) default — curl-friendly NDJSON (JSONL-style lines)
	return StreamNDJSON
}

func isBrowserUA(ua string) bool {
	// Common browser tokens. Avoid matching "curl" which never appears here.
	return strings.Contains(ua, "mozilla/") ||
		strings.Contains(ua, "chrome/") ||
		strings.Contains(ua, "safari/") ||
		strings.Contains(ua, "firefox/") ||
		strings.Contains(ua, "edg/") ||
		strings.Contains(ua, "applewebkit")
}
