package htmlbind

import (
	"math/bits"
	"unicode/utf8"
)

// The escapers scan a value eight bytes at a time, the way jsonbind's parser
// and encoder do: one mask marks every lane the scan cannot copy through, and
// the byte loop takes over only at a hit. Both escape sets here are small
// enough that the mask is a handful of operations, which is what lets a
// single pass beat the two library scans the HTML side used as a prefilter.

const (
	wordLow  = 0x0101010101010101
	wordHigh = 0x8080808080808080
)

// loadWord reads the first eight bytes of v as one little-endian word. Written
// as byte loads rather than through encoding/binary because the type set has a
// string in it; the compiler fuses the eight loads into one for either member.
func loadWord[T ~string | ~[]byte](v T) uint64 {
	_ = v[7]
	return uint64(v[0]) | uint64(v[1])<<8 | uint64(v[2])<<16 | uint64(v[3])<<24 |
		uint64(v[4])<<32 | uint64(v[5])<<40 | uint64(v[6])<<48 | uint64(v[7])<<56
}

// wordIndex is the byte offset of the lowest marked lane. A borrow crossing a
// lane boundary can mark a lane above a genuine hit but never below one, so
// the lowest marked lane is always genuine.
func wordIndex(mask uint64) int { return bits.TrailingZeros64(mask) / 8 }

// htmlEntityWord marks the lanes holding one of the five characters Escape
// writes as entities. The ampersand and the quote (0x26, 0x22) differ in one
// bit, as do the angle brackets (0x3c, 0x3e), so each pair is one has-zero
// test over the word with that bit forced on; the apostrophe is the third.
func htmlEntityWord(w uint64) uint64 {
	qa := (w | wordLow*0x04) ^ (wordLow * '&')
	lg := (w | wordLow*0x02) ^ (wordLow * '>')
	ap := w ^ (wordLow * '\'')
	return ((qa-wordLow)&^qa | (lg-wordLow)&^lg | (ap-wordLow)&^ap) & wordHigh
}

// htmlStopWord adds the high bit: a lane an escaper has to decode as well as
// one it has to rewrite.
func htmlStopWord(w uint64) uint64 { return htmlEntityWord(w) | w&wordHigh }

// htmlStop is the byte form of htmlStopWord.
func htmlStop(c byte) bool {
	return c >= utf8.RuneSelf || c == '&' || c == '<' || c == '>' || c == '"' || c == '\''
}

// htmlRun returns the index of the first byte at or after i that the HTML
// escapers cannot copy through — an entity character or the start of a
// multibyte sequence — or len(value) when the rest is plain.
func htmlRun(value string, i int) int {
	for i+8 <= len(value) {
		if m := htmlStopWord(loadWord(value[i:])); m != 0 {
			return i + wordIndex(m)
		}
		i += 8
	}
	for i < len(value) && !htmlStop(value[i]) {
		i++
	}
	return i
}

// htmlEntity is the entity for one of the five characters htmlStop admits.
func htmlEntity(c byte) string {
	switch c {
	case '&':
		return "&amp;"
	case '<':
		return "&lt;"
	case '>':
		return "&gt;"
	case '"':
		return "&#34;"
	default:
		return "&#39;"
	}
}

// multibyteRun walks the run of bytes with the high bit set that starts at i.
// It returns the index of the first byte that is not part of a valid encoding
// and true, or the index where the run ends and false. A run of valid runes
// stays here rather than returning to the word scan at every rune, since each
// of them would stop it again.
func multibyteRun(value string, i int) (int, bool) {
	for i < len(value) && value[i] >= utf8.RuneSelf {
		_, width := utf8.DecodeRuneInString(value[i:])
		if width == 1 {
			return i, true
		}
		i += width
	}
	return i, false
}

// jsonStopWord marks the lanes AppendJSONString cannot copy through: a byte
// below 0x20, one of the five punctuation bytes the JSON escaper rewrites, or
// a byte with the high bit set. It is jsonbind's swarEscapeStop, with the
// backslash in place of the apostrophe.
func jsonStopWord(w uint64) uint64 {
	qa := (w | wordLow*0x04) ^ (wordLow * '&')
	lg := (w | wordLow*0x02) ^ (wordLow * '>')
	bs := w ^ (wordLow * '\\')
	return ((w-wordLow*0x20)&^w | (qa-wordLow)&^qa | (lg-wordLow)&^lg | (bs-wordLow)&^bs | w) & wordHigh
}

// jsonStop is the byte form of jsonStopWord.
func jsonStop(c byte) bool {
	return c < 0x20 || c >= utf8.RuneSelf || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&'
}

// jsonRun is htmlRun for the JSON escaper, over either member of its type set.
func jsonRun[T ~string | ~[]byte](value T, i int) int {
	for i+8 <= len(value) {
		if m := jsonStopWord(loadWord(value[i:])); m != 0 {
			return i + wordIndex(m)
		}
		i += 8
	}
	for i < len(value) && !jsonStop(value[i]) {
		i++
	}
	return i
}
