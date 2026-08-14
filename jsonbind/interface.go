package jsonbind

// The two interfaces below are how a type carries its own JSON codec.
//
// Everything else in this package reaches a codec by having planned the type:
// the generator analyzes a package, emits appendUserJSON and decodeUserBytes,
// and registers them. That makes encodability a property of having been
// analyzed, which no type outside the analyzed package can acquire. A type
// satisfying one of these interfaces carries its encoder with it instead, so a
// value from a package this module never saw is still written and read without
// reflection.
//
// A type assertion is dispatch, not field walking, so nothing here reintroduces
// reflection and a TinyGo target is unaffected.
//
// The method names are deliberately not encoding/json's. The standard
// interfaces are not recognized yet — that waits until encoding/json/v2 ships
// without the experiment — and using distinct names means a type cannot satisfy
// one of these by accident while meaning the other.

// Appender is a type that encodes itself as JSON by appending to dst and
// returning the extended slice.
//
// This is the method form of what the generator already emits, so a generated
// codec satisfies it by delegation and a hand-written one costs the same as the
// generated body would have.
//
// There is no error result. The append path has none anywhere below this point,
// and every value that reaches it is one the caller already holds, so an
// implementation that cannot produce a document for its own value has no state
// worth reporting. An implementation must append valid JSON for every value of
// its type.
type Appender interface {
	AppendJSONTo(dst []byte) []byte
}

// Decoder is a type that decodes one complete JSON document into itself.
//
// data holds exactly one JSON value. The implementation fills the receiver, so
// the method belongs on the pointer and *T rather than T is what satisfies this.
//
// Unlike Appender, this does not compose to any depth for free: a nested field
// is decoded by walking a Parser, and a method taking a complete slice joins
// that walk only by being handed the sub-document, which costs scanning that
// region twice. Generated decoders take that path for a field whose type
// satisfies this and no other field, so a document holding none pays nothing.
type Decoder interface {
	DecodeJSONFrom(data []byte) error
}
