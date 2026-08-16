package pagesfixture

import (
	"net/http"
	"strings"
	"testing"
)

// The archive page writes its heading before its binding. Hoisting evaluates
// the binding first anyway, and the served bytes have to come out in written
// order — which is what makes the reordering unobservable rather than merely
// intended.
//
// Every other fixture puts its binding first, so without this the hoist was
// only ever checked against generated source text, never against a response.
func TestHoistingKeepsTheServedOrder(t *testing.T) {
	rec := get(t, serveMux(), "/archive")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	heading := strings.Index(body, "archive for")
	bound := strings.Index(body, "latest:")
	if heading < 0 || bound < 0 {
		t.Fatalf("body = %q", body)
	}
	if heading > bound {
		t.Fatalf("the hoist moved the markup, not just the computation:\n%s", body)
	}
}
