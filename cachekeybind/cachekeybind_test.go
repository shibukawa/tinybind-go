package cachekeybind_test

import (
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/cachekeybind"
)

type sensorID string

type page int

func TestKeyStringFramesLengthFirst(t *testing.T) {
	if got, want := cachekeybind.KeyString("abc"), "3:abc"; got != want {
		t.Fatalf("KeyString = %q, want %q", got, want)
	}
	if got, want := cachekeybind.KeyString(""), "0:"; got != want {
		t.Fatalf("KeyString empty = %q, want %q", got, want)
	}
}

func TestKeyStringAcceptsNamedStringType(t *testing.T) {
	if got, want := cachekeybind.KeyString(sensorID("s1")), "2:s1"; got != want {
		t.Fatalf("KeyString named = %q, want %q", got, want)
	}
}

// The framing exists so a concatenation can only be split one way. Without it
// "ab"+"c" and "a"+"bc" reach one key, which is a wrong answer rather than a
// stale one.
func TestFramingCannotAliasAcrossFieldBoundaries(t *testing.T) {
	left := cachekeybind.KeyString("ab") + cachekeybind.KeyString("c")
	right := cachekeybind.KeyString("a") + cachekeybind.KeyString("bc")
	if left == right {
		t.Fatalf("framed pairs collide: %q", left)
	}
}

func TestKeyIntCoversEveryWidthAndNamedTypes(t *testing.T) {
	if got, want := cachekeybind.KeyInt(42), "2:42"; got != want {
		t.Fatalf("KeyInt = %q, want %q", got, want)
	}
	if got, want := cachekeybind.KeyInt(int64(-9007199254740993)), "17:-9007199254740993"; got != want {
		t.Fatalf("KeyInt int64 = %q, want %q", got, want)
	}
	// A named int type is exactly what htmlbind's own KeyInt cannot take, and
	// the reason this package frames its own helpers rather than importing them.
	if got, want := cachekeybind.KeyInt(page(7)), "1:7"; got != want {
		t.Fatalf("KeyInt named = %q, want %q", got, want)
	}
}

func TestKeyUintCoversUnsigned(t *testing.T) {
	if got, want := cachekeybind.KeyUint(uint64(18446744073709551615)), "20:18446744073709551615"; got != want {
		t.Fatalf("KeyUint = %q, want %q", got, want)
	}
}

func TestKeyBoolAndKeyBytes(t *testing.T) {
	if got, want := cachekeybind.KeyBool(true), "4:true"; got != want {
		t.Fatalf("KeyBool = %q, want %q", got, want)
	}
	if got, want := cachekeybind.KeyBytes([]byte("hi")), "2:hi"; got != want {
		t.Fatalf("KeyBytes = %q, want %q", got, want)
	}
}

// A float32 widens exactly, so two distinct float32 values must stay distinct.
func TestKeyFloatSeparatesDistinctFloat32Values(t *testing.T) {
	a := cachekeybind.KeyFloat(float32(0.1))
	b := cachekeybind.KeyFloat(float32(0.2))
	if a == b {
		t.Fatalf("distinct float32 values framed alike: %q", a)
	}
	if got, want := cachekeybind.KeyFloat(1.5), "3:1.5"; got != want {
		t.Fatalf("KeyFloat = %q, want %q", got, want)
	}
}

// Two equal instants in different locations describe one moment, so they must
// reach one entry.
func TestKeyTimeIsLocationIndependent(t *testing.T) {
	utc := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	east := utc.In(time.FixedZone("east", 9*3600))
	if cachekeybind.KeyTime(utc) != cachekeybind.KeyTime(east) {
		t.Fatalf("same instant framed differently: %q vs %q", cachekeybind.KeyTime(utc), cachekeybind.KeyTime(east))
	}
}

func TestKeyOptionalSeparatesAbsenceFromEveryValue(t *testing.T) {
	encode := func(v string) string { return cachekeybind.KeyString(v) }
	absent := cachekeybind.KeyOptional(nil, encode)
	empty := ""
	present := cachekeybind.KeyOptional(&empty, encode)
	if absent == present {
		t.Fatalf("nil and empty framed alike: %q", absent)
	}
}

// A slice of one two-element string must not collide with two one-element ones.
func TestKeyArrayCountsBeforeElements(t *testing.T) {
	encode := func(v string) string { return cachekeybind.KeyString(v) }
	one := cachekeybind.KeyArray([]string{"ab"}, encode)
	two := cachekeybind.KeyArray([]string{"a", "b"}, encode)
	if one == two {
		t.Fatalf("distinct slices framed alike: %q", one)
	}
}

// A nil slice and an empty slice are equal as inputs to a fetch.
func TestKeyArrayTreatsNilAsEmpty(t *testing.T) {
	encode := func(v string) string { return cachekeybind.KeyString(v) }
	if cachekeybind.KeyArray(nil, encode) != cachekeybind.KeyArray([]string{}, encode) {
		t.Fatal("nil and empty slice framed differently")
	}
}

type staticKey struct{}

func (staticKey) CacheKey() string { return "0:" }

func TestCacheKeyInterfaceIsSatisfiableByAValue(t *testing.T) {
	var key cachekeybind.CacheKey = staticKey{}
	if key.CacheKey() != "0:" {
		t.Fatalf("CacheKey = %q", key.CacheKey())
	}
}
