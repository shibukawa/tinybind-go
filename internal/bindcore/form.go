package bindcore

import (
	"encoding/json"
	"strings"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// FormValues is a form body read once. An urlencoded body is the same
// key=value spans a query string is, split the same way and unescaped only
// when a key is actually looked up, so the url.Values map, its per-key
// slices and the copy of every member that net/http's ParseForm built are
// not built at all. A multipart body keeps the map its parser produced, since
// that parser has already paid for the copies.
//
// The zero value is the absence of a form body, which is what a binder asks
// Present about before reading the fields that may come from one.
type FormValues struct {
	present bool
	q       QueryValues
	m       map[string]string
}

// FormFromQuery wraps an urlencoded body already split into pairs.
func FormFromQuery(q QueryValues) FormValues { return FormValues{present: true, q: q} }

// FormFromMap wraps the fields of a multipart body.
func FormFromMap(m map[string]string) FormValues { return FormValues{present: true, m: m} }

// Present reports whether the request carried a form body at all.
func (f FormValues) Present() bool { return f.present }

// Lookup returns the first value for key, the way ParseForm's map did.
func (f FormValues) Lookup(key string) (string, bool) {
	if f.m != nil {
		v, ok := f.m[key]
		return v, ok
	}
	return f.q.Lookup(key)
}

// each visits every decodable pair once, first occurrence of a key first,
// which is the first-wins rule the map form kept.
func (f FormValues) each(fn func(key, value string)) {
	if f.m != nil {
		for k, v := range f.m {
			fn(k, v)
		}
		return
	}
	seen := map[string]struct{}{}
	for i := range f.q.pairs {
		k, v, ok := decodeQueryPair(f.q.pairs[i].rawKey, f.q.pairs[i].rawValue)
		if !ok {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		fn(k, v)
	}
}

// decodeQueryPair unescapes both halves of a pair, refusing the pair when
// either does not decode, as Lookup does.
func decodeQueryPair(rawKey, rawValue string) (key, value string, ok bool) {
	key, value = rawKey, rawValue
	if QueryNeedsUnescape(rawKey) {
		if key, ok = unescapeQuery(rawKey); !ok {
			return "", "", false
		}
	}
	if QueryNeedsUnescape(rawValue) {
		if value, ok = unescapeQuery(rawValue); !ok {
			return "", "", false
		}
	}
	return key, value, true
}

// RestAny builds map[string]any from the form keys not in exclude.
func (f FormValues) RestAny(exclude []string) map[string]any {
	out := make(map[string]any)
	if !f.present {
		return out
	}
	f.each(func(k, v string) {
		if !isExcluded(exclude, k) {
			out[k] = v
		}
	})
	return out
}

// RestRaw builds map[string]json.RawMessage from the form keys not in
// exclude, each value quoted by the same append helper generated encoders
// use.
func (f FormValues) RestRaw(exclude []string) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage)
	if !f.present {
		return out
	}
	f.each(func(k, v string) {
		if !isExcluded(exclude, k) {
			out[k] = json.RawMessage(jsonbind.AppendString(nil, v))
		}
	})
	return out
}

// MaxFormBodyBytes is the most an urlencoded body may hold, the bound
// net/http's ParseForm applies.
const MaxFormBodyBytes = 10 << 20

// ReadFormValues splits an urlencoded body the caller has read into pairs.
// body must be owned by the caller; the values alias it.
func ReadFormValues(body string) FormValues {
	return FormFromQuery(ParseQuery(strings.TrimSpace(body)))
}
