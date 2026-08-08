package httpbind

import (
	"net/http"
	"net/url"
)

// Queries parses the request's query string once. Generated binders call this
// a single time per request and resolve each field with QueryLookup, instead
// of re-parsing the raw query per field the way QueryValue does.
func Queries(r *http.Request) url.Values {
	if r == nil || r.URL == nil || r.URL.RawQuery == "" {
		return nil
	}
	return r.URL.Query()
}

// QueryLookup returns the first value for key from pre-parsed query values.
func QueryLookup(q url.Values, key string) (string, bool) {
	vs := q[key]
	if len(vs) == 0 {
		return "", false
	}
	return vs[0], true
}
