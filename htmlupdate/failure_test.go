package htmlupdate_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// redrawServerWith mounts the redraw endpoint under caller-supplied options, so
// a test can watch what the endpoint reports rather than only what it writes.
func redrawServerWith(t *testing.T, opts htmlupdate.Options) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	opts.Mount(mux, cardRegistry(t))
	return mux
}

// Every way a redraw can be refused reaches the caller, because a failure that
// only becomes a status code is invisible to a logger and a tracer.
func TestEveryRedrawFailureReachesTheCaller(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		stale   bool
		kind    htmlupdate.FailureKind
		status  int
		hasErr  bool
		named   bool
		message string
	}{
		{
			name:   "malformed path",
			path:   htmlupdate.DefaultPathPrefix + "/redraw/" + cardKind,
			kind:   htmlupdate.FailureMalformedPath,
			status: http.StatusNotFound,
		},
		{
			name:   "unknown component",
			path:   options.RedrawPath("Gone@0000", "card-1", url.Values{"page": {"1"}}),
			kind:   htmlupdate.FailureUnknownComponent,
			status: http.StatusNotFound,
			named:  true,
		},
		{
			name:   "stale page",
			path:   options.RedrawPath(cardKind, "card-1", url.Values{"page": {"1"}}),
			stale:  true,
			kind:   htmlupdate.FailureStalePage,
			status: http.StatusConflict,
			named:  true,
		},
		{
			name: "arguments too large",
			path: options.RedrawPath(cardKind, "card-1", url.Values{"page": {"1"}}) +
				"&pad=" + strings.Repeat("x", htmlupdate.DefaultMaxQueryBytes),
			kind:   htmlupdate.FailureArgumentsTooLarge,
			status: http.StatusRequestURITooLong,
			named:  true,
		},
		{
			name:   "invalid arguments",
			path:   options.RedrawPath(cardKind, "card-1", url.Values{"page": {"not a number"}}),
			kind:   htmlupdate.FailureInvalidArguments,
			status: http.StatusBadRequest,
			hasErr: true,
			named:  true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var seen []htmlupdate.Failure
			opts := options
			opts.OnFailure = func(w http.ResponseWriter, r *http.Request, failure htmlupdate.Failure) {
				seen = append(seen, failure)
				// A caller answering in its own format, which is the whole
				// point of the hook.
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(failure.Status)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"title":  failure.Kind.String(),
					"status": failure.Status,
				})
			}
			request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
			if !testCase.stale {
				request.Header.Set("X-Tinybind-Build", htmlupdate.BuildID())
			}
			recorder := httptest.NewRecorder()
			redrawServerWith(t, opts).ServeHTTP(recorder, request)

			if len(seen) != 1 {
				t.Fatalf("hook called %d times, want 1", len(seen))
			}
			failure := seen[0]
			if failure.Kind != testCase.kind {
				t.Fatalf("kind = %v, want %v", failure.Kind, testCase.kind)
			}
			if failure.Status != testCase.status {
				t.Fatalf("status = %d, want %d", failure.Status, testCase.status)
			}
			if testCase.hasErr && failure.Err == nil {
				t.Fatal("cause is nil, so the caller cannot log why the decoder refused")
			}
			if !testCase.hasErr && failure.Err != nil {
				t.Fatalf("cause = %v, want none", failure.Err)
			}
			if testCase.named && failure.KindID == "" {
				t.Fatal("KindID is empty, so a log line cannot say what was asked for")
			}
			// The caller's response is what the client gets, not this
			// package's.
			if recorder.Code != testCase.status {
				t.Fatalf("response status = %d, want %d", recorder.Code, testCase.status)
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Fatalf("content type = %q, want the caller's", got)
			}
		})
	}
}

// A render that fails after its arguments decoded is a server fault rather than
// a bad request, and it carries its cause.
func TestRedrawRenderFailureReachesTheCaller(t *testing.T) {
	var seen htmlupdate.Failure
	opts := options
	opts.OnFailure = func(w http.ResponseWriter, r *http.Request, failure htmlupdate.Failure) {
		seen = failure
		htmlupdate.WriteFailure(w, failure)
	}
	registry := &htmlupdate.Registry{}
	broken := errors.New("upstream unavailable")
	if err := registry.Register(htmlupdate.Reloadable{
		KindID: "Broken@0001",
		Render: func(*http.Request, string, url.Values) (htmlbind.Fragment, error) {
			return htmlbind.Fragment{}, broken
		},
	}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	opts.Mount(mux, registry)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, buildRequest(opts.RedrawPath("Broken@0001", "b-1", nil)))

	if seen.Kind != htmlupdate.FailureInvalidArguments {
		t.Fatalf("kind = %v", seen.Kind)
	}
	if !errors.Is(seen, broken) {
		t.Fatalf("cause = %v, want %v reachable through errors.Is", seen.Err, broken)
	}
	// Delegating to WriteFailure keeps the default body, which is how a caller
	// that only wants to observe stays out of the response's way.
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

// A caller supplying no hook sees exactly the bytes this package wrote before
// the hook existed.
func TestFailureDefaultsAreUnchanged(t *testing.T) {
	path := options.RedrawPath(cardKind, "card-1", url.Values{"page": {"nope"}})
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, buildRequest(path))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Body.String(); got != "invalid redraw arguments\n" {
		t.Fatalf("body = %q", got)
	}
}

// A deployment behind a proxy with its own URL limit lowers the bound without
// forking, which is the same shape MaxManifestBytes already had.
func TestRedrawQueryBoundIsConfigurable(t *testing.T) {
	opts := options
	opts.MaxQueryBytes = 32
	path := options.RedrawPath(cardKind, "card-1", url.Values{"page": {"1"}}) +
		"&pad=" + strings.Repeat("x", 64)
	recorder := httptest.NewRecorder()
	redrawServerWith(t, opts).ServeHTTP(recorder, buildRequest(path))
	if recorder.Code != http.StatusRequestURITooLong {
		t.Fatalf("status = %d, want the configured bound to apply", recorder.Code)
	}
}
