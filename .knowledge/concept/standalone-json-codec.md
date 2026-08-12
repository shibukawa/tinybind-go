---
id: concept:standalone-json-codec
type: concept
title: Standalone Typed JSON Codec
---
Typed JSON encode/decode usable outside HTTP Bind/Write: same application types, no *http.Request or ResponseWriter required.

```yaml
intent: JSON I/O is a first-class library capability, not only request/response mapping
public_api:
  - api:decode-json
  - api:encode-json
naming:
  style: Encode / Decode pair (json.Encoder / json.Decoder aligned)
  decode: "func DecodeJSON[T any](r io.Reader) (T, error)"
  encode: "func EncodeJSON[T any](w io.Writer, v T) error"
not:
  - pretty-print / indent
  - content negotiation
  - problem+json error envelope (use api:write-error for HTTP errors)
  - ReadJSON / WriteJSON as public standalone names
io:
  decode: io.Reader
  encode: io.Writer
encoding:
  style: compact single-line JSON (encoding/json Encoder default; no SetIndent)
  trailing_newline: allowed if Encoder adds one; callers must not rely on absence
  nil_slice_map: "[] and {} as encoding/json/v2 writes them, not encoding/json's null for both"
  json_tag:
    name: first comma-separated part; falls back to the wire name, then lower camel case
    dash: 'json:"-" drops the field from encode and decode alike; the HTTP binder still reads it by wire name'
    omitempty: encoding/json/v2 reading — omits "" [] {}, writes 0 and false, inert on numbers bools and nested objects
    omitzero: omits the Go zero value, which is what reaches 0 and false; a nested struct is zero when all its fields are
    unknown_option: a generation error, not a silently inert tag
type_path:
  preferred: reuse generated codecs / registry for types known to concept:code-generation
  goal: decision:reflection-free for registered models
  fallback: document if unregistered T is rejected vs encoding/json
relation_to_http:
  api:bind: HTTP sources (query path header cookie multipart) plus payload JSON
  api:write: HTTP headers status Accept streaming
  standalone: pure JSON document in/out only
runtime_boundary:
  package: github.com/shibukawa/tinybind-go/jsonbind
  dependency: transport-neutral leaf with no net/http or database/sql
  errors: transport-neutral jsonbind.Error; httpbind translates at api:bind
  registry: owned by jsonbind; generated JSON-only init registers here
nested: rule:nested-request-binding applies to document shape when implemented
rest: term:payload-rest is HTTP payload-only; standalone uses JSON document fields (json tags or generated field plan)
related:
  - api:decode-json
  - api:encode-json
  - api:bind
  - api:write
  - concept:code-generation
  - concept:request-binding
  - concept:response-binding
  - decision:reflection-free
  - decision:runtime-package-boundaries
  - system:tinybind
```
