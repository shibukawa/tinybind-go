// Package cachekeybind provides typed, reflection-free cache key generation.
//
// A struct marks the fields a cached result depends on, tinybind-gen emits the
// key method, and no call site concatenates anything by hand:
//
//	type UserSummary struct {
//		UserID string `cache:"key"`
//		Page   int    `cache:"key"`
//		Name   string
//		Total  int
//	}
//
//	summary, err := memo(ctx, store, key, fetch) // key.CacheKey() is generated
//
// Marking is opt-in because the struct passed to a cache is often a storage
// entity rather than a purpose-built key: most of its fields are the result
// rather than the query, and keying on those would mean building the key from
// the value the lookup exists to avoid fetching.
//
// One struct yields one key. An entity wanted in a second cache store gets a
// second struct, which keeps the identity below derivable from the type alone.
//
// # What generation does and does not guarantee
//
// It guarantees the identity prefix is present and the framing cannot alias, so
// two key types holding equal field values never reach one entry. It does not
// guarantee completeness: a dependency the author never marks is absent from
// the key, and no compiler can see that requirement. What it removes is every
// failure that survives a correct dependency set.
package cachekeybind

import (
	"strconv"
	"strings"
	"time"
)

// CacheKey reports the cache key of a value. Generated code implements it on
// the value receiver.
//
// The returned string carries the type's identity followed by the framed
// encoding of every marked field, so it is safe to concatenate with a scope
// prefix but must never reach a browser: it holds field values in plaintext.
type CacheKey interface {
	CacheKey() string
}

// Framing rule for the helpers below: every value is written as its byte
// length, a colon, and the value itself. A concatenation of framed values can
// only be split one way, so two different field lists cannot encode to the
// same key.

// KeyString frames a string. It is generic over ~string so a generated enum or
// named string type needs no conversion at the call site.
func KeyString[T ~string](value T) string {
	return strconv.Itoa(len(value)) + ":" + string(value)
}

// KeyBytes frames a byte slice.
func KeyBytes(value []byte) string { return KeyString(string(value)) }

// KeyBool frames a bool.
func KeyBool[T ~bool](value T) string { return KeyString(strconv.FormatBool(bool(value))) }

// KeyInt frames any signed integer. It is generic over every width so a named
// int type or an int64 needs no conversion, which is where htmlbind's own
// KeyInt stops.
func KeyInt[T ~int | ~int8 | ~int16 | ~int32 | ~int64](value T) string {
	return KeyString(strconv.FormatInt(int64(value), 10))
}

// KeyUint frames any unsigned integer.
func KeyUint[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr](value T) string {
	return KeyString(strconv.FormatUint(uint64(value), 10))
}

// KeyFloat frames a float. A float32 is widened first, which is exact, so two
// distinct float32 values still frame distinctly.
//
// Negative zero frames differently from positive zero even though the two
// compare equal. That costs a miss rather than a wrong answer, and the
// alternative is normalizing a value the author chose to key on.
func KeyFloat[T ~float32 | ~float64](value T) string {
	return KeyString(strconv.FormatFloat(float64(value), 'g', -1, 64))
}

// KeyTime frames a time. It uses a fixed layout with nanosecond precision in
// UTC, so two equal instants in different locations encode identically and a
// monotonic reading never reaches the key.
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
//
// A nil slice and an empty slice frame identically. They are equal as inputs to
// a fetch, and distinguishing them would key on a distinction the caller did
// not make.
func KeyArray[T any](values []T, encode func(T) string) string {
	var out strings.Builder
	out.WriteString(strconv.Itoa(len(values)))
	out.WriteByte(':')
	for _, value := range values {
		out.WriteString(encode(value))
	}
	return out.String()
}
