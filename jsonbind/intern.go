package jsonbind

import (
	"encoding/binary"
	"sync"
	"unicode/utf8"
)

// A decoded document is mostly strings, and many of them recur: the language
// codes, colour names and identifiers a feed repeats in every record, and the
// member names of every object an any field holds. The cache below hands a
// recurring value back without allocating it again.
//
// It is a direct-mapped table of short strings, hashed from their first and
// last bytes and their length, so a lookup costs two loads and a compare and
// a miss costs the same plus the allocation the string needed anyway. Strings
// longer than internMax are not cached: long values are text, which seldom
// repeats, and the compare on a miss would cost more than the hash saves.
//
// A table lives in a pool and follows a Parser from first use until Release,
// so it carries what one document taught into the next one decoded on the
// same table. A document that shares nothing with its predecessor pays the
// hash and the compare and gains nothing, which is the price the mixed case
// is bought with.

const (
	internMin   = 2 // one-byte strings are interned by the runtime already
	internMax   = 64
	internSlots = 256
)

type stringCache struct {
	s [internSlots]string
}

var stringCachePool = sync.Pool{New: func() any { return new(stringCache) }}

// internHash folds the first and last bytes and the length of b into a slot.
// b is at least internMin bytes. It is kept small enough to inline into
// String, where it runs for every cacheable value.
func internHash(b []byte) uint32 {
	var h uint64
	if len(b) >= 8 {
		h = binary.LittleEndian.Uint64(b) ^ binary.LittleEndian.Uint64(b[len(b)-8:])*0x9e3779b97f4a7c15
	} else {
		h = (uint64(b[0]) | uint64(b[len(b)-1])<<8 | uint64(b[len(b)/2])<<16) * 0x9e3779b97f4a7c15
	}
	h ^= uint64(len(b)) * 0xff51afd7ed558ccd
	return uint32(h>>32) % internSlots
}

// intern returns span as a string, from the cache when an equal one is held.
// A string stored here is valid UTF-8, so a hit skips the validation a
// non-ASCII span otherwise pays; a span holding invalid UTF-8 never compares
// equal to the coerced string it was stored as, and is coerced again.
func (p *Parser) intern(span []byte, nonASCII bool) string {
	if len(span) < internMin || len(span) > internMax {
		return p.makeString(span, nonASCII)
	}
	if p.cache == nil {
		p.cache = stringCachePool.Get().(*stringCache)
	}
	i := internHash(span)
	if s := p.cache.s[i]; s == string(span) {
		return s
	}
	s := p.makeString(span, nonASCII)
	p.cache.s[i] = s
	return s
}

// makeString allocates span as a string, replacing invalid UTF-8 the way
// encoding/json does.
func (p *Parser) makeString(span []byte, nonASCII bool) string {
	if !nonASCII || utf8.Valid(span) {
		return string(span)
	}
	p.scratch = coerceUTF8(append(p.scratch[:0], span...))
	return string(p.scratch)
}

// Intern returns b as a string, sharing an earlier allocation when the cache
// holds an equal one. Generated decoders use it for map keys, which recur
// across the objects of one document more than any value does. b is taken as
// it is, with no validation.
func (p *Parser) Intern(b []byte) string {
	if len(b) < internMin || len(b) > internMax {
		return string(b)
	}
	if p.cache == nil {
		p.cache = stringCachePool.Get().(*stringCache)
	}
	i := internHash(b)
	if s := p.cache.s[i]; s == string(b) {
		return s
	}
	s := string(b)
	p.cache.s[i] = s
	return s
}

// Release hands the parser's string cache back to the pool, so the next
// decode on any parser starts from what this one learned. A parser that is
// never released keeps its table until it is collected, which costs the pool
// a table and nothing else.
func (p *Parser) Release() {
	if p.cache != nil {
		stringCachePool.Put(p.cache)
		p.cache = nil
	}
}
