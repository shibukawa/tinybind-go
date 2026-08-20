package httpbind

import (
	"net/http"
	"net/url"
	"strings"
)

// QueryValues is the request's query string split once into raw key=value
// spans, in wire order.
//
// Generated binders only ever ask for a key's first value, so the url.Values
// map — one allocation per key plus an unescaped copy of every member — buys
// random access nobody uses. Splitting into spans costs one slice, and a span
// is unescaped only when it is actually looked up and actually escaped: the
// same trade jsonbind.Object makes for JSON bodies.
type QueryValues struct {
	pairs []queryPair
}

// queryPair holds one undecoded key=value span of the raw query.
type queryPair struct{ rawKey, rawValue string }

// Queries parses the request's query string once. Generated binders call this
// a single time per request and resolve each field with QueryLookup, instead
// of re-parsing the raw query per field the way QueryValue does.
func Queries(r *http.Request) QueryValues {
	if r == nil || r.URL == nil || r.URL.RawQuery == "" {
		return QueryValues{}
	}
	raw := r.URL.RawQuery
	n := 1
	for i := 0; i < len(raw); i++ {
		if raw[i] == '&' {
			n++
		}
	}
	pairs := make([]queryPair, 0, n)
	for len(raw) > 0 {
		var seg string
		if i := strings.IndexByte(raw, '&'); i >= 0 {
			seg, raw = raw[:i], raw[i+1:]
		} else {
			seg, raw = raw, ""
		}
		// url.ParseQuery drops empty segments and rejects semicolon-bearing
		// pairs; matching that keeps this view and r.URL.Query() agreeing on
		// which pairs exist.
		if seg == "" || strings.IndexByte(seg, ';') >= 0 {
			continue
		}
		k, v, _ := strings.Cut(seg, "=")
		pairs = append(pairs, queryPair{rawKey: k, rawValue: v})
	}
	return QueryValues{pairs: pairs}
}

// QueryLookup returns the first value for key from pre-parsed query values.
func QueryLookup(q QueryValues, key string) (string, bool) {
	for i := range q.pairs {
		if v, ok, matched := matchQueryPair(q.pairs[i].rawKey, q.pairs[i].rawValue, key); matched {
			return v, ok
		}
	}
	return "", false
}

// matchQueryPair decides one raw pair against a wanted key. matched reports
// the pair answered the lookup; a pair whose escapes do not decode is treated
// the way url.ParseQuery treats it — as if it were not there.
func matchQueryPair(rawKey, rawValue, key string) (value string, ok bool, matched bool) {
	if queryNeedsUnescape(rawKey) {
		k, err := url.QueryUnescape(rawKey)
		if err != nil || k != key {
			return "", false, false
		}
	} else if rawKey != key {
		return "", false, false
	}
	if !queryNeedsUnescape(rawValue) {
		return rawValue, true, true
	}
	v, err := url.QueryUnescape(rawValue)
	if err != nil {
		return "", false, false
	}
	return v, true, true
}

// queryNeedsUnescape reports whether a raw span decodes to something other
// than itself. The common query — unescaped ASCII keys and values — skips the
// decode and its allocation entirely.
func queryNeedsUnescape(s string) bool {
	return strings.IndexByte(s, '%') >= 0 || strings.IndexByte(s, '+') >= 0
}

// queryScan resolves one key straight off the raw query, for the callers that
// want a single value and would waste a full split on it.
func queryScan(rawQuery, key string) (string, bool) {
	raw := rawQuery
	for len(raw) > 0 {
		var seg string
		if i := strings.IndexByte(raw, '&'); i >= 0 {
			seg, raw = raw[:i], raw[i+1:]
		} else {
			seg, raw = raw, ""
		}
		if seg == "" || strings.IndexByte(seg, ';') >= 0 {
			continue
		}
		k, v, _ := strings.Cut(seg, "=")
		if value, ok, matched := matchQueryPair(k, v, key); matched {
			return value, ok
		}
	}
	return "", false
}
