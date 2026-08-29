package bindcore

import "strings"

// The query view lives here whole, for the reason the socket and the stream do:
// which pairs a raw query even has, and what a span decodes to, is behaviour
// two runtimes would be two chances to disagree about.
//
// They did disagree. The net/http surface scanned the raw query under
// url.ParseQuery's admission rules while the fasthttp one read the driver's
// parsed Args, so ?a=1;b=2 bound nothing on one and "1;b=2" on the other, and a
// broken escape bound nothing on one and its literal text on the other. Both
// spellings are the client's to choose, which makes the difference an attacker's
// to choose. One implementation is the only way that stays fixed.

// QueryValues is a request's query string split once into raw key=value spans,
// in wire order.
//
// Generated binders only ever ask for a key's first value, so the url.Values
// map — one allocation per key plus an unescaped copy of every member — buys
// random access nobody uses. Splitting into spans costs one slice, and a span
// is unescaped only when it is actually looked up and actually escaped: the
// same trade the binders' inline JSON walk makes for body members.
type QueryValues struct {
	pairs []queryPair
}

// queryPair holds one undecoded key=value span of the raw query.
type queryPair struct{ rawKey, rawValue string }

// ParseQuery splits raw into the pairs a lookup may answer from.
//
// raw must be a string the caller owns. The spans alias it and every value
// handed back is cut from it, which is what lets the fasthttp surface convert
// the pooled query bytes once here rather than per field.
func ParseQuery(raw string) QueryValues {
	if raw == "" {
		return QueryValues{}
	}
	n := 1
	for i := 0; i < len(raw); i++ {
		if raw[i] == '&' {
			n++
		}
	}
	pairs := make([]queryPair, 0, n)
	for len(raw) > 0 {
		var seg string
		seg, raw = cutQuerySegment(raw)
		if !admitsQuerySegment(seg) {
			continue
		}
		k, v, _ := strings.Cut(seg, "=")
		pairs = append(pairs, queryPair{rawKey: k, rawValue: v})
	}
	return QueryValues{pairs: pairs}
}

// Lookup returns the first value for key.
func (q QueryValues) Lookup(key string) (string, bool) {
	for i := range q.pairs {
		if v, ok, matched := MatchQueryPair(q.pairs[i].rawKey, q.pairs[i].rawValue, key); matched {
			return v, ok
		}
	}
	return "", false
}

// LookupAll returns every value for key, in the order the URL wrote them.
//
// A repeated key is the array spelling a browser produces: an urlencoded form
// writes one pair per successful control, so a checkbox group named tag submits
// tag=a&tag=b. Nothing else is an array here — brackets are ordinary key
// characters, and a comma is an ordinary value character.
//
// An empty value contributes nothing. A blank control submits its key with no
// value, so counting one would turn an untouched filter field into an element
// no user chose, and tag= and a bare tag are indistinguishable anyway.
func (q QueryValues) LookupAll(key string) []string {
	var out []string
	for i := range q.pairs {
		v, ok, matched := MatchQueryPair(q.pairs[i].rawKey, q.pairs[i].rawValue, key)
		if matched && ok && v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ScanQuery resolves one key straight off the raw query, for the callers that
// want a single value and would waste a full split on it.
func ScanQuery(raw, key string) (string, bool) {
	for len(raw) > 0 {
		var seg string
		seg, raw = cutQuerySegment(raw)
		if !admitsQuerySegment(seg) {
			continue
		}
		k, v, _ := strings.Cut(seg, "=")
		if value, ok, matched := MatchQueryPair(k, v, key); matched {
			return value, ok
		}
	}
	return "", false
}

// cutQuerySegment takes the next &-delimited span off raw.
func cutQuerySegment(raw string) (seg, rest string) {
	if i := strings.IndexByte(raw, '&'); i >= 0 {
		return raw[:i], raw[i+1:]
	}
	return raw, ""
}

// admitsQuerySegment reports whether a span is a pair at all. url.ParseQuery
// drops empty segments and rejects semicolon-bearing pairs; matching that keeps
// this view and r.URL.Query() agreeing on which pairs exist, which is the whole
// point of not using the map.
func admitsQuerySegment(seg string) bool {
	return seg != "" && strings.IndexByte(seg, ';') < 0
}

// MatchQueryPair decides one raw pair against a wanted key. matched reports
// that the pair answered the lookup; a pair whose escapes do not decode is
// treated the way url.ParseQuery treats it — as if it were not there.
func MatchQueryPair(rawKey, rawValue, key string) (value string, ok, matched bool) {
	if QueryNeedsUnescape(rawKey) {
		k, decoded := unescapeQuery(rawKey)
		if !decoded || k != key {
			return "", false, false
		}
	} else if rawKey != key {
		return "", false, false
	}
	if !QueryNeedsUnescape(rawValue) {
		return rawValue, true, true
	}
	v, decoded := unescapeQuery(rawValue)
	if !decoded {
		return "", false, false
	}
	return v, true, true
}

// QueryNeedsUnescape reports whether a raw span decodes to something other than
// itself. The common query — unescaped ASCII keys and values — skips the decode
// and its allocation entirely.
func QueryNeedsUnescape(s string) bool {
	return strings.IndexByte(s, '%') >= 0 || strings.IndexByte(s, '+') >= 0
}

// unescapeQuery decodes one raw span the way url.QueryUnescape does: '+' is a
// space, %XX is a byte, and a '%' not followed by two hex digits makes the
// whole span undecodable rather than literal.
//
// It is spelled out rather than delegated so this package stays off net/url,
// which a fasthttp build otherwise never links and a TinyGo target would pay
// for. The equivalence is not assumed: TestUnescapeQueryMatchesNetURL drives
// both over the same corpus, and FuzzUnescapeQueryMatchesNetURL keeps doing so.
func unescapeQuery(s string) (string, bool) {
	escapes := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+2 >= len(s) || !isHexDigit(s[i+1]) || !isHexDigit(s[i+2]) {
			return "", false
		}
		escapes++
		i += 2
	}
	out := make([]byte, 0, len(s)-2*escapes)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '%':
			out = append(out, unhexDigit(s[i+1])<<4|unhexDigit(s[i+2]))
			i += 2
		case '+':
			out = append(out, ' ')
		default:
			out = append(out, s[i])
		}
	}
	return string(out), true
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func unhexDigit(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}
