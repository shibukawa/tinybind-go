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
  - resolve the generated encoder for T from the writer registry
  - append the document into a pooled buffer, then hand the bytes to ctx
  - status 200 by default; WriteStatus writes no body for 204
  - WriteError emits policy:problem-details as application/problem+json, hiding internal causes on 5xx
byte_identical_to_httpbind: required; the same model must produce the same body and the same Content-Type on either transport
buffer_note: generated writers already build into a jsonbind pooled buffer and hand over bytes, so the fasthttp writer needs no second copy
related:
  - concept:response-binding
  - policy:problem-details
  - concept:error-helpers
  - decision:fasthttpbind-runtime-package
  - concept:fasthttp-handler
```
