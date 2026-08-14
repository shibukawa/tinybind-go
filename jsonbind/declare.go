package jsonbind

// Declaration is what an annotation below returns. It carries nothing: the
// value exists only so the annotation can be written as a package-level
// declaration, which is where generation reads it.
type Declaration struct{}

// The three annotations below ask for a codec that no call site in the package
// would have asked for.
//
// Generation is otherwise driven entirely by usage: a codec is emitted for a
// type some configured generic call names, and a type nothing calls gets
// nothing. That is right for a type used where the analysis can see it, and
// wrong for two cases — a type crossing a boundary with no generic call at the
// crossing, and a type whose only call is one the generator itself writes.
// Before these annotations the only way to reach such a codec was to write a
// call site meant to be found rather than to be run.
//
// An annotation also publishes the codec as the methods of [Appender] and
// [Decoder], which a discovered call site does not. Declaring a codec for a
// type this package's own calls do not reach is already saying the type is used
// from somewhere the analysis cannot see, and the methods are what make it
// reachable from there. A project writing no annotation emits exactly the bytes
// it emits today.
//
// Write one at package level, beside the type:
//
//	var _ = jsonbind.GenerateCodec[User]()
//
// The call runs at init and does nothing. The declaration is the point.

// GenerateCodec asks for T's encoder and decoder, and for both methods.
func GenerateCodec[T any]() Declaration { return Declaration{} }

// GenerateEncoder asks for T's encoder and for [Appender] alone. Use it for a
// type only ever written, so a decoder is not carried into the binary with it.
func GenerateEncoder[T any]() Declaration { return Declaration{} }

// GenerateDecoder asks for T's decoder and for [Decoder] alone. Use it for a
// type only ever read.
func GenerateDecoder[T any]() Declaration { return Declaration{} }
