package httpbinder_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpbinder "github.com/shibukawa/httpbind-go"
)

type evt struct {
	Type string `json:"type"`
	N    int    `json:"n"`
}

func TestNewStream_MultipleWrites_NDJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	req.Header.Set("User-Agent", "curl/8.4.0")

	s, err := httpbinder.NewStream[evt](rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if s.Format() != httpbinder.StreamNDJSON {
		t.Fatalf("format %q", s.Format())
	}
	if err := s.Write(evt{Type: "a", N: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(evt{Type: "b", N: 2}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

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

func TestNewStream_AcceptSSE(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	req.Header.Set("Accept", "text/html, text/event-stream, application/json")

	s, err := httpbinder.NewStream[evt](rec, req)
	if err != nil {
		t.Fatal(err)
	}
	// first matching stream media type: text/html ignored, then event-stream
	if s.Format() != httpbinder.StreamSSE {
		t.Fatalf("format %q", s.Format())
	}
	_ = s.Write(evt{Type: "x", N: 1})
	_ = s.Write(evt{Type: "y", N: 2})
	_ = s.Close()
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("ctype %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if strings.Count(body, "data: ") != 2 {
		t.Fatalf("sse events: %s", body)
	}
}

func TestNewStream_JSONArray_AcceptJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	req.Header.Set("Accept", "application/json")

	s, err := httpbinder.NewStream[evt](rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if s.Format() != httpbinder.StreamJSONArray {
		t.Fatalf("format %q", s.Format())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("ctype %q", rec.Header().Get("Content-Type"))
	}
	_ = s.Write(evt{Type: "a", N: 1})
	_ = s.Write(evt{Type: "b", N: 2})
	_ = s.Close()

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

func TestNewStream_JSONArray_EmptyClose(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/chat?stream=json", nil)
	s, err := httpbinder.NewStream[evt](rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if s.Format() != httpbinder.StreamJSONArray {
		t.Fatalf("format %q", s.Format())
	}
	_ = s.Close()
	if rec.Body.String() != "[]" {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestNewStream_JSONArray_QueryOverridesUA(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/chat?stream=array", nil)
	req.Header.Set("User-Agent", "curl/8.0")
	s, err := httpbinder.NewStream[evt](rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if s.Format() != httpbinder.StreamJSONArray {
		t.Fatalf("format %q", s.Format())
	}
}

func TestNewStream_JSONL_NotArray(t *testing.T) {
	// JSONL/NDJSON must stay line-delimited, not a JSON array.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/chat?stream=jsonl", nil)
	s, err := httpbinder.NewStream[evt](rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if s.Format() != httpbinder.StreamNDJSON {
		t.Fatalf("format %q want ndjson", s.Format())
	}
	_ = s.Write(evt{Type: "a", N: 1})
	_ = s.Close()
	body := rec.Body.String()
	if strings.HasPrefix(strings.TrimSpace(body), "[") {
		t.Fatalf("jsonl must not be array: %q", body)
	}
}

func TestNewStream_QueryParamOverridesUA(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/chat?stream=sse", nil)
	req.Header.Set("User-Agent", "curl/8.0")

	s, err := httpbinder.NewStream[evt](rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if s.Format() != httpbinder.StreamSSE {
		t.Fatalf("format %q", s.Format())
	}
}

func TestNewStream_BrowserUA(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/chat", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) AppleWebKit/537.36 Chrome/120.0.0.0")

	s, err := httpbinder.NewStream[evt](rec, req)
	if err != nil {
		t.Fatal(err)
	}
	if s.Format() != httpbinder.StreamSSE {
		t.Fatalf("format %q want sse", s.Format())
	}
}

func TestNewStream_WriteAfterClose(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	s, err := httpbinder.NewStream[evt](rec, req)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if err := s.Write(evt{Type: "x"}); err == nil {
		t.Fatal("expected error after close")
	}
}

func TestNegotiateStreamFormat_Exported(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?stream=ndjson", nil)
	req.Header.Set("Accept", "text/event-stream")
	if httpbinder.NegotiateStreamFormat(req) != httpbinder.StreamNDJSON {
		t.Fatal("query should win over Accept")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Accept", "application/json")
	if httpbinder.NegotiateStreamFormat(req2) != httpbinder.StreamJSONArray {
		t.Fatal("application/json should select JSON array (not JSONL)")
	}

	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.Header.Set("Accept", "application/jsonl")
	if httpbinder.NegotiateStreamFormat(req3) != httpbinder.StreamNDJSON {
		t.Fatal("application/jsonl should select NDJSON")
	}
}
