# tomlbind guide

`tomlbind` decodes one TOML document into a struct. It is the TOML layer of
[configbind](configbind.md) on its own: no environment variables, no
command-line options, no configuration directory search, and none of the
parsers those need linked into the binary.

```go
settings, err := tomlbind.DecodeTOML[Settings](file)
```

The generator reads that call, writes the decoder, and puts a `DecodeTOMLFrom`
method on the type. The first build after adding a call does not compile — the
method does not exist yet — so run the generator and build again. That is the
same bootstrap every other generated mode has, and it is the trade that makes a
missing decoder a compile error instead of a run-time surprise.

- [When to use which](#when-to-use-which)
- [Entry points](#entry-points)
- [Declaring a decoder](#declaring-a-decoder)
- [What gets generated](#what-gets-generated)
- [Struct tags](#struct-tags)
- [Field types](#field-types)
- [TOML files](#toml-files)
- [Errors](#errors)
- [Generation](#generation)

## When to use which

| | tomlbind | configbind |
| --- | --- | --- |
| reads | one TOML document from an `io.Reader` or a byte slice | a file found by path or config dir, then env, then CLI |
| layering | none | `default < TOML < env < CLI`, fixed |
| `${NAME}` in a string | left as written | expanded from the environment |
| provenance, scaffolds, subcommands | no | yes |
| links | `minitoml` only | `minitoml`, `cliparser`, `configpath`, `configdir`, `go-envparse` |

The struct is the same on both sides. `tomlbind` reads the `key` and `default`
tags configbind reads, and a document configbind would load binds the same way,
so a struct can start under tomlbind and move to configbind when it needs the
other sources.

## Entry points

```go
tomlbind.DecodeTOML[T](r io.Reader) (T, error)
tomlbind.DecodeTOMLBytes[T](data []byte) (T, error)
```

`DecodeTOML` reads `r` to its end. A document is read once at startup, so there
is no streaming parser; a size limit is the caller's `io.LimitReader`. Both
return a fresh `T`: a decode replaces the value rather than layering onto one.

The type argument is the only thing a call site writes. The entry points
constrain on the generated method through a pointer, so a type with no decoder
does not satisfy the constraint and the call fails to build.

## Declaring a decoder

Generation is per package and the method lands on the type, so a type that is
decoded from another package declares its decoder beside itself:

```go
package config

var _ = tomlbind.GenerateDecoder[Settings]()
```

The call runs at init and does nothing. The declaration is the point: the
generator finds it where it finds any other call, and the method is written in
the package that owns the type.

A declaration naming a type from another package is a generation error with the
remedy. An ordinary `DecodeTOML[other.T]` call naming one is left to the
compiler, which reports the unsatisfied constraint at the call site.

## What gets generated

For `DecodeTOML[Settings](r)` the generator writes, into `tomlbind_gen.go`:

```go
func decodeSettingsTOML(dst *Settings, doc minitoml.Document) error
func (v *Settings) DecodeTOMLFrom(data []byte) error
```

The first is the apply function `minitoml/codegen` emits for configbind, with
the document root as its prefix; nested structs and array-of-table elements are
inlined into it, so only the types you name get a method. The second parses the
bytes with `minitoml`, resets `*v`, and applies.

Nothing is registered and no `init` is emitted. The generated file imports
`fmt`, `minitoml`, and `time` when a duration field needs it.

## Struct tags

| Tag | Meaning |
| --- | --- |
| `key:"name"` | The TOML key, when the snake-cased field name is not it |
| `default:"value"` | The value taken when the key is absent, written as it would be in TOML |

Everything else configbind reads — `opt`, `env`, `help`, `enum`, `falsy`,
`dependon`, `summary`, `secret` — is about the other sources and the report,
and is ignored here. A struct carrying them still decodes.

A field with no `default` and no key in the document keeps the zero value.

## Field types

`string`, `bool`, every integer width (`int` through `uint64`), `float64` and
`float32`, `time.Duration` from a string such as `"250ms"`, `[]string`, a
nested struct as a table, and a slice of a same-package struct as an array of
tables.

A TOML integer widens into a float field. A TOML float in an integer field is
an error, as is a value outside the field's width. An alias of a supported type
is that type.

`float64` and `float32` are the one thing tomlbind accepts that configbind does
not: configbind has no float form for its env and CLI layers, so a struct
shared with it keeps floats out.

## TOML files

The parser is `minitoml`, which reads the subset configbind reads:

- tables, nested tables, and dotted bare keys
- string, bool, integer and float scalars
- arrays of scalars
- arrays of tables, `[[name]]`, one element per header
- comments

Quoted keys, inline tables, nested arrays, and date-times are refused. A key
defined twice is an error. See [configbind's TOML files](configbind.md#toml-files)
for the full shape rules; they are the same rules.

## Errors

A parse error names the line and column. An apply error names the full key
path, `tomlbind: listeners.port: ...`, and says what was wrong with the value.
Both are ordinary errors: decoding runs once per document, so there is no
sentinel to compare against.

## Generation

```bash
tinybind-gen generate -dir .
```

A package calling no entry point writes no `tomlbind_gen.go`. The pass is
`decode-toml` under `-disable`, and `tomlbind` itself imports `io` and nothing
else: the interface is spelled in bytes, and only the generated code names
`minitoml`.

The generated decoder builds and runs under TinyGo for `wasm` and `wasip1`.
