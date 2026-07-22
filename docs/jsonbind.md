# jsonbind User Guide

`jsonbind` converts between Go structs and JSON documents without depending on HTTP. Its API uses only `io.Reader` and `io.Writer`, making it suitable for CLIs, files, message queues, and WASM applications.

## What is automated

- Decoding JSON objects into typed structs
- Encoding typed structs as JSON
- Mapping nested structs, slices, and maps
- Reporting field-specific JSON type errors
- Enforcing a JSON document size limit
- Generating only the decoder or encoder actually used by `DecodeJSON[T]` and `EncodeJSON[T]`

`jsonbind` does not select HTTP statuses or set HTTP headers. Use [httpbind](httpbind.md) for HTTP request and response handling.

## What you provide

1. Go structs representing JSON documents
2. Concrete calls to `jsonbind.DecodeJSON[T]` or `EncodeJSON[T]`
3. A code-generation command
4. The `io.Reader` or `io.Writer`

## Setup and generation

```go
package document

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir .
```

```bash
go generate ./...
```

The generator inspects generic type arguments. A type used only with decode gets only a decoder; a type used only with encode gets only an encoder.

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

Without an explicit wire name, a field becomes lower camel case. `DecodeJSON` ignores fields tagged for the HTTP-only `query`, `path`, `header`, or `cookie` sources. `EncodeJSON`, however, emits struct fields, so a JSON-only model is clearest when it uses only standard `json` names.

The generated codec currently uses only the name portion of a `json` tag. It does not apply `omitempty` or exclusion via `json:"-"`; design models assuming that fields are emitted.

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

A non-positive `DecodeJSONLimit` value uses the process-wide limit.

## Error handling

`jsonbind` errors are transport-neutral and do not imply an HTTP status.

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

When JSON decoding happens through `httpbind.Bind`, these errors are converted to HTTP validation, bad-request, or payload-too-large errors.

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

In a JSON-only package, import `jsonbind` directly and call `DecodeJSON` / `EncodeJSON` instead of the root HTTP package. The generated output then depends only on `jsonbind`, not `net/http`. This separation is particularly useful for TinyGo and WASM builds.

## Missing generated codecs

The generator may not find a concrete type when it is passed dynamically through a generic wrapper. Put a concrete call in the analyzed package:

```go
func DecodeUser(r io.Reader) (User, error) {
	return jsonbind.DecodeJSON[User](r)
}
```

If runtime reports that no generated decoder or encoder exists, verify that the concrete call is in the same package and that the generated file is included in the build.
