package httpbind_test

import (
	"net/http/httptest"
	"net/url"
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
)

// The flat scanner replaced r.URL.Query(), so what it must stay is agreement:
// for every key, QueryLookup answers exactly what url.ParseQuery's first value
// would have been, across escapes, semicolon pairs, and broken escapes.
func TestQueryLookupMatchesParseQuery(t *testing.T) {
	rawQueries := []string{
		"",
		"name=Alice&email=a%40example.com",
		"a=1&a=2",
		"flag&empty=&x=y",
		"sp=a+b&pct=%20end",
		"bad=%zz&good=1",
		"weird%20key=v",
		"semi=1;drop=2&kept=3",
		"=nokey&also=fine",
		"plus+key=v2",
	}
	keys := []string{"name", "email", "a", "flag", "empty", "x", "sp", "pct", "bad", "good", "weird key", "semi", "kept", "", "also", "plus key", "absent"}
	for _, raw := range rawQueries {
		r := httptest.NewRequest("GET", "/?"+raw, nil)
		q := httpbind.Queries(r)
		want, _ := url.ParseQuery(raw)
		for _, key := range keys {
			wantVals, wantOK := want[key]
			gotVal, gotOK := httpbind.QueryLookup(q, key)
			if gotOK != (wantOK && len(wantVals) > 0) {
				t.Fatalf("raw=%q key=%q: presence got %v want %v", raw, key, gotOK, wantOK)
			}
			if gotOK && gotVal != wantVals[0] {
				t.Fatalf("raw=%q key=%q: got %q want %q", raw, key, gotVal, wantVals[0])
			}
			// QueryValue scans without pre-splitting; it must agree too.
			sv, sok := httpbind.QueryValue(r, key)
			if sok != gotOK || sv != gotVal {
				t.Fatalf("raw=%q key=%q: QueryValue (%q,%v) != QueryLookup (%q,%v)", raw, key, sv, sok, gotVal, gotOK)
			}
		}
	}
}
