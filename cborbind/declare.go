package cborbind

// Declaration is what an annotation below returns. It carries nothing: the
// value exists only so the annotation can be written as a package-level
// declaration, which is where generation reads it.
type Declaration struct{}

// The annotations below ask for a CBOR codec that no call site in the package
// would have asked for.
//
// Write one at package level, beside the type:
//
//	var _ = cborbind.GenerateWireCodec[PlayerInput]()
//
// The call runs at init and does nothing. The declaration is the point.
//
// # The profile is part of the declaration
//
// A wire codec and a world codec are different contracts, not one contract
// with an option, and the two ends of a connection must not disagree about
// which is in use. So the profile is named by the annotation rather than set
// by a generator flag, and the emitted code pins it: a wire codec will not
// read a world message.
//
// Wire is the frozen realtime format. A struct encodes as a fixed-length
// array in declaration order, with no field names and no room for an unknown
// field. It is what a message per player per tick is for.
//
// World is the evolvable one. A struct encodes as a map with deterministic
// key order, and a decoder skips a field it does not know, which is the schema
// tolerance a snapshot needs to outlive the version that wrote it.
//
// # Directions are named
//
// Emitting a decoder for a type only ever written is code size in a wasm
// client, so each profile has a codec form and two narrower ones. Disabling
// one direction through the generator's feature switches leaves the other
// standing.

// GenerateWireCodec asks for T's wire-profile encoder and decoder, and for
// both methods.
func GenerateWireCodec[T any]() Declaration { return Declaration{} }

// GenerateWireEncoder asks for T's wire-profile encoder and for AppendCBORTo
// alone. Use it for a message only ever sent.
func GenerateWireEncoder[T any]() Declaration { return Declaration{} }

// GenerateWireDecoder asks for T's wire-profile decoder and for DecodeCBORFrom
// alone. Use it for a message only ever received.
func GenerateWireDecoder[T any]() Declaration { return Declaration{} }

// GenerateWorldCodec asks for T's world-profile encoder and decoder, and for
// both methods.
func GenerateWorldCodec[T any]() Declaration { return Declaration{} }

// GenerateWorldEncoder asks for T's world-profile encoder and for AppendCBORTo
// alone.
func GenerateWorldEncoder[T any]() Declaration { return Declaration{} }

// GenerateWorldDecoder asks for T's world-profile decoder and for
// DecodeCBORFrom alone.
func GenerateWorldDecoder[T any]() Declaration { return Declaration{} }

// The delta annotations below ask for the diff and apply of
// requirement:cbor-state-delta-generation, on top of the codec.
//
// A delta is diffed from values that must also be encodable, so declaring one
// declares the codec for that profile too rather than failing on its absence.
//
// What is generated is a named delta type per struct in the reachable set, its
// own AppendCBORTo and DecodeCBORFrom, a Diff that reports what changed, and an
// Apply that puts it back. The game declares struct types and never writes the
// code that finds a difference.
//
// A collection is diffed element by element only when its element type declares
// which field identifies it:
//
//	type House struct {
//		ID    uint32 `cbor:"id,identity"`
//		Power int32
//	}
//
// Without an identity nothing distinguishes an entity that changed from one
// that was replaced, so the collection is carried whole under a single bit.
// That is legal and often right for a short fixed slice, and expensive for an
// entity list, so the run reports every collection it decided to carry whole.

// GenerateWireDelta asks for T's wire-profile delta, and for the wire codec it
// is diffed from.
func GenerateWireDelta[T any]() Declaration { return Declaration{} }

// GenerateWorldDelta asks for T's world-profile delta, and for the world codec
// it is diffed from.
func GenerateWorldDelta[T any]() Declaration { return Declaration{} }
