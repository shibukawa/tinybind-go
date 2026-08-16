package htmlbind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMemoryCacheExpiresEntries(t *testing.T) {
	cache := NewMemoryCache(0)
	now := time.Unix(0, 0)
	cache.now = func() time.Time { return now }
	cache.Set(context.Background(), "k", []byte("v"), time.Minute)
	if _, ok := cache.Get(context.Background(), "k"); !ok {
		t.Fatal("fresh entry was not readable")
	}
	now = now.Add(time.Minute)
	if _, ok := cache.Get(context.Background(), "k"); ok {
		t.Fatal("expired entry was still readable")
	}
	if cache.Len() != 0 {
		t.Fatalf("expired entry was not dropped: %d", cache.Len())
	}
}

func TestMemoryCacheBoundsEntryCount(t *testing.T) {
	cache := NewMemoryCache(2)
	for _, key := range []string{"a", "b", "c"} {
		cache.Set(context.Background(), key, []byte(key), time.Minute)
	}
	if cache.Len() != 2 {
		t.Fatalf("cache holds %d entries, want 2", cache.Len())
	}
	if _, ok := cache.Get(context.Background(), "a"); ok {
		t.Fatal("oldest entry survived eviction")
	}
	for _, key := range []string{"b", "c"} {
		if _, ok := cache.Get(context.Background(), key); !ok {
			t.Fatalf("entry %q was evicted too early", key)
		}
	}
}

func TestMemoryCacheIgnoresNonPositiveTTL(t *testing.T) {
	cache := NewMemoryCache(0)
	cache.Set(context.Background(), "k", []byte("v"), 0)
	if cache.Len() != 0 {
		t.Fatal("a zero TTL stored an entry")
	}
}

// TestCacheKeysCannotAlias is the property the framing exists for: no
// combination of parameter values can spell out the encoding of another.
func TestCacheKeysCannotAlias(t *testing.T) {
	pairs := [][2]string{
		{KeyString("ab") + KeyString("c"), KeyString("a") + KeyString("bc")},
		{KeyString("1:a") + KeyString(""), KeyString("") + KeyString("1:a")},
		{KeyArray([]string{"ab"}, KeyString[string]), KeyArray([]string{"a", "b"}, KeyString[string])},
	}
	for _, pair := range pairs {
		if pair[0] == pair[1] {
			t.Fatalf("distinct inputs share the key %q", pair[0])
		}
	}
	absent := KeyOptional[string](nil, KeyString[string])
	empty := KeyOptional(new(string), KeyString[string])
	if absent == empty {
		t.Fatalf("an absent value shares the key of an empty one: %q", absent)
	}
}

func TestCacheKeyTimeIgnoresLocation(t *testing.T) {
	instant := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	elsewhere := instant.In(time.FixedZone("JST", 9*60*60))
	if KeyTime(instant) != KeyTime(elsewhere) {
		t.Fatal("the same instant encoded differently in two locations")
	}
}

// TestCacheKeySeparatesBindingValues is the correctness property the Bindings
// field exists for: an implicit binding is not a declared parameter, so nothing
// else in the key would tell two of its values apart, and a stored body would be
// served across them.
//
// See .knowledge decision:implicit-binding-cache-identity.
func TestCacheKeySeparatesBindingValues(t *testing.T) {
	type params struct{ Name string }
	bindingOf := func(value string) func(context.Context) string {
		return func(context.Context) string { return KeyString(value) }
	}
	policy := CachePolicy[params]{ID: "p", Key: func(p params) string { return KeyString(p.Name) }}

	policy.Bindings = bindingOf("ja")
	ja := policy.cacheKey(context.Background(), "", params{Name: "x"})
	policy.Bindings = bindingOf("en")
	en := policy.cacheKey(context.Background(), "", params{Name: "x"})
	if ja == en {
		t.Fatalf("two binding values share the key %q", ja)
	}

	// A component reading no binding keeps exactly the key it had before the
	// field existed, so a project declaring none is untouched.
	policy.Bindings = nil
	none := policy.cacheKey(context.Background(), "", params{Name: "x"})
	if none != KeyString("p")+KeyString("x") {
		t.Fatalf("a component reading no binding changed key shape: %q", none)
	}

	// The scope stays the prefix, so deleting one remains a key range.
	policy.Scoped = true
	policy.Bindings = bindingOf("ja")
	scoped := policy.cacheKey(context.Background(), "user-7", params{Name: "x"})
	if !strings.HasPrefix(scoped, KeyString("user-7")) {
		t.Fatalf("the binding displaced the scope prefix: %q", scoped)
	}
}
