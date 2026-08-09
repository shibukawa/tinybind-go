package fasthttpbind_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/fasthttpbind"
	"github.com/shibukawa/tinybind-go/jsonbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

type ev struct {
	Type  string
	Delta string
}

func init() {
	jsonbind.RegisterEncode[ev](func(w io.Writer, v ev) error {
		_, err := w.Write([]byte(`{"type":"` + v.Type + `","delta":"` + v.Delta + `"}` + "\n"))
		return err
	})
}

// streamCase is the same callback body on both transports. Keeping it one
// function is the point of the callback shape: a rewrite changes the entry
// call, never this.
func streamCase(s *fasthttpbind.Stream[ev]) error {
	if err := s.Write(ev{Type: "delta", Delta: "hi"}); err != nil {
		return err
	}
	return s.Write(ev{Type: "done"})
}

func netHTTPStream(t *testing.T, accept string, fn func(*httpbind.Stream[ev]) error) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	if accept != "" {
		r.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	httpbind.WriteStream(rec, r, fn)
	return rec
}

func fastStream(t *testing.T, accept string, fn func(*fasthttpbind.Stream[ev]) error) (*fasthttp.RequestCtx, []byte) {
	t.Helper()
	var fr fasthttp.Request
	fr.SetRequestURI("/events")
	fr.Header.SetMethod(http.MethodGet)
	if accept != "" {
		fr.Header.Set("Accept", accept)
	}
	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&fr, nil, nil)

	fasthttpbind.WriteStream(ctx, fn)

	// The body writer runs after the handler returns, so draining the response
	// is what actually executes fn.
	var buf bytes.Buffer
	if err := ctx.Response.BodyWriteTo(&buf); err != nil {
		t.Fatalf("drain body: %v", err)
	}
	return ctx, buf.Bytes()
}

func TestWriteStreamParity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		accept string
	}{
		{"ndjson_default", ""},
		{"sse", "text/event-stream"},
		{"json_array", "application/json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := netHTTPStream(t, tc.accept, streamCase)
			ctx, body := fastStream(t, tc.accept, streamCase)

			if got, want := ctx.Response.StatusCode(), rec.Code; got != want {
				t.Errorf("status: fasthttp %d, net/http %d", got, want)
			}
			if got, want := string(ctx.Response.Header.ContentType()), rec.Header().Get("Content-Type"); got != want {
				t.Errorf("content-type: fasthttp %q, net/http %q", got, want)
			}
			if got, want := body, rec.Body.Bytes(); !bytes.Equal(got, want) {
				t.Errorf("body differs\n net/http: %q\n fasthttp: %q", want, got)
			}
		})
	}
}

// The defect the callback shape exists to remove: a JSON array document that
// ends without its bracket because the caller forgot to close the stream.
func TestWriteStreamAlwaysTerminatesJSONArray(t *testing.T) {
	failing := func(s *fasthttpbind.Stream[ev]) error {
		if err := s.Write(ev{Type: "delta", Delta: "hi"}); err != nil {
			return err
		}
		return errors.New("gave up halfway")
	}
	_, body := fastStream(t, "application/json", failing)
	if !bytes.HasSuffix(body, []byte("]")) {
		t.Errorf("array was left unterminated: %q", body)
	}
	if !json_valid(body) {
		t.Errorf("array is not valid JSON: %q", body)
	}
}

func TestWriteStreamEmptyArrayIsStillADocument(t *testing.T) {
	_, body := fastStream(t, "application/json", func(s *fasthttpbind.Stream[ev]) error { return nil })
	if string(body) != "[]" {
		t.Errorf("empty stream body = %q, want %q", body, "[]")
	}
}

// A post-commit failure has no status left to carry it, so the hook is the
// only place it can go. Verify it actually arrives, on both transports.
func TestStreamErrorReachesTheHandler(t *testing.T) {
	want := errors.New("boom")
	var got []error
	fasthttpbind.SetStreamErrorHandler(func(err error) { got = append(got, err) })
	defer fasthttpbind.SetStreamErrorHandler(nil)

	failing := func(s *fasthttpbind.Stream[ev]) error {
		_ = s.Write(ev{Type: "delta"})
		return want
	}
	fastStream(t, "", failing)
	netHTTPStream(t, "", func(s *httpbind.Stream[ev]) error {
		_ = s.Write(ev{Type: "delta"})
		return want
	})

	if len(got) != 2 {
		t.Fatalf("handler saw %d errors, want 2 (one per transport): %v", len(got), got)
	}
	for i, err := range got {
		if !errors.Is(err, want) {
			t.Errorf("error %d = %v, want %v", i, err, want)
		}
	}
}

func TestNegotiateStreamFormatParity(t *testing.T) {
	for _, accept := range []string{"", "text/event-stream", "application/json", "application/x-ndjson", "text/html"} {
		r := httptest.NewRequest(http.MethodGet, "/events", nil)
		var fr fasthttp.Request
		fr.SetRequestURI("/events")
		if accept != "" {
			r.Header.Set("Accept", accept)
			fr.Header.Set("Accept", accept)
		}
		ctx := &fasthttp.RequestCtx{}
		ctx.Init(&fr, nil, nil)

		if got, want := fasthttpbind.NegotiateStreamFormat(ctx), httpbind.NegotiateStreamFormat(r); got != want {
			t.Errorf("Accept %q: fasthttp %q, net/http %q", accept, got, want)
		}
	}
}

func json_valid(b []byte) bool {
	_, err := jsonbind.RawJSONArray(b)
	return err == nil
}
