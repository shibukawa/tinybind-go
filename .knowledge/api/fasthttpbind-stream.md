---
id: api:fasthttpbind-stream
type: api
title: fasthttpbind Streaming
---
Typed incremental response stream over SetBodyStreamWriter, sharing one callback shape with httpbind.

```yaml
status: proposed 2026-08-08
entry_point: api:write-stream
shape_settled_by: decision:stream-callback-shape
resolution_2026_08_08: the shape inversion is resolved by moving httpbind to the callback too, rather than by giving fasthttp a second stream API
formats: sse | ndjson | json-array, chosen by rule:stream-content-negotiation, unchanged
implementation:
  - negotiate and set response headers on ctx while the handler still owns it
  - install the callback through ctx.SetBodyStreamWriter
  - build Stream[T] over the *bufio.Writer the callback receives
  - flush through that writer's Flush error method after each event
  - close the stream when the callback returns, writing the json-array trailer
handler_returns_first:
  fact: the callback runs after the handler returns, which is the property fasthttp exists to have
  cost: an error raised inside the callback cannot reach handler code, which is why api:write-stream returns nothing on either transport
lifetime: rule:fasthttpbind-requestctx-lifetime forbids reading ctx from inside the callback; everything the stream needs is captured before the handler returns
htmlbind_note: the runtime Flush duck-types Flush and Flush error, so a *bufio.Writer satisfies it and the HTML render path needs no port
htmlupdate_note: the live boundary recordWriter holds an http.Flusher field, so that path is transport-bound in the runtime and is deferred by requirement:fasthttpbind-parity-scope
related:
  - concept:streaming
  - api:stream-write
  - rule:stream-termination-marker
  - decision:fasthttpbind-runtime-package
  - requirement:fasthttpbind-parity-scope
```
