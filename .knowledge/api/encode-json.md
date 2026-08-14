---
id: api:encode-json
type: api
title: jsonbind.EncodeJSON
---
Generic compact JSON encoder of typed T to io.Writer; independent of http.ResponseWriter.

```yaml
signature: "func EncodeJSON[T any](w io.Writer, v T) error"
example: "err := jsonbind.EncodeJSON(w, output)"
pair: api:decode-json
behavior:
  - encode v as compact JSON to w
  - no pretty-print / indent API
  - not HTTP: no Status Code, no Content-Type header
  - prefer generated codec when T is registered (decision:reflection-free)
name_note: |
  The untyped helper WriteJSON(http.ResponseWriter, status int, v any) stayed in
  httpbind until 2026-08-14, when the never-called helper was removed from both
  runtimes; public standalone API is jsonbind.EncodeJSON.
differs_from:
  api:write: Write sets HTTP status/headers and uses Accept/stream paths
  api:write-error: problem+json for errors
related:
  - concept:standalone-json-codec
  - api:decode-json
  - api:write
  - concept:code-generation
  - concept:response-binding
  - system:tinybind
```
