package htmlbind

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CacheStore holds rendered component output for components declared with the
// cache annotation. The caller supplies one per render through WithCache, so a
// store is an ordinary caller resource rather than package state.
//
// An implementation is used from several goroutines during one render and must
// be safe for concurrent use.
type CacheStore interface {
	// Get returns previously stored output for key. An expired or absent entry
	// reports false. The returned bytes are written unmodified, so a store must
	// not reuse or mutate the slice it hands back.
	Get(ctx context.Context, key string) ([]byte, bool)
	// Set stores output for at most ttl. It returns nothing: a cache write
	// failure must not fail a response that already rendered correctly, so an
	// implementation reports its own failures.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration)
}

// CachePolicy is the cache configuration compiled into a component's plan.
// Generated code builds one; application code never does.
type CachePolicy[P any] struct {
	// ID is the component identity plus a fingerprint of its generated plan, so
	// regenerated code cannot read entries written by the previous code.
	ID string
	// TTL is how long an entry may be reused.
	TTL time.Duration
	// Key appends the canonical encoding of every declared parameter.
	Key func(P) string
	// Bindings appends the framed value of every implicit binding this
	// component's call graph reads, in the order they were declared to the
	// generator. It is nil for a component reading none, which is every
	// component in a project that declares no binding.
	//
	// A binding is not a declared parameter, so nothing else in this key would
	// tell two values of it apart, and a stored body would be served across
	// them. See .knowledge decision:implicit-binding-cache-identity.
	Bindings func(context.Context) string
	// Scoped marks a component declared private, whose key is prefixed with the
	// render's scope value so one key yields a separate entry per reader.
	//
	// It is the default for a declared cache: a component that is actually
	// shared says so with scope: "public", because the cost of getting that
	// wrong is a miss and the cost of the other mistake is one reader's output
	// served to another.
	Scoped bool
}

// cacheKey joins the scope, the plan fingerprint, and the encoded parameters.
// Every part is framed, so no value can spell out another component's key.
//
// The scope is prepended rather than appended so entries for one reader share a
// prefix, which a store that organizes by key range can use. A public component
// passes an empty scope and gets no prefix at all, which is what keeps its key
// identical to the one it had before scoping existed.
func (c *CachePolicy[P]) cacheKey(ctx context.Context, scope string, params P) string {
	// Binding values sit after the scope prefix and before the parameters. They
	// cannot go first: the scope is the prefix a store deletes by range, and
	// putting anything ahead of it would break that.
	bindings := ""
	if c.Bindings != nil {
		bindings = c.Bindings(ctx)
	}
	if !c.Scoped {
		return KeyString(c.ID) + bindings + c.Key(params)
	}
	return KeyString(scope) + KeyString(c.ID) + bindings + c.Key(params)
}

// Framing rule for the helpers below: every value is written as its byte
// length, a colon, and the value itself. A concatenation of framed values can
// only be split one way, so two different parameter lists cannot encode to the
// same key.

// KeyString frames a string for a cache key. It is generic over ~string so a
// generated enum or trusted string type needs no conversion at the call site.
func KeyString[T ~string](value T) string {
	return strconv.Itoa(len(value)) + ":" + string(value)
}

// KeyBytes frames a byte slice for a cache key.
func KeyBytes(value []byte) string { return KeyString(string(value)) }

// KeyBool frames a bool for a cache key.
func KeyBool(value bool) string { return KeyString(strconv.FormatBool(value)) }

// KeyInt frames an int for a cache key.
func KeyInt(value int) string { return KeyString(strconv.Itoa(value)) }

// KeyFloat frames a float64 for a cache key.
func KeyFloat(value float64) string {
	return KeyString(strconv.FormatFloat(value, 'g', -1, 64))
}

// KeyTime frames a time for a cache key. It uses a fixed layout with nanosecond
// precision so two equal instants in different locations encode identically.
func KeyTime(value time.Time) string {
	return KeyString(value.UTC().Format(time.RFC3339Nano))
}

// KeyOptional frames a pointer, distinguishing absence from any present value.
func KeyOptional[T any](value *T, encode func(T) string) string {
	if value == nil {
		return "-"
	}
	return "+" + encode(*value)
}

// KeyArray frames a slice as its element count followed by its framed elements,
// so a slice of one two-element string cannot collide with two one-element ones.
func KeyArray[T any](values []T, encode func(T) string) string {
	var out strings.Builder
	out.WriteString(strconv.Itoa(len(values)))
	out.WriteByte(':')
	for _, value := range values {
		out.WriteString(encode(value))
	}
	return out.String()
}

// MemoryCache is an in-process CacheStore with TTL expiry and a maximum entry
// count. It is the default a single-process server needs; a shared store is an
// adapter the caller writes.
type MemoryCache struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	// order approximates insertion age. Eviction pops from the front by moving
	// head instead of shifting the slice; a key expiry already removed keeps its
	// slot and is skipped when it surfaces.
	order []string
	head  int
	max   int
	// now is swappable so expiry is testable without sleeping.
	now func() time.Time
}

type memoryEntry struct {
	value   []byte
	expires time.Time
}

// NewMemoryCache returns a store holding at most maxEntries entries. A
// non-positive maxEntries means unbounded.
func NewMemoryCache(maxEntries int) *MemoryCache {
	return &MemoryCache{entries: map[string]memoryEntry{}, max: maxEntries, now: time.Now}
}

// Get implements CacheStore.
func (c *MemoryCache) Get(_ context.Context, key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !c.now().Before(entry.expires) {
		delete(c.entries, key)
		c.compact()
		return nil, false
	}
	return entry.value, true
}

// Set implements CacheStore. A non-positive ttl stores nothing.
func (c *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}
	c.entries[key] = memoryEntry{value: value, expires: c.now().Add(ttl)}
	// Insertion order approximates age well enough for a render cache, and it
	// avoids the bookkeeping a true LRU would add to every hit.
	for c.max > 0 && len(c.entries) > c.max && c.head < len(c.order) {
		oldest := c.order[c.head]
		c.head++
		delete(c.entries, oldest)
	}
	c.compact()
}

// Len reports how many entries the cache currently holds, including entries
// that have expired but not yet been evicted.
func (c *MemoryCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// compact rebuilds order once most of it no longer names a live entry, so the
// queue cannot grow past the map it approximates. The caller holds the lock.
func (c *MemoryCache) compact() {
	if len(c.order)-c.head <= len(c.entries)*2+16 {
		return
	}
	kept := c.order[:0]
	for _, key := range c.order[c.head:] {
		if _, ok := c.entries[key]; ok {
			kept = append(kept, key)
		}
	}
	c.order, c.head = kept, 0
}
