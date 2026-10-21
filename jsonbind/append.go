package jsonbind

import (
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

// Generated encoders build a document by appending to a byte slice and write it
// once. Escaping matches encoding/json's default encoder byte for byte, so
// switching a codec over does not change the bytes on the wire.

var bufPool = sync.Pool{New: func() any { b := make([]byte, 0, 1024); return &b }}

// GetBuffer borrows an encode buffer. Generated code returns it with
// PutBuffer once the bytes have been written out.
func GetBuffer() *[]byte {
	b := bufPool.Get().(*[]byte)
	*b = (*b)[:0]
	return b
}

// PutBuffer returns an encode buffer to the pool. Oversized buffers are
// dropped so one large document does not pin memory for the process lifetime.
func PutBuffer(b *[]byte) {
	if b == nil || cap(*b) > 1<<20 {
		return
	}
	bufPool.Put(b)
}

const hexDigits = "0123456789abcdef"

// AppendString appends a JSON string literal. Like encoding/json's encoder it
// escapes <, > and & so the result is safe to embed in HTML, and U+2028/U+2029
// so it stays valid JavaScript.
//
// A run of plain ASCII is passed over eight bytes at a time, the scan
// Parser.String makes on the way in, and copied through in one piece; the
// last seven bytes, which are the whole of a short value, go one at a time. A
// run of multibyte runes stays with the rune decoder rather than returning to
// the word scan at every rune, since each of them would stop it again.
func AppendString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	start := 0
	i := 0
	for i < len(s) {
		for i+8 <= len(s) {
			if m := swarEscapeStop(loadWord(s[i:])); m != 0 {
				i += swarIndex(m)
				break
			}
			i += 8
		}
		for i < len(s) && !escapeStop(s[i]) {
			i++
		}
		if i >= len(s) {
			break
		}
		c := s[i]
		if c < utf8.RuneSelf {
			dst = append(dst, s[start:i]...)
			switch c {
			case '"':
				dst = append(dst, '\\', '"')
			case '\\':
				dst = append(dst, '\\', '\\')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
			}
			i++
			start = i
			continue
		}
		for i < len(s) && s[i] >= utf8.RuneSelf {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				dst = append(dst, s[start:i]...)
				dst = append(dst, '\\', 'u', 'f', 'f', 'f', 'd')
				i += size
				start = i
				continue
			}
			if r == '\u2028' || r == '\u2029' {
				dst = append(dst, s[start:i]...)
				dst = append(dst, '\\', 'u', '2', '0', '2', hexDigits[r&0xF])
				i += size
				start = i
				continue
			}
			i += size
		}
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"')
}

// escapeStop reports a byte AppendString cannot copy through: a control byte,
// one of the five escaped punctuation bytes, or the start of a multibyte
// sequence. It is the byte form of swarEscapeStop.
func escapeStop(c byte) bool {
	return c < 0x20 || c >= utf8.RuneSelf || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&'
}

// loadWord reads the first eight bytes of s as one little-endian word. Written
// as byte loads rather than through encoding/binary because the parser's
// buffer is bytes and this one is a string; the compiler fuses the eight loads
// into one.
func loadWord(s string) uint64 {
	_ = s[7]
	return uint64(s[0]) | uint64(s[1])<<8 | uint64(s[2])<<16 | uint64(s[3])<<24 |
		uint64(s[4])<<32 | uint64(s[5])<<40 | uint64(s[6])<<48 | uint64(s[7])<<56
}

// swarEscapeStop marks the lanes AppendString cannot copy through: a byte
// below 0x20, one of the five punctuation bytes encoding/json escapes, or a
// byte with the high bit set. The quote and the ampersand (0x22, 0x26) differ
// in one bit, as do the two angle brackets (0x3c, 0x3e), so each pair is one
// has-zero test over the word with that bit forced on. As in swarStringStop a
// borrow can mark a lane above a hit but never below one, and the caller
// takes the lowest.
func swarEscapeStop(w uint64) uint64 {
	qa := (w | swarLow*0x04) ^ (swarLow * '&')
	lg := (w | swarLow*0x02) ^ (swarLow * '>')
	bs := w ^ (swarLow * '\\')
	return ((w-swarLow*0x20)&^w | (qa-swarLow)&^qa | (lg-swarLow)&^lg | (bs-swarLow)&^bs | w) & swarHigh
}

// AppendInt appends a JSON number.
func AppendInt(dst []byte, v int64) []byte { return strconv.AppendInt(dst, v, 10) }

// AppendUint appends a JSON number.
func AppendUint(dst []byte, v uint64) []byte { return strconv.AppendUint(dst, v, 10) }

// AppendBool appends a JSON boolean.
func AppendBool(dst []byte, v bool) []byte { return strconv.AppendBool(dst, v) }

// AppendFloat appends a JSON number using encoding/json's formatting: shortest
// round-trip, switching to exponent form outside [1e-6, 1e21) and trimming the
// exponent's leading zero.
func AppendFloat(dst []byte, v float64) []byte {
	if dst, ok := appendShortFloat(dst, v); ok {
		return dst
	}
	abs := v
	if abs < 0 {
		abs = -abs
	}
	format := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	dst = strconv.AppendFloat(dst, v, format, -1, 64)
	if format == 'e' {
		n := len(dst)
		if n >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
			dst[n-2] = dst[n-1]
			dst = dst[:n-1]
		}
	}
	return dst
}

