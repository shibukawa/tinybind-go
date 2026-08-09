---
id: api:new-stream
type: api
title: httpbind.NewStream
---
Creates a typed response stream bound to the ResponseWriter; transport format is chosen from the request.

```yaml
status: shipped; superseded by decision:stream-callback-shape, which replaces this entry point with api:write-stream
signature: "func NewStream[T any](w http.ResponseWriter, r *http.Request) (*Stream[T], error)"
example: |
  stream, err := httpbind.NewStream[ChatEvent](w, r)
  if err != nil {
      httpbind.WriteError(w, r, err)
      return
  }
  defer stream.Close()
behavior:
  - negotiate format via rule:stream-content-negotiation
  - write response headers and status once on first successful open
  - return Stream[T] for repeated api:stream-write
  - flush after each event when ResponseWriter supports Flusher
  - formats: sse | ndjson (JSONL) | json-array
errors:
  - if headers already written
  - if negotiation fails (optional; prefer default)
related:
  - concept:streaming
  - api:stream-write
  - rule:stream-content-negotiation
```
