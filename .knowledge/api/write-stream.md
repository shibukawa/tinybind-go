---
id: api:write-stream
type: api
title: WriteStream Callback Entry
---
Opens a negotiated typed stream, runs the caller's callback against it, and closes it, on either transport.

```yaml
status: proposed by decision:stream-callback-shape
signatures:
  httpbind: "func WriteStream[T any](w http.ResponseWriter, r *http.Request, fn func(*Stream[T]) error)"
  fasthttpbind: "func WriteStream[T any](ctx *fasthttp.RequestCtx, fn func(*Stream[T]) error)"
name_reason: Stream is already the type, so the entry point cannot share the name; WriteStream joins Write, WriteStatus and WriteError
example: |
  httpbind.WriteStream(w, r, func(s *httpbind.Stream[ChatEvent]) error {
      if err := s.Write(ChatEvent{Type: "delta", Delta: "hi"}); err != nil {
          return err
      }
      return s.Write(ChatEvent{Type: "done"})
  })
status_2026_08_08: implemented on both surfaces
behavior:
  - negotiate the format once, by rule:stream-content-negotiation
  - write headers and 200 before the callback runs
  - run the callback with a live Stream[T]
  - close the stream when the callback returns, whatever it returns
  - route a returned error by decision:stream-callback-shape, which as built means the shared error handler
shared_implementation:
  where: bindcore owns StreamFormat, the framing, the header table, the negotiation rule, and the error handler
  aliased: each surface declares Stream[T] as a generic alias of the one type, so the framing bytes cannot drift
  verified: parity tests compare status, Content-Type and body bytes across ndjson, sse and json-array; further tests cover the always-terminated array, the empty-array document, and the error handler receiving from both transports
transport_difference_hidden_from_callers:
  net_http: the callback runs inline, before the handler returns
  fasthttp: the callback is installed through SetBodyStreamWriter and runs after the handler returns, writing into a *bufio.Writer
  identical: the callback body, which is why no return value is offered
flush:
  net_http: the http.Flusher assertion the current implementation makes
  fasthttp: the *bufio.Writer Flush error method
  note: htmlbind Flush already duck-types both, so the HTML render path needs no branch
lifetime_note: on fasthttp the callback outlives the handler, so rule:fasthttpbind-requestctx-lifetime forbids reading ctx inside it; anything the callback needs is captured before WriteStream returns
related:
  - concept:streaming
  - api:stream-write
  - api:new-stream
  - rule:stream-termination-marker
  - policy:problem-details
```
