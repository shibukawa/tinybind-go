// Package tomlbind decodes one TOML document into a struct through generated
// code, and imports nothing of configbind.
//
// configbind answers "load my service's settings": it layers a TOML file,
// environment variables and command-line options onto one struct, and links
// the parsers for all three to do it. This package is the TOML layer on its
// own, for a program that reads a file and wants nothing else.
//
//	settings, err := tomlbind.DecodeTOML[Settings](file)
//
// Calling an entry point is the ask. The generator reads the call, emits the
// decoder, and puts the DecodeTOMLFrom method on the type, so the second build
// compiles and every build after that is an ordinary method call. A type with
// no generated method does not satisfy the constraint, and the call site fails
// to build rather than failing at the first read.
//
// # Declaring a decoder
//
// Generation is per package, and the method lands on the type, so a type that
// is decoded from another package declares its decoder beside itself:
//
//	var _ = tomlbind.GenerateDecoder[Settings]()
//
// # What a document may hold
//
// The parser is minitoml, which reads the configuration subset configbind
// reads: tables, dotted bare keys, string, bool, integer and float scalars,
// arrays of scalars, and arrays of tables. Quoted keys, inline tables and
// nested arrays are refused. The struct's key and default tags are the ones
// configbind reads, so one struct serves both.
//
// # This package imports io and nothing else
//
// The interface below is spelled in bytes, so minitoml is named only by the
// generated code that calls it.
package tomlbind

import "io"

// Decodable is the decoder the generator emits. The receiver is a pointer,
// data holds one whole TOML document, and the method replaces the value.
type Decodable interface {
	DecodeTOMLFrom(data []byte) error
}

// Declaration is what GenerateDecoder returns. It carries nothing: the value
// exists only so the annotation can be written as a package-level declaration,
// which is where generation reads it.
type Declaration struct{}

// GenerateDecoder asks for T's decoder without a call site to derive it from.
// Write it at package level, beside the type:
//
//	var _ = tomlbind.GenerateDecoder[Settings]()
//
// The call runs at init and does nothing. The declaration is the point.
func GenerateDecoder[T any]() Declaration { return Declaration{} }

// DecodeTOML reads r to its end and decodes the document into a T. PT is
// inferred from T, so a call site writes one type argument:
//
//	settings, err := tomlbind.DecodeTOML[Settings](file)
//
// A document is read once at startup, so it is read whole rather than
// streamed; a size limit is the caller's io.LimitReader.
func DecodeTOML[T any, PT interface {
	*T
	Decodable
}](r io.Reader) (T, error) {
	var out T
	data, err := io.ReadAll(r)
	if err != nil {
		return out, err
	}
	return DecodeTOMLBytes[T, PT](data)
}

// DecodeTOMLBytes decodes one TOML document held in memory into a T.
func DecodeTOMLBytes[T any, PT interface {
	*T
	Decodable
}](data []byte) (T, error) {
	var out T
	if err := PT(&out).DecodeTOMLFrom(data); err != nil {
		return out, err
	}
	return out, nil
}
