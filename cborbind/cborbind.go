// Package cborbind generates CBOR codecs for the types its entry points name.
//
// There is no declaration to write and no Codec to ask for: calling an entry
// point is the ask. The generator reads the call, emits the codec, and puts
// the matching method on the type, so the second build compiles and every
// build after that is an ordinary method call.
//
//	buf = cborbind.AppendCBORInArrayTo(buf[:0], input)
//	input, err := cborbind.DecodeCBORInArrayFrom[PlayerInput](buf)
//
// # The name says which container
//
// A struct encodes either as an array or as a map, and the two are different
// contracts rather than one contract with an option:
//
//   - Array is positional. Member names are not on the wire, so it is the
//     smaller of the two, and adding a field changes the length -- both ends
//     have to be rebuilt together.
//   - Map is keyed. A decoder skips a key it does not know, so an old build
//     reads a newer message and the two ends can ship separately.
//
// A type may have both, since the two methods do not collide. One that has
// exactly one also gets the driver's AppendCBORTo and DecodeCBORFrom
// delegating to it, so any consumer holding a cbor.Appender reaches it.
//
// # Nothing else is configured
//
// There is no profile. What a codec can encode is a property of the emitter,
// and what it does encode is a property of the struct.
//
// # This package imports nothing
//
// The interfaces below are spelled in terms of byte slices, so the driver is
// named only by the generated code that calls it. A package importing
// cborbind and nothing else links no CBOR implementation at all.
package cborbind

// ArrayAppender is the array-shaped encoder the generator emits. The method
// appends exactly one CBOR array and returns the extended slice.
type ArrayAppender interface {
	AppendCBORInArrayTo(dst []byte) []byte
}

// MapAppender is the map-shaped encoder the generator emits.
type MapAppender interface {
	AppendCBORInMapTo(dst []byte) []byte
}

// ArrayDecodable is the array-shaped decoder the generator emits. The receiver
// is a pointer, and data holds one array and nothing after it.
type ArrayDecodable interface {
	DecodeCBORInArrayFrom(data []byte) error
}

// MapDecodable is the map-shaped decoder the generator emits.
type MapDecodable interface {
	DecodeCBORInMapFrom(data []byte) error
}

// AppendCBORInArrayTo appends v as a CBOR array and returns the extended
// slice. Calling it is what asks the generator for T's array codec.
//
// The type parameter is constrained, so a T with no generated codec is a
// compile error rather than a run-time surprise, and the call allocates
// nothing: no interface value is ever materialised.
func AppendCBORInArrayTo[T ArrayAppender](dst []byte, v T) []byte {
	return v.AppendCBORInArrayTo(dst)
}

// AppendCBORInMapTo appends v as a CBOR map and returns the extended slice.
func AppendCBORInMapTo[T MapAppender](dst []byte, v T) []byte {
	return v.AppendCBORInMapTo(dst)
}

// DecodeCBORInArrayFrom decodes one CBOR array into a T. PT is inferred from
// T, so a call site writes one type argument:
//
//	v, err := cborbind.DecodeCBORInArrayFrom[PlayerInput](data)
func DecodeCBORInArrayFrom[T any, PT interface {
	*T
	ArrayDecodable
}](data []byte) (T, error) {
	var out T
	err := PT(&out).DecodeCBORInArrayFrom(data)
	return out, err
}

// DecodeCBORInMapFrom decodes one CBOR map into a T.
func DecodeCBORInMapFrom[T any, PT interface {
	*T
	MapDecodable
}](data []byte) (T, error) {
	var out T
	err := PT(&out).DecodeCBORInMapFrom(data)
	return out, err
}

// ErrShape is returned by a generated decoder when the value it is handed is
// not the container its shape expects: an array of a different length for the
// array codec, or anything but a map for the map codec.
//
// It is a sentinel rather than a formatted message because a decoder runs per
// message and the caller already knows which type it asked to decode. The
// driver's own errors, which carry an offset and a container path, pass
// through unwrapped for everything below the container itself.
var ErrShape = errShape{}

type errShape struct{}

func (errShape) Error() string {
	return "cborbind: the value is not the CBOR container this codec decodes"
}
