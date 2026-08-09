package fasthttpupdate_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/fasthttpupdate"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The stream half compared the same way as the read-only half: one description
// of a request, both transports, the bytes on the wire held against each other.
//
// The callback body is written once and passed to both entries. That is the
// property the shape exists for — if the two shells needed different producer
// code, a rewritten handler would differ by more than its signature line.

func navigation(target string) headerPair {
	return headerPair{method: http.MethodGet, target: target, headers: map[string]string{
		"X-Tinybind-Render": "navigation",
		"X-Tinybind-Build":  "b1",
	}}
}

func live(target string) headerPair {
	return headerPair{method: http.MethodGet, target: target, headers: map[string]string{
		"X-Tinybind-Render": "live",
		"X-Tinybind-Build":  "b1",
	}}
}

// fastBody serializes what the handler left on the response, including a body
// installed with SetBodyStreamWriter, which is only produced when it is read.
func fastBody(t *testing.T, ctx *fasthttp.RequestCtx) string {
	t.Helper()
	var buf bytes.Buffer
	if err := ctx.Response.BodyWriteTo(&buf); err != nil {
		t.Fatalf("read the streamed body: %v", err)
	}
	return buf.String()
}

func sameRecords(t *testing.T, what, want, got string) {
	t.Helper()
	if want == "" {
		t.Fatalf("%s: net/http wrote nothing to compare against", what)
	}
	if want != got {
		t.Errorf("%s differs\n net/http: %q\n fasthttp: %q", what, want, got)
	}
}

// produce is the producer body, written once and driven by both entries.
func produce(stream *htmlupdate.DeltaStream) error {
	stream.Replace("c1", `<main id="c1">first</main>`, htmlupdate.ManifestEntry{Frame: "f1"})
	stream.Unchanged("c2", htmlupdate.ManifestEntry{Frame: "f2"})
	return nil
}

func TestWriteStreamParity(t *testing.T) {
	req := navigation("/feed")
	head := []string{"<title>Feed</title>"}

	recorder := httptest.NewRecorder()
	netOptions.WriteStream(recorder, req.netHTTP(), head, produce)

	ctx := req.fast()
	fastOptions.WriteStream(ctx, head, produce)

	sameRecords(t, "write stream", recorder.Body.String(), fastBody(t, ctx))
	if !strings.Contains(recorder.Body.String(), `"r":"end"`) {
		t.Error("net/http stream carried no terminator")
	}
}

// The entry closes the stream whether or not the producer succeeded, which is
// the defect the held shape allowed: a producer that returned early left a
// client holding a response it had to treat as truncated.
func TestWriteStreamClosesAfterAFailedProducerParity(t *testing.T) {
	req := navigation("/feed")
	failing := func(stream *htmlupdate.DeltaStream) error {
		stream.Replace("c1", `<main id="c1">partial</main>`, htmlupdate.ManifestEntry{Frame: "f1"})
		return errors.New("boundary failed")
	}

	var netErr, fastErr error
	htmlupdate.SetStreamErrorHandler(func(err error) { netErr = err })
	recorder := httptest.NewRecorder()
	netOptions.WriteStream(recorder, req.netHTTP(), nil, failing)

	fasthttpupdate.SetStreamErrorHandler(func(err error) { fastErr = err })
	ctx := req.fast()
	fastOptions.WriteStream(ctx, nil, failing)
	body := fastBody(t, ctx)
	htmlupdate.SetStreamErrorHandler(nil)

	sameRecords(t, "failed producer", recorder.Body.String(), body)
	if !strings.Contains(body, `"reason":"failed"`) {
		t.Errorf("the failure did not reach the terminator: %q", body)
	}
	if netErr == nil || fastErr == nil {
		t.Errorf("the post-commit failure was not reported: net/http %v, fasthttp %v", netErr, fastErr)
	}
}

