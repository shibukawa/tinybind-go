package httpbind

import (
	"errors"
	"net/http"
	"strings"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

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

// Stream is a typed incremental response stream.
//
// Ideal handler usage:
//
//	stream, err := httpbind.NewStream[ChatEvent](w, r)
//	if err != nil { ... }
//	defer stream.Close()
//	_ = stream.Write(ChatEvent{Type: "delta", Delta: "hi"})
//	_ = stream.Write(ChatEvent{Type: "done"})
//
// Format (SSE vs NDJSON vs JSON array) is chosen once by rule:stream-content-negotiation.
// Write may be called many times; headers/status are sent only in NewStream.
// JSON array framing requires Close (via defer) so the trailing ']' is written.
//
// Events are encoded through the jsonbind codec registry: T must have a
// generated encoder (or one registered manually via jsonbind.RegisterEncode).
type Stream[T any] struct {
	w       http.ResponseWriter
	format  StreamFormat
	closed  bool
	started bool // JSON array: '[' already written
}

// NewStream negotiates transport format from the request, writes response
// headers and 200 once, and returns a stream for incremental Write calls.
func NewStream[T any](w http.ResponseWriter, r *http.Request) (*Stream[T], error) {
	if w == nil {
		return nil, BadRequest(Problem{Code: "stream", Message: "nil ResponseWriter"})
	}
	format := NegotiateStreamFormat(r)
	s := &Stream[T]{
		w:      w,
		format: format,
	}
	switch format {
	case StreamSSE:
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// Disable proxy buffering when supported (nginx etc.).
		w.Header().Set("X-Accel-Buffering", "no")
	case StreamJSONArray:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
	default: // NDJSON / JSONL
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.WriteHeader(http.StatusOK)
	return s, nil
}

// Format returns the negotiated stream format (sse | ndjson | json-array).
func (s *Stream[T]) Format() StreamFormat {
	if s == nil {
		return ""
	}
	return s.format
}

var (
	errNilStream    = errors.New("httpbind: nil stream")
	errStreamClosed = errors.New("httpbind: stream closed")
)

var (
	ssePrefix     = []byte("data: ")
	sseNewline    = []byte("\n")
	arrayOpen     = []byte("[")
	arrayComma    = []byte(",")
	arrayClose    = []byte("]")
	arrayEmptyDoc = []byte("[]")
)

// Write encodes one event in the negotiated format.
// Callable many times; does not re-send HTTP status or headers.
//
// Generated encoders terminate each event with a single '\n', which supplies
// the NDJSON line ending and the first newline of the SSE frame.
func (s *Stream[T]) Write(v T) error {
	if s == nil {
		return Internal(errNilStream)
	}
	if s.closed {
		return Internal(errStreamClosed)
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
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

// Close marks the stream finished. Idempotent.
// For JSON array format, Close writes the trailing ']' (or "[]" if no Write).
// SSE and NDJSON do not require a special trailer; still call Close for symmetry.
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
		if f, ok := s.w.(http.Flusher); ok {
			f.Flush()
		}
	}
	return nil
}

// NegotiateStreamFormat selects SSE, NDJSON, or JSON array using:
//  1. ?stream= query
//  2. Accept
//  3. User-Agent heuristics
//  4. default NDJSON
//
// Exported for tests and advanced callers.
//
// Note: NDJSON/JSONL (line-delimited objects) is distinct from JSON array
// (a single [...] document). application/json selects the array form;
// application/x-ndjson / application/jsonl select NDJSON.
func NegotiateStreamFormat(r *http.Request) StreamFormat {
	if r == nil {
		return StreamNDJSON
	}

	// 1) stream query parameter
	if q := strings.TrimSpace(r.URL.Query().Get("stream")); q != "" {
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
	if accept := r.Header.Get("Accept"); accept != "" {
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
	ua := strings.ToLower(r.Header.Get("User-Agent"))
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
