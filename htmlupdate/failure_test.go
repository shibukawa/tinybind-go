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

// Every way a redraw can be refused reaches the caller, because a failure that
// only becomes a status code is invisible to a logger and a tracer.
func TestEveryRedrawFailureReachesTheCaller(t *testing.T) {
	cases := []struct {
		name    string
		request *http.Request
		kind    htmlupdate.FailureKind
		status  int
		hasErr  bool
		named   bool
		message string
	}{
		{
			name:    "names no component",
			request: redrawRequest("", "", nil),
			kind:    htmlupdate.FailureMalformedRequest,
			status:  http.StatusBadRequest,
		},
		{
			name:    "unknown component",
			request: redrawRequest("Gone@0000", "card-1", url.Values{"page": {"1"}}),
			kind:    htmlupdate.FailureUnknownComponent,
			status:  http.StatusNotFound,
			named:   true,
		},
		{
			name: "arguments too large",
			request: redrawRequest(cardKind, "card-1", url.Values{
				"page": {"1"},
				"pad":  {strings.Repeat("x", htmlupdate.DefaultMaxQueryBytes)},
			}),
			kind:   htmlupdate.FailureArgumentsTooLarge,
			status: http.StatusRequestURITooLong,
			named:  true,
		},
		{
			name:    "invalid arguments",
			request: redrawRequest(cardKind, "card-1", url.Values{"page": {"not a number"}}),
			kind:    htmlupdate.FailureInvalidArguments,
			status:  http.StatusBadRequest,
			hasErr:  true,
			named:   true,
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
			recorder := httptest.NewRecorder()
			redrawServerWith(t, opts).ServeHTTP(recorder, testCase.request)

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
	recorder := httptest.NewRecorder()
	opts.Redraw(recorder, redrawRequest("Broken@0001", "b-1", nil), registry)

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

// A caller supplying no hook gets this module's own error format. The update
// endpoints were the only paths writing plain text, which left one project with
// two error shapes depending on which entry refused the request.
func TestFailureDefaultIsProblemDetails(t *testing.T) {
	recorder := httptest.NewRecorder()
	redrawServer(t).ServeHTTP(recorder, redrawRequest(cardKind, "card-1", url.Values{"page": {"nope"}}))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	// The media type is the discriminator: application/json is an update to
	// apply, including a non-2xx one, and this is a request that produced none.
	if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("content type = %q", got)
	}
	var body struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %s", recorder.Body.String())
	}
	if body.Status != http.StatusBadRequest || body.Type != "about:blank" {
		t.Fatalf("body = %+v", body)
	}
	// The kind travels as the code, because a stale page and a failed render are
	// one status to a proxy and different events to whoever is on call.
	if body.Code != "invalid_arguments" {
		t.Fatalf("code = %q", body.Code)
	}
	if body.Detail != "invalid redraw arguments" {
		t.Fatalf("detail = %q", body.Detail)
	}
}

// A parameter the decoder refused is reported as a field-level error naming it,
// rather than as one line of prose a caller would have to parse. The reason
// never quotes the value, which is attacker-supplied.
func TestFailureNamesTheRefusedParameter(t *testing.T) {
	registry := &htmlupdate.Registry{}
	if err := registry.Register(htmlupdate.Reloadable{
		KindID: "Typed@0001",
		Render: func(_ *http.Request, id string, values url.Values) (htmlbind.Fragment, error) {
			var page int
			if err := htmlupdate.QueryInt(values, "page", &page); err != nil {
				return htmlbind.Fragment{}, err
			}
			return htmlbind.Bind(badgePlan, badgeParams{ID: id, Count: page}), nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	options.Redraw(recorder, redrawRequest("Typed@0001", "t-1", url.Values{"page": {"nope"}}), registry)

	var body struct {
		Errors []struct {
			Field    string `json:"field"`
			Location string `json:"location"`
			Message  string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %s", recorder.Body.String())
	}
	if len(body.Errors) != 1 {
		t.Fatalf("errors = %+v, body = %s", body.Errors, recorder.Body.String())
	}
	got := body.Errors[0]
	if got.Field != "page" || got.Location != "query" {
		t.Fatalf("errors[0] = %+v", got)
	}
	if strings.Contains(recorder.Body.String(), "nope") {
		t.Fatalf("the refused value was reflected back: %s", recorder.Body.String())
	}
}

// The cause never reaches the response, whatever it says. It travels to the
// caller on the Failure value instead, which is what the hook exists for.
func TestFailureBodyOmitsTheCause(t *testing.T) {
	registry := &htmlupdate.Registry{}
	if err := registry.Register(htmlupdate.Reloadable{
		KindID: "Leaky@0001",
		Render: func(*http.Request, string, url.Values) (htmlbind.Fragment, error) {
			return htmlbind.Fragment{}, errors.New("dial tcp 10.0.0.5:5432: connection refused")
		},
	}); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	options.Redraw(recorder, redrawRequest("Leaky@0001", "l-1", nil), registry)
	if strings.Contains(recorder.Body.String(), "10.0.0.5") {
		t.Fatalf("the cause reached the response: %s", recorder.Body.String())
	}
}

// A deployment behind a proxy with its own URL limit lowers the bound without
// forking, which is the same shape MaxManifestBytes already had.
func TestRedrawQueryBoundIsConfigurable(t *testing.T) {
	opts := options
	opts.MaxQueryBytes = 32
	request := redrawRequest(cardKind, "card-1", url.Values{"page": {"1"}, "pad": {strings.Repeat("x", 64)}})
	recorder := httptest.NewRecorder()
	redrawServerWith(t, opts).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestURITooLong {
		t.Fatalf("status = %d, want the configured bound to apply", recorder.Code)
	}
}
