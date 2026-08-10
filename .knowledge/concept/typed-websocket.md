---
id: concept:typed-websocket
type: concept
title: Typed WebSocket Sockets
---
A WebSocket endpoint is a callback holding a typed Socket[In, Out] whose Read and Write carry JSON through jsonbind, identical in source on net/http and fasthttp.

```yaml
status: implemented 2026-08-10
shape: |
  httpbind.WebSocket(w, r, func(s *httpbind.Socket[ClientMsg, ServerMsg]) error {
      for {
          in, err := s.Read()
          if err != nil {
              return err
          }
          switch in.Type {
          case "start":
              err = s.Write(ServerMsg{Type: "ready"})
          case "message":
              err = s.Write(ServerMsg{Type: "message", Text: in.Text})
          case "end":
              return nil
          }
          if err != nil {
              return err
          }
      }
  })
relation_to_streaming:
  same: a callback the runtime hands a typed object to, closed by the runtime whatever the callback returns, with post-commit failures routed to an installed sink
  different: a stream writes only, so it needed one type argument and no read discipline; a socket reads and writes, which is where every decision below diverges
  reused_wholesale: the callback-entry argument of decision:stream-callback-shape, which fasthttp forces a second time
  not_reused: rule:stream-content-negotiation, because a socket has one framing and nothing to negotiate
layers:
  transport_free_core: bindcore owns Socket[In, Out], the read and write discipline, the lifecycle, and the option defaults
  per_transport_shell: httpbind and fasthttpbind each own the upgrade and the origin check, under one declaration name
  driver: system:tinygodriver-websocket
what_the_library_owns:
  - typed Read and Write over jsonbind, per decision:websocket-message-typing
  - write serialization, so a push goroutine is safe, per decision:websocket-loop-and-write-serialization
  - the connection lifecycle: read limit, idle deadline, ping cadence, graceful close, per decision:websocket-lifecycle-ownership
  - a handshake refusal shaped as policy:problem-details
what_the_application_owns:
  - the read loop and every message it produces
  - the protocol: which variants exist and how they are discriminated
  - any registry of live connections; no hub ships, per decision:websocket-lifecycle-ownership
not_in_scope_2026_08_10:
  - a broadcast hub or topic registry
  - an OpenAPI or AsyncAPI artifact for socket endpoints
  - carrying concept:live-boundary-updates over a socket; decision:live-transport-boundary keeps those on the page route
related:
  - concept:streaming
  - decision:websocket-callback-shape
  - decision:websocket-message-typing
  - api:websocket-entry
  - api:socket-read-write
  - system:tinygodriver-websocket
```
