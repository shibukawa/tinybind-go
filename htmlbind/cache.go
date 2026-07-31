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
}

// cacheKey joins the plan fingerprint with the encoded parameters. Both parts
// are framed, so no parameter value can spell out another component's key.
func (c *CachePolicy[P]) cacheKey(params P) string {
	return KeyString(c.ID) + c.Key(params)
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
	order   []string
	max     int
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
		c.remove(key)
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
	for c.max > 0 && len(c.entries) > c.max {
		c.remove(c.order[0])
	}
}

// Len reports how many entries the cache currently holds, including entries
// that have expired but not yet been evicted.
func (c *MemoryCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// remove drops one key. The caller holds the lock.
func (c *MemoryCache) remove(key string) {
	delete(c.entries, key)
	for i, candidate := range c.order {
		if candidate == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}
