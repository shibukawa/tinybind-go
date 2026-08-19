// Package cborbind is the declaration surface for generated CBOR codecs.
//
// It is deliberately almost empty. Everything a codec needs at run time --
// the Append family, the Reader, the two profiles, the width-enforcing reads
// and the error type -- already lives in
// github.com/shibukawa/tinygodriver/encoding/cbor, and generated code names
// that package directly. What this package adds is the annotation that asks
// for a codec, because a game hands its message types to a session framework
// which encodes them generically, so there is no generic call in the game's
// own source for usage-directed generation to find.
//
// # No interface is declared here
//
// The codec contract is the driver's: cbor.Appender with AppendCBORTo and
// cbor.Decodable with DecodeCBORFrom. This package does not restate them.
// Two spellings of one contract means a type can satisfy the wrong one and be
// silently skipped, and a silently skipped field under a fixed-shape profile
// is a running protocol with a hole in it rather than a parse failure.
//
// The generator recognizes those two interfaces structurally, so a package
// whose types carry them need not import this one.
//
// # A type carrying the methods wins
//
// A type that already has AppendCBORTo is encoded through it, even when a
// declaration in the same package also named that type. The same holds for
// DecodeCBORFrom. That is how a fixed-point value reaches the wire: declare a
// distinct type per scale, give each its own two methods, and the scale is
// carried by the type rather than by a tag the generator would have to read.
package cborbind
