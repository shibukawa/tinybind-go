---
id: concept:streaming
type: concept
title: Typed Streaming
---
Streaming uses a writable Stream[T] the runtime hands to a callback; handlers call Write repeatedly for each event.

```yaml
api: api:new-stream
type: "httpbind.Stream[T]"
not:
  - WriteNDJSON batch helper
  - WriteSSE batch helper
handler_shape: |
  httpbind.WriteStream(w, r, func(s *httpbind.Stream[ChatEvent]) error { ... })
  if err != nil { ... }
  defer stream.Close()
  _ = stream.Write(ChatEvent{Type: "delta", Delta: "hi"})
  _ = stream.Write(ChatEvent{Type: "done"})
service_note: |
  The handler-side API is WriteStream + Write; the held NewStream entry was removed 2026-08-10, see api:new-stream.
  Returning Stream[T] from a pure service function remains a future convenience
  if generation wires it to the same runtime writer.
formats:
  - name: sse
    media_type: text/event-stream
    note: Server-Sent Events; data: <json> frames
  - name: ndjson
    media_type: application/x-ndjson
    aliases: [JSONL, application/jsonl, application/ndjson]
    note: one JSON object per line; NOT a single JSON array document
  - name: json-array
    media_type: application/json
    framing: "[obj1,obj2,...]"
    note: single JSON array document; Close writes trailing bracket
    not: JSONL
selection: rule:stream-content-negotiation
openapi: rule:openapi-streaming-content
bidirectional_sibling:
  what: concept:typed-websocket, which reads as well as writes
  shares: the callback entry, the runtime-owned close, and the post-commit error sink
  differs: two type arguments instead of one, one framing instead of three negotiated, and a read discipline a write-only stream never needed
related:
  - api:new-stream
  - api:stream-write
  - rule:stream-content-negotiation
  - concept:net-http-handler
  - concept:typed-websocket
  - system:tinybind
```
