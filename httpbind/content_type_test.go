package httpbind_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shibukawa/tinybind-go/httpbind"
)

func TestIsJSONRequest_JSONAndPlusJSON(t *testing.T) {
	cases := []struct {
		ct   string
		want bool
	}{
		{"", false},
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"APPLICATION/JSON", true},
		{"text/json", true},
		{"application/problem+json", true},
		{"application/problem+json; charset=utf-8", true},
		{"application/vnd.api+json", true},
		{"application/merge-patch+json", true},
		{"application/json-patch+json", true},
		// not JSON
		{"application/jsonl", false},
		{"application/x-ndjson", false},
		{"application/json-seq", false},
		{"application/x-www-form-urlencoded", false},
		{"multipart/form-data; boundary=x", false},
		{"text/plain", false},
		{"application/problem+xml", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if tc.ct != "" {
			req.Header.Set("Content-Type", tc.ct)
		}
		got := httpbind.IsJSONRequest(req)
		if got != tc.want {
			t.Errorf("Content-Type %q: got %v want %v", tc.ct, got, tc.want)
		}
	}
}