// AppendRaw appends an already-encoded JSON value, or null when it is empty.
func AppendRaw(dst []byte, raw []byte) []byte {
	if len(raw) == 0 {
		return append(dst, "null"...)
	}
	return append(dst, raw...)
}

// AppendAny appends an arbitrary Go value produced by rest-field decoding.
// It covers the shapes Parser.Any yields plus the common scalar types, and any
// type carrying its own encoder through [Appender]; anything else is written as
// null rather than failing an otherwise valid response.
//
// The Appender arm is what keeps a user type out of that null. Before it a
// value the switch did not name — which is every named struct — reached the
// default and encoded as null, producing a wrong document rather than a
// reported error. It sits after the concrete cases so a builtin shape is still
// matched by identity rather than by method set.
func AppendAny(dst []byte, v any) []byte {
	switch t := v.(type) {
	case nil:
		return append(dst, "null"...)
	case string:
		return AppendString(dst, t)
	case bool:
		return AppendBool(dst, t)
	case float64:
		return AppendFloat(dst, t)
	case float32:
		return AppendFloat(dst, float64(t))
	case int:
		return AppendInt(dst, int64(t))
	case int8:
		return AppendInt(dst, int64(t))
	case int16:
		return AppendInt(dst, int64(t))
	case int32:
		return AppendInt(dst, int64(t))
	case int64:
		return AppendInt(dst, t)
	case uint:
		return AppendUint(dst, uint64(t))
	case uint8:
		return AppendUint(dst, uint64(t))
	case uint16:
		return AppendUint(dst, uint64(t))
	case uint32:
		return AppendUint(dst, uint64(t))
	case uint64:
		return AppendUint(dst, t)
	case []byte:
		return AppendRaw(dst, t)
	case []any:
		dst = append(dst, '[')
		for i, e := range t {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = AppendAny(dst, e)
		}
		return append(dst, ']')
	case map[string]any:
		dst = append(dst, '{')
		for i, k := range sortedKeys(t) {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = AppendString(dst, k)
			dst = append(dst, ':')
			dst = AppendAny(dst, t[k])
		}
		return append(dst, '}')
	case []string:
		dst = append(dst, '[')
		for i, e := range t {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = AppendString(dst, e)
		}
		return append(dst, ']')
	case Appender:
		return t.AppendJSONTo(dst)
	default:
		return append(dst, "null"...)
	}
}

// SortedKeys orders map keys so a map encodes deterministically, matching
// encoding/json's behaviour. Generated encoders call it for every map-typed
// field and for payload:"*" rest maps.
func SortedKeys[V any](m map[string]V) []string { return sortedKeys(m) }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Insertion sort: rest maps are small, and this avoids pulling the sort
	// package (and its reflect-adjacent generic machinery) into TinyGo builds.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// appendShortFloat writes v when it is an integer or a decimal of at most
// three fraction digits, which is what most numbers in a document are: a
// count, a price, a ratio. strconv's shortest formatting is the fallback and
// costs about twice as much.
//
// The output is the one strconv would give. An integer below 2^53 is its own
// shortest decimal, since floats are spaced no wider than one there. For a
// fraction, the candidates are tried in order of length and the first whose
// division comes back as exactly v is taken: division is correctly rounded,
// so that candidate round-trips; nothing shorter does, or it would have been
// taken first; and no other decimal of the same length rounds to v, because
// below 1e9 the spacing of such decimals is far wider than the spacing of
// floats. Zero is left to strconv so a negative zero keeps its sign.
func appendShortFloat(dst []byte, v float64) ([]byte, bool) {
	abs := math.Abs(v)
	if v == 0 || abs >= 1e15 || abs != abs {
		return dst, false
	}
	if abs == math.Trunc(abs) {
		return strconv.AppendInt(dst, int64(v), 10), true
	}
	if abs >= 1e9 {
		return dst, false
	}
	for k := 1; k <= 3; k++ {
		scaled := math.Round(abs * pow10[k])
		if scaled/pow10[k] != abs {
			continue
		}
		if v < 0 {
			dst = append(dst, '-')
		}
		m := int64(scaled)
		whole, frac := m/int64(pow10[k]), m%int64(pow10[k])
		dst = strconv.AppendInt(dst, whole, 10)
		dst = append(dst, '.')
		for d := k - 1; d >= 1; d-- {
			if frac < int64(pow10[d]) {
				dst = append(dst, '0')
			}
		}
		return strconv.AppendInt(dst, frac, 10), true
	}
	return dst, false
}

// SizeHint remembers how large a type's encoding has been, so an encoder that
// must hand its result to the caller allocates once at that size rather than
// growing through several reallocations. Generated AppendJSONTo methods keep
// one per type. The zero value is ready to use and safe for concurrent use.
type SizeHint struct {
	n atomic.Int64
}

// Grow returns dst with room for an encoding the size of the largest one
// recorded so far, plus an eighth. A buffer that already has the room, and
// a hint that has recorded nothing yet, come back untouched.
func (h *SizeHint) Grow(dst []byte) []byte {
	need := int(h.n.Load())
	need += need / 8
	if need == 0 || cap(dst)-len(dst) >= need {
		return dst
	}
	grown := make([]byte, len(dst), len(dst)+need)
	copy(grown, dst)
	return grown
}

// Record notes the length of a finished encoding. The hint only grows, so one
// large value does not make every later call start small and grow again.
func (h *SizeHint) Record(n int) {
	for {
		old := h.n.Load()
		if int64(n) <= old || h.n.CompareAndSwap(old, int64(n)) {
			return
		}
	}
}
