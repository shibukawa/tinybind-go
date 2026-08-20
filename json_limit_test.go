package httpbind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadJSONBodyRejectsOversize(t *testing.T) {
	old := MaxJSONBodyBytes()
	SetMaxJSONBodyBytes(16)
	defer SetMaxJSONBodyBytes(old)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"value":"this is too large"}`))
	_, err := ReadJSONBody(r)
	he, ok := AsHTTPError(err)
	if !ok || he.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %#v", err)
	}
}
