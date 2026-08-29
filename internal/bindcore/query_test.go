package bindcore

import (
	"net/url"
	"testing"
)

// unescapeQuery and ParseQuery restate rules net/url already owns, so the tests
// that matter compare them against net/url rather than against a table someone
// wrote from the same reading of the same spec. net/url is imported here and
// nowhere in the package itself, which is the whole point: a fasthttp build
// links the copy, and this is what proves the copy still answers the same.

var unescapeCorpus = []string{
	"", "plain", "a+b", "+", "++", "%20", "%2F", "%2f", "%41%42%43",
	"%", "%a", "%zz", "%2", "a%2", "a%", "%%", "%%41", "100%", "%e3%81%82",
	"a+b%20c", "=", "&", "%00", "%7F", "%FF", "%ff%fe", "a%2Bb", "%25", "%2520",
}

func TestUnescapeQueryMatchesNetURL(t *testing.T) {
	for _, in := range unescapeCorpus {
		checkUnescapeQuery(t, in)
	}
}

func FuzzUnescapeQueryMatchesNetURL(f *testing.F) {
	for _, in := range unescapeCorpus {
		f.Add(in)
	}
	f.Fuzz(func(t *testing.T, in string) { checkUnescapeQuery(t, in) })
}

func checkUnescapeQuery(t *testing.T, in string) {
	t.Helper()
	want, wantErr := url.QueryUnescape(in)
	got, ok := unescapeQuery(in)
	if ok != (wantErr == nil) {
		t.Fatalf("unescapeQuery(%q) ok = %v, url.QueryUnescape err = %v", in, ok, wantErr)
	}
	if ok && got != want {
		t.Fatalf("unescapeQuery(%q) = %q, url.QueryUnescape = %q", in, got, want)
	}
}

var parseQueryCorpus = []string{
	"", "a=1", "a=1&a=2", "a", "a=", "=1", "a=1;b=2", ";", "a;b=1",
	"a%2Fb=c", "a+b=c+d", "%zz=1", "a=%zz", "a=%", "&&a=1", "a=1&", "%41=%42",
	"a=%E3%81%82", "tag=a&tag=&tag=b", "a=1&b=2&a=3",
}

// TestParseQueryMatchesNetURL pins the admission rules: which pairs exist at
// all, and what each decodes to, has to be url.ParseQuery's answer, because a
// binder and an application reading r.URL.Query() would otherwise see different
// requests for the same bytes.
func TestParseQueryMatchesNetURL(t *testing.T) {
	for _, raw := range parseQueryCorpus {
		for _, key := range []string{"a", "b", "tag", "", "a/b", "a b"} {
			checkParseQuery(t, raw, key)
		}
	}
}

func FuzzParseQueryMatchesNetURL(f *testing.F) {
	for _, raw := range parseQueryCorpus {
		f.Add(raw, "a")
	}
	f.Fuzz(func(t *testing.T, raw, key string) { checkParseQuery(t, raw, key) })
}

func checkParseQuery(t *testing.T, raw, key string) {
	t.Helper()
	// url.ParseQuery reports an error for the pairs it drops and still returns
	// the ones it kept, which is the behaviour being matched; the error itself
	// says nothing about any single key.
	want, _ := url.ParseQuery(raw)
	values := ParseQuery(raw)

	gotValue, gotOK := values.Lookup(key)
	if gotOK != (len(want[key]) > 0) {
		t.Fatalf("raw=%q key=%q: Lookup ok = %v, url.ParseQuery has %d values", raw, key, gotOK, len(want[key]))
	}
	if gotOK && gotValue != want[key][0] {
		t.Fatalf("raw=%q key=%q: Lookup = %q, want %q", raw, key, gotValue, want[key][0])
	}

	scanValue, scanOK := ScanQuery(raw, key)
	if scanOK != gotOK || scanValue != gotValue {
		t.Fatalf("raw=%q key=%q: ScanQuery = (%q,%v), Lookup = (%q,%v)", raw, key, scanValue, scanOK, gotValue, gotOK)
	}

	// LookupAll is url.ParseQuery's list without the empty members, which is
	// the one documented departure: an empty value is a blank control, not an
	// element the user chose.
	var wantAll []string
	for _, v := range want[key] {
		if v != "" {
			wantAll = append(wantAll, v)
		}
	}
	gotAll := values.LookupAll(key)
	if len(gotAll) != len(wantAll) {
		t.Fatalf("raw=%q key=%q: LookupAll = %q, want %q", raw, key, gotAll, wantAll)
	}
	for i := range gotAll {
		if gotAll[i] != wantAll[i] {
			t.Fatalf("raw=%q key=%q: LookupAll = %q, want %q", raw, key, gotAll, wantAll)
		}
	}
}
