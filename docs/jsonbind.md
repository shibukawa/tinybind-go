# jsonbind User Guide

`jsonbind` converts Go structs to JSON documents and back without touching `net/http`. The entire API is `io.Reader` and `io.Writer`, which is what makes it usable in a CLI, a file loader, a message-queue consumer, or a WASM build where an HTTP dependency would be dead weight.

## What is automated

- Decoding JSON objects into typed structs
- Encoding typed structs as JSON
- Mapping nested structs, slices, and maps
- Reporting field-specific JSON type errors
- Enforcing a JSON document size limit
- Generating only the decoder or encoder actually used by `DecodeJSON[T]` and `EncodeJSON[T]`

Choosing an HTTP status and setting headers stay outside that boundary. Those belong to [httpbind](httpbind.md), which handles request and response concerns on top of the same codecs.

## What you provide

1. Go structs representing JSON documents
2. Concrete calls to `jsonbind.DecodeJSON[T]` or `EncodeJSON[T]`
3. A code-generation command
4. The `io.Reader` or `io.Writer`

## Setup and generation

```go
package document

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

```bash
go generate ./...
```

The generator inspects generic type arguments, so the output tracks how you actually call it. A type used only with decode gets only a decoder; a type used only with encode gets only an encoder. A codec you never call is never generated.

## Basic example

```go
package document

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

type User struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	Active bool     `json:"active"`
	Tags   []string `json:"tags"`
}

func decodeExample() error {
	in := strings.NewReader(`{
  "id": 1,
  "name": "Ada",
  "active": true,
  "tags": ["admin", "author"]
}`)

	user, err := jsonbind.DecodeJSON[User](in)
	if err != nil {
		return err
	}
	fmt.Println(user.Name)
	return nil
}

func encodeExample(user User) (string, error) {
	var out bytes.Buffer
	if err := jsonbind.EncodeJSON(&out, user); err != nil {
		return "", err
	}
	return out.String(), nil
}
```

## Supported models

The commonly supported combinations are:

- `string`
- `int`
- `int64`
- `bool`
- `float64`
- Slices of those scalar types
- Nested structs
- Slices of structs
- Scalar maps such as `map[string]string`
- `map[string]Struct`

```go
type Address struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

type Profile struct {
	Name      string             `json:"name"`
	Address   Address            `json:"address"`
	History   []Address          `json:"history"`
	Labels    map[string]string  `json:"labels"`
	AddressBy map[string]Address `json:"addressBy"`
}

func use(r io.Reader, w io.Writer) error {
	profile, err := jsonbind.DecodeJSON[Profile](r)
	if err != nil {
		return err
	}
	return jsonbind.EncodeJSON(w, profile)
}
```

Without an explicit wire name, a field becomes lower camel case. `DecodeJSON` ignores fields tagged for the HTTP-only `query`, `path`, `header`, and `cookie` sources; `EncodeJSON` does not make that distinction and emits struct fields as it finds them. A JSON-only model is therefore clearest when it carries nothing but standard `json` names.

One habit carried over from `encoding/json` needs unlearning here. The generated codec reads only the name portion of a `json` tag: `omitempty` has no effect, and `json:"-"` excludes nothing. Design models on the assumption that every declared field is written out.

## How the wire bytes differ from encoding/json

The codec reads a document in a single forward pass and writes one by appending
to a buffer, so it never builds an intermediate map and never reflects over your
structs. Three consequences are worth knowing before you diff output against
`encoding/json`:

- **Members come out in struct field order**, not sorted by name. Map-typed
  fields and `payload:"*"` rest maps are still sorted, because there is no
  declaration order to follow. Everything else follows the struct.
- **Field names match exactly.** `encoding/json` falls back to a
  case-insensitive match, so it binds `{"userId": …}` to a field named `userid`.
  This codec does not, which is also the direction `encoding/json/v2` took.
- **A duplicate name is decoded, not discarded.** `encoding/json` keeps the last
  occurrence and never looks at the earlier ones, so a wrongly typed duplicate
  passes silently. Here every occurrence is decoded as it arrives, and a bad one
  reports a field error.

String escaping, number formatting, and the `null` handling for absent slices
and maps all match `encoding/json` byte for byte, including the HTML escaping of
`<`, `>` and `&` that makes output safe to embed in a page. The one exception is
invalid UTF-8, which is written as the `\ufffd` escape: that is what the default
encoder produces, while `encoding/json` under `GOEXPERIMENT=jsonv2` writes the
replacement character raw. Both decode to the same string.

## Retaining unknown fields

Use `payload:"*"` to collect properties that were not explicitly declared:

```go
type Envelope struct {
	Kind  string         `json:"kind" payload:"kind"`
	Extra map[string]any `payload:"*"`
}
```

Use `json.RawMessage` when the values must remain undecoded:

```go
type RawEnvelope struct {
	Kind  string                     `json:"kind" payload:"kind"`
	Extra map[string]json.RawMessage `payload:"*"`
}
```

`Extra` contains only properties not represented by an explicitly declared document field.

## Read limits

The default JSON document limit is 1 MiB.

Change the process-wide limit during startup:

```go
func init() {
	jsonbind.SetMaxJSONBodyBytes(4 << 20) // 4 MiB
}
```

Override it for one call:

```go
doc, err := jsonbind.DecodeJSONLimit[Document](reader, 64<<10) // 64 KiB
```

A non-positive `DecodeJSONLimit` value falls back to the process-wide limit.

## Error handling

`jsonbind` errors are transport-neutral and do not imply an HTTP status. Each failure carries a code, and a field-specific failure also names the field that caused it:

```go
doc, err := jsonbind.DecodeJSON[Document](reader)
if err != nil {
	if jsonErr, ok := jsonbind.AsError(err); ok {
		log.Printf("code=%s field=%s message=%s",
			jsonErr.Code,
			jsonErr.Field,
			jsonErr.Message,
		)
	}
	return err
}
```

Common error codes:

| Code | Meaning |
| --- | --- |
| `json_parse` | Invalid JSON syntax, object/array shape, or value type |
| `json_field` | Invalid value for a specific field |
| `payload_too_large` | The document exceeded the configured limit |
| `body_read` | Reading from the reader failed |
| `internal` | A caller error such as a nil writer |

The status decision is deferred, not skipped. When JSON decoding happens through `httpbind.Bind`, these same errors become HTTP validation, bad-request, or payload-too-large errors.

## Reading and writing files

```go
func load(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	return jsonbind.DecodeJSON[Config](f)
}

func save(path string, value Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jsonbind.EncodeJSON(f, value)
}
```

## Keeping generation HTTP-free

The import path decides the dependency. In a JSON-only package, call `jsonbind.DecodeJSON` / `EncodeJSON` directly instead of going through the root HTTP package; the generated output then references `jsonbind` alone and never pulls in `net/http`. For TinyGo and WASM builds, where every transitive dependency costs binary size, that separation is worth enforcing deliberately.

## Missing generated codecs

Generation runs without complaint, and then runtime reports that no generated decoder or encoder exists. The usual cause is a type that reaches `DecodeJSON` only through a generic wrapper, because the generator then sees a type parameter rather than a concrete type. Give it a concrete call inside the analyzed package:

```go
func DecodeUser(r io.Reader) (User, error) {
	return jsonbind.DecodeJSON[User](r)
}
```

If the error survives that, check two things: that the concrete call lives in the package the generator analyzed, and that the generated file is part of the build.
