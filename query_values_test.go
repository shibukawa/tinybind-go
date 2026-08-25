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

// QueryLookupAll is the array spelling, so it must agree with url.ParseQuery
// the way QueryLookup does: same keys, same order, minus the empty values a
// blank control contributes and an element must not become.
func TestQueryLookupAllMatchesParseQuery(t *testing.T) {
	rawQueries := []string{
		"",
		"tag=a&tag=b",
		"tag=b&tag=a&tag=b",
		"q=go&tag=a&other=1&tag=b",
		"tag=&tag=a",
		"tag&tag=a",
		"tag=",
		"tag=a%2Cb",
		"tag=a+b&tag=%20c",
		"tag=%zz&tag=ok",
		"semi=1;tag=2&tag=3",
	}
	keys := []string{"tag", "q", "other", "semi", "absent"}
	for _, raw := range rawQueries {
		r := httptest.NewRequest("GET", "/?"+raw, nil)
		q := httpbind.Queries(r)
		parsed, _ := url.ParseQuery(raw)
		for _, key := range keys {
			var want []string
			for _, v := range parsed[key] {
				if v != "" {
					want = append(want, v)
				}
			}
			got := httpbind.QueryLookupAll(q, key)
			if len(got) != len(want) {
				t.Fatalf("raw=%q key=%q: got %q want %q", raw, key, got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("raw=%q key=%q: element %d got %q want %q", raw, key, i, got[i], want[i])
				}
			}
		}
	}
}

// A comma is an ordinary value character. tag=a%2Cb and tag=a,b are the same
// single element, which is why the joined spelling is not an array here.
func TestQueryLookupAllDoesNotSplitOnComma(t *testing.T) {
	for _, raw := range []string{"tag=a%2Cb", "tag=a,b"} {
		r := httptest.NewRequest("GET", "/?"+raw, nil)
		got := httpbind.QueryLookupAll(httpbind.Queries(r), "tag")
		if len(got) != 1 || got[0] != "a,b" {
			t.Errorf("raw=%q: got %q, want one element %q", raw, got, "a,b")
		}
	}
}

// Brackets are ordinary key characters, so the PHP spelling binds the key it
// literally names and nothing reaches the key an author declared.
func TestQueryLookupAllTreatsBracketsAsPartOfTheKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/?tag[]=a&tag[]=b", nil)
	q := httpbind.Queries(r)
	if got := httpbind.QueryLookupAll(q, "tag"); len(got) != 0 {
		t.Errorf(`QueryLookupAll(q, "tag") = %q, want none for a bracket-spelled query`, got)
	}
	if got := httpbind.QueryLookupAll(q, "tag[]"); len(got) != 2 {
		t.Errorf(`QueryLookupAll(q, "tag[]") = %q, want both values`, got)
	}
}
