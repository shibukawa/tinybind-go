package fasthttpbind_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/fasthttpbind"
	"github.com/shibukawa/tinybind-go/jsonbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

type created struct {
	ID string
}

// Both WriteStatus implementations serialize through the shared jsonbind
// encoder registry rather than through their own writer registry, so one
// registration serves both and the two transports have to agree byte for byte.
func init() {
	jsonbind.RegisterEncode[created](func(w io.Writer, v created) error {
		_, err := w.Write([]byte(`{"id":"` + v.ID + `"}` + "\n"))
		return err
	})
}

func TestWriteStatusMatchesNetHTTP(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		value  created
	}{
		{"created carries a body", http.StatusCreated, created{ID: "u1"}},
		{"accepted carries a body", http.StatusAccepted, created{ID: "u2"}},
		{"no content carries none", http.StatusNoContent, created{ID: "u3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if err := httpbind.WriteStatus(rec, req, tc.status, tc.value); err != nil {
				t.Fatalf("net/http WriteStatus: %v", err)
			}

			ctx := &fasthttp.RequestCtx{}
			if err := fasthttpbind.WriteStatus(ctx, tc.status, tc.value); err != nil {
				t.Fatalf("fasthttp WriteStatus: %v", err)
			}

			if got, want := ctx.Response.StatusCode(), rec.Code; got != want {
				t.Errorf("status = %d, want %d", got, want)
			}
			if got, want := string(ctx.Response.Body()), rec.Body.String(); got != want {
				t.Errorf("body = %q, want %q", got, want)
			}
			gotCT := string(ctx.Response.Header.Peek("Content-Type"))
			if want := rec.Header().Get("Content-Type"); gotCT != want {
				t.Errorf("content-type = %q, want %q", gotCT, want)
			}
		})
	}
}

// The encoder registry is the one WriteStatus reads, and it is not the writer
// registry Write reads. Registering a writer alone used to look like enough,
// which is the shape the generated init had.
func TestWriteStatusNeedsTheEncoderNotTheWriter(t *testing.T) {
	type writerOnly struct{ A string }
	fasthttpbind.RegisterWrite[writerOnly](func(ctx *fasthttp.RequestCtx, v writerOnly) error {
		return fasthttpbind.WriteJSONBytes(ctx, http.StatusOK, []byte(`{"a":"`+v.A+`"}`))
	})

	if err := fasthttpbind.Write[writerOnly](&fasthttp.RequestCtx{}, writerOnly{A: "x"}); err != nil {
		t.Fatalf("Write with a registered writer: %v", err)
	}
	err := fasthttpbind.WriteStatus(&fasthttp.RequestCtx{}, http.StatusCreated, writerOnly{A: "x"})
	if err == nil {
		t.Fatal("WriteStatus succeeded with only a writer registered; it reads the encoder registry")
	}
}
