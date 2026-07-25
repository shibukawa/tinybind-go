package pages

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const rendered = "\n<!DOCTYPE html>\n<h1>Hello &amp; welcome</h1>\n"

func TestRenderedOutput(t *testing.T) {
	var output strings.Builder
	if err := Hello(&output, HelloParams{}); err != nil {
		t.Fatal(err)
	}
	if output.String() != rendered {
		t.Fatalf("output = %q, want %q", output.String(), rendered)
	}
}

// TestRenderedToResponseWriter shows that the caller owns every HTTP concern:
// the component only writes bytes.
func TestRenderedToResponseWriter(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := Hello(recorder, HelloParams{}); err != nil {
		t.Fatal(err)
	}
	if recorder.Body.String() != rendered {
		t.Fatalf("output = %q, want %q", recorder.Body.String(), rendered)
	}
	if got := recorder.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d", got)
	}
}