// A live stream's terminator has to say "come back" rather than "stop", or a
// server rotating a connection freezes every open screen until somebody
// reloads.
func TestWriteLiveStreamParity(t *testing.T) {
	req := live("/feed")
	rollover := func(stream *htmlupdate.DeltaStream) error {
		stream.Replace("c1", `<main id="c1">now</main>`, htmlupdate.ManifestEntry{})
		return stream.Retry(0)
	}

	recorder := httptest.NewRecorder()
	netOptions.WriteLiveStream(recorder, req.netHTTP(), nil, rollover)

	ctx := req.fast()
	fastOptions.WriteLiveStream(ctx, nil, rollover)
	body := fastBody(t, ctx)

	sameRecords(t, "live stream", recorder.Body.String(), body)
	if !strings.Contains(body, `"reason":"retry"`) {
		t.Errorf("a healthy rollover did not close retry: %q", body)
	}
	if strings.Count(body, `"r":"end"`) != 1 {
		t.Errorf("terminator written more than once: %q", body)
	}
}

func TestRenderStreamParity(t *testing.T) {
	req := navigation("/feed")
	fragment := htmlbind.Bind(badgePlan, badgeParams{ID: "cart", Count: 4})

	recorder := httptest.NewRecorder()
	if err := netOptions.RenderStream(recorder, req.netHTTP(), nil, fragment); err != nil {
		t.Fatal(err)
	}
	ctx := req.fast()
	if err := fastOptions.RenderStream(ctx, nil, fragment); err != nil {
		t.Fatal(err)
	}
	sameRecords(t, "render stream", recorder.Body.String(), fastBody(t, ctx))
}

func TestRenderStreamAsyncParity(t *testing.T) {
	req := navigation("/feed")
	fragment := htmlbind.Bind(badgePlan, badgeParams{ID: "cart", Count: 4})

	recorder := httptest.NewRecorder()
	if err := netOptions.RenderStreamAsync(context.Background(), recorder, req.netHTTP(), nil, fragment); err != nil {
		t.Fatal(err)
	}
	ctx := req.fast()
	if err := fastOptions.RenderStreamAsync(context.Background(), ctx, nil, fragment); err != nil {
		t.Fatal(err)
	}
	sameRecords(t, "async render stream", recorder.Body.String(), fastBody(t, ctx))
}

// Render buffers, so it needs no body stream writer on either transport. This
// is the entry the downstream report grouped with the streaming half; it never
// belonged there.
func TestRenderParity(t *testing.T) {
	fragment := htmlbind.Bind(badgePlan, badgeParams{ID: "cart", Count: 4})

	for _, tc := range []struct {
		name string
		req  headerPair
	}{
		{"document", headerPair{method: http.MethodGet, target: "/feed"}},
		{"navigation delta", navigation("/feed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.req
			recorder := httptest.NewRecorder()
			if err := netOptions.Render(recorder, req.netHTTP(), nil, fragment); err != nil {
				t.Fatal(err)
			}
			ctx := req.fast()
			if err := fastOptions.Render(ctx, nil, fragment); err != nil {
				t.Fatal(err)
			}
			sameRecords(t, "render", recorder.Body.String(), fastBody(t, ctx))
		})
	}
}

// A RequestCtx handed in as the cancellation context is pooled and reused, and
// the delivery it would guard runs after the handler returned. Substituting it
// is what keeps a rewritten handler — which reaches here with the same
// identifier in both positions — from reading recycled memory.
func TestLiveStreamDetachesTheRequestCtx(t *testing.T) {
	req := live("/feed")
	ctx := req.fast()
	fragment := htmlbind.Bind(badgePlan, badgeParams{ID: "cart", Count: 4})

	// Passing the RequestCtx as its own cancellation is exactly what the
	// transform produces, so it has to be survivable rather than merely
	// documented.
	if err := fastOptions.RenderLiveStream(ctx, ctx, nil, fragment); err != nil {
		t.Fatal(err)
	}
	if body := fastBody(t, ctx); !strings.Contains(body, `"r":"end"`) {
		t.Errorf("the live stream did not terminate: %q", body)
	}
}
