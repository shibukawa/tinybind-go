package httpbind_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

type evt struct {
	Type string `json:"type"`
	N    int    `json:"n"`
}

// Mirrors the encoder shape the generator emits for stream event types.
func init() {
	jsonbind.RegisterEncode[evt](func(w io.Writer, v evt) error {
		buf := jsonbind.GetBuffer()
		b := append((*buf)[:0], `{"type":`...)
		b = jsonbind.AppendString(b, v.Type)
		b = append(b, `,"n":`...)
		b = jsonbind.AppendInt(b, int64(v.N))
		b = append(b, '}', '\n')
		*buf = b
		_, err := w.Write(b)
		jsonbind.PutBuffer(buf)
		return err
	})
}

// The held-stream entry is gone, so these drive the callback instead. What each
// case actually asserts is the negotiated format and the bytes it framed, and
// neither of those moved: the entry opens the stream exactly where the removed
// one did.
func streamCase(t *testing.T, req *http.Request, body func(*httpbind.Stream[evt]) error) (*httptest.ResponseRecorder, httpbind.StreamFormat) {
	t.Helper()
	rec := httptest.NewRecorder()
	var format httpbind.StreamFormat
	httpbind.WriteStream(rec, req, func(s *httpbind.Stream[evt]) error {
		format = s.Format()
		if body == nil {
			return nil
		}
		return body(s)
	})
	return rec, format
}

func TestWriteStream_MultipleWrites_NDJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	req.Header.Set("User-Agent", "curl/8.4.0")

	rec, format := streamCase(t, req, func(s *httpbind.Stream[evt]) error {
		if err := s.Write(evt{Type: "a", N: 1}); err != nil {
			return err
		}
		return s.Write(evt{Type: "b", N: 2})
	})
	if format != httpbind.StreamNDJSON {
		t.Fatalf("format %q", format)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "ndjson") {
		t.Fatalf("ctype %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"a"`) || !strings.Contains(body, `"type":"b"`) {
		t.Fatalf("body %s", body)
	}
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines %d %q", len(lines), body)
	}
}

func TestWriteStream_AcceptSSE(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	req.Header.Set("Accept", "text/html, text/event-stream, application/json")

	rec, format := streamCase(t, req, func(s *httpbind.Stream[evt]) error {
		_ = s.Write(evt{Type: "x", N: 1})
		return s.Write(evt{Type: "y", N: 2})
	})
	// first matching stream media type: text/html ignored, then event-stream
	if format != httpbind.StreamSSE {
		t.Fatalf("format %q", format)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("ctype %q", rec.Header().Get("Content-Type"))
	}
	if body := rec.Body.String(); strings.Count(body, "data: ") != 2 {
		t.Fatalf("sse events: %s", body)
	}
}

func TestWriteStream_JSONArray_AcceptJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	req.Header.Set("Accept", "application/json")

	rec, format := streamCase(t, req, func(s *httpbind.Stream[evt]) error {
		_ = s.Write(evt{Type: "a", N: 1})
		return s.Write(evt{Type: "b", N: 2})
	})
	if format != httpbind.StreamJSONArray {
		t.Fatalf("format %q", format)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("ctype %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	var got []evt
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("not a JSON array: %v body=%q", err, body)
	}
	if len(got) != 2 || got[0].Type != "a" || got[1].Type != "b" {
		t.Fatalf("got %#v body=%q", got, body)
	}
	// Must be a single array document, not JSONL lines.
	if strings.Count(body, "\n") > 0 && !strings.HasPrefix(strings.TrimSpace(body), "[") {
		t.Fatalf("expected array document, body=%q", body)
	}
	if body[0] != '[' || body[len(body)-1] != ']' {
		t.Fatalf("framing body=%q", body)
	}
}

// The entry closes the stream, so an empty producer still terminates the array
// document. That is the defect the held shape allowed: a caller who forgot the
// defer sent an unterminated body with a 200 on it.
func TestWriteStream_JSONArray_EmptyClose(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chat?stream=json", nil)
	rec, format := streamCase(t, req, nil)
	if format != httpbind.StreamJSONArray {
		t.Fatalf("format %q", format)
	}
	if rec.Body.String() != "[]" {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestWriteStream_JSONArray_QueryOverridesUA(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chat?stream=array", nil)
	req.Header.Set("User-Agent", "curl/8.0")
	if _, format := streamCase(t, req, nil); format != httpbind.StreamJSONArray {
		t.Fatalf("format %q", format)
	}
}

func TestWriteStream_JSONL_NotArray(t *testing.T) {
	// JSONL/NDJSON must stay line-delimited, not a JSON array.
	req := httptest.NewRequest(http.MethodGet, "/chat?stream=jsonl", nil)
	rec, format := streamCase(t, req, func(s *httpbind.Stream[evt]) error {
		return s.Write(evt{Type: "a", N: 1})
	})
	if format != httpbind.StreamNDJSON {
		t.Fatalf("format %q want ndjson", format)
	}
	if body := rec.Body.String(); strings.HasPrefix(strings.TrimSpace(body), "[") {
		t.Fatalf("jsonl must not be array: %q", body)
	}
}

func TestWriteStream_QueryParamOverridesUA(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chat?stream=sse", nil)
	req.Header.Set("User-Agent", "curl/8.0")
	if _, format := streamCase(t, req, nil); format != httpbind.StreamSSE {
		t.Fatalf("format %q", format)
	}
}

func TestWriteStream_BrowserUA(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chat", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) AppleWebKit/537.36 Chrome/120.0.0.0")
	if _, format := streamCase(t, req, nil); format != httpbind.StreamSSE {
		t.Fatalf("format %q want sse", format)
	}
}

// Closing inside the producer is allowed and the entry's own Close after it is
// a no-op, so a producer that terminates early cannot produce two endings.
func TestWriteStream_WriteAfterClose(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec, _ := streamCase(t, req, func(s *httpbind.Stream[evt]) error {
		if err := s.Close(); err != nil {
			return err
		}
		if err := s.Write(evt{Type: "x"}); err == nil {
			t.Error("expected error after close")
		}
		return nil
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestNegotiateStreamFormat_Exported(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?stream=ndjson", nil)
	req.Header.Set("Accept", "text/event-stream")
	if httpbind.NegotiateStreamFormat(req) != httpbind.StreamNDJSON {
		t.Fatal("query should win over Accept")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Accept", "application/json")
	if httpbind.NegotiateStreamFormat(req2) != httpbind.StreamJSONArray {
		t.Fatal("application/json should select JSON array (not JSONL)")
	}

	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.Header.Set("Accept", "application/jsonl")
	if httpbind.NegotiateStreamFormat(req3) != httpbind.StreamNDJSON {
		t.Fatal("application/jsonl should select NDJSON")
	}
}
