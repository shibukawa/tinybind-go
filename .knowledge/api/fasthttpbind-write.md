---
id: api:fasthttpbind-write
type: api
title: fasthttpbind Write, WriteStatus, and WriteError
---
Response writers that serialize a typed value into a fasthttp RequestCtx, producing bytes identical to their httpbind counterparts.

```yaml
signatures:
  - "func Write[T any](ctx *fasthttp.RequestCtx, value T) error"
  - "func WriteStatus[T any](ctx *fasthttp.RequestCtx, status int, value T) error"
  - "func WriteError(ctx *fasthttp.RequestCtx, err error)"
mirrors:
  - api:write
  - api:write-status
  - api:write-error
parameter_note: no separate request parameter, because RequestCtx carries both halves; httpbind passes r only for negotiation and already discards it in WriteStatus and WriteError
behavior:
  - Write resolves the generated writer for T from the writer registry
  - WriteStatus serializes through jsonbind.EncodeJSON, which reads the encoder registry; a registered writer alone does not serve it
  - append the document into a pooled buffer, then hand the bytes to ctx
  - status 200 by default; WriteStatus writes no body for 204
  - WriteError emits policy:problem-details as application/problem+json, hiding internal causes on 5xx
registry_split: the same two registries api:write-status describes, and the same trap; rule:call-operation-usage is the mapping both transports share
byte_identical_to_httpbind: required; the same model must produce the same body and the same Content-Type on either transport
no_default_content_type: |
  fasthttp fills in a Content-Type for a response that names none, and net/http
  sends none for a bodyless 204, so WriteStatus declines the default there. It is
  the only path here that sets a status without naming a content type: the other
  writers set theirs first, and api:fasthttpbind-stream sets its own from the
  negotiated format.
buffer_note: generated writers already build into a jsonbind pooled buffer and hand over bytes, so the fasthttp writer needs no second copy
related:
  - concept:response-binding
  - policy:problem-details
  - concept:error-helpers
  - decision:fasthttpbind-runtime-package
  - concept:fasthttp-handler
  - rule:call-operation-usage
  - requirement:fasthttpbind-parity-scope
```
