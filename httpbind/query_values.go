package httpbind

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
)

// QueryValues is the request's query string split once into raw key=value
// spans, in wire order.
//
// It is an alias, like the error model and File, so the value a binder holds is
// the same type on either transport runtime and the lookups below cannot drift
// from the fasthttp ones.
type QueryValues = bindcore.QueryValues

// Queries parses the request's query string once. Generated binders call this
// a single time per request and resolve each field with QueryLookup, instead
// of re-parsing the raw query per field the way QueryValue does.
func Queries(r *http.Request) QueryValues {
	if r == nil || r.URL == nil {
		return QueryValues{}
	}
	return bindcore.ParseQuery(r.URL.RawQuery)
}

// QueryLookup returns the first value for key from pre-parsed query values.
func QueryLookup(q QueryValues, key string) (string, bool) { return q.Lookup(key) }

// QueryLookupAll returns every value for key, in the order the URL wrote them.
//
// A repeated key is the array spelling a browser produces: an urlencoded form
// writes one pair per successful control, so a checkbox group named tag
// submits tag=a&tag=b. Nothing else is an array here — brackets are ordinary
// key characters, and a comma is an ordinary value character.
//
// An empty value contributes nothing. A blank control submits its key with no
// value, so counting one would turn an untouched filter field into an element
// no user chose, and tag= and a bare tag are indistinguishable anyway.
func QueryLookupAll(q QueryValues, key string) []string { return q.LookupAll(key) }
