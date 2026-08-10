---
id: decision:websocket-callback-shape
type: decision
title: WebSocket Entry Is A Callback That Returns Only The Handshake Error
---
Give both transports one entry that takes a callback holding the socket and returns only the handshake error, because fasthttp can express nothing else and the handshake is the one failure both transports can still answer.

```yaml
status: implemented 2026-08-10
precedent: decision:stream-callback-shape, whose argument fasthttp forces a second time
forcing_constraint:
  fasthttp: "FastHTTPUpgrader.Upgrade(ctx, func(*Conn)) runs the callback after the request handler returned, and closes the connection when it returns"
  net_http: "Upgrader.Upgrade(w, r, hdr) (*Conn, error) hands back a hijacked Conn the handler may keep, return from, or hand to goroutines"
  effect: the net/http shape has no fasthttp transcription; the callback shape has a net/http one
  choice_taken: move net/http to the shape fasthttp requires, so one handler source compiles on both
two_windows_not_one:
  handshake:
    when: before any 101
    both_transports: synchronous and in handler scope, so an error can travel back
    answer: the entry returns it
  after_upgrade:
    when: the callback is running
    net_http: still in handler scope, but the 101 is already sent
    fasthttp: the handler has returned
    answer: post-commit on both, routed to the installed sink like a stream error
  why_this_differs_from_the_stream: decision:stream-callback-shape returns nothing because its only pre-commit failure was one fasthttp could not express; here the handshake is pre-commit on both, so returning it costs no symmetry
returning_the_handshake_error:
  meaning: the handshake failed and a response has already been written
  not_meaning: the caller should write a response
  why_returned_at_all: the handler still holds the request, so it can log or count the refusal with the peer in scope, which is what the driver's own example does
  why_not_routed_to_the_sink: a process-wide sink cannot see which route or which peer, and a refusal is ordinary traffic rather than an escaped failure
  call_site_cost: "the happy path reads _ = httpbind.WebSocket(...), which is the price of an honest signature"
runtime_closes_the_socket:
  what: the entry closes the socket when the callback returns, whatever it returns
  why: the same two defects decision:stream-callback-shape names — a forgotten close and a discarded error — appear here as a peer left waiting for a close frame and an invisible write failure
  detail: decision:websocket-lifecycle-ownership, which makes the close a close handshake rather than a socket teardown
lifetime_note: on fasthttp the callback outlives the handler, so rule:fasthttpbind-requestctx-lifetime forbids reading ctx inside it; everything the callback needs is captured before the entry returns, including the peer address and any authenticated identity
naming:
  chosen: WebSocket, because Socket is the type and an entry point cannot share the name, the same collision WriteStream resolved
  considered_upgrade: reads correctly for the pre-commit failure but promises gorilla's return-a-Conn shape, which this is not
  considered_serve_socket: accurate and unlike anything else in the surface, which is the argument against it
related:
  - concept:typed-websocket
  - decision:stream-callback-shape
  - api:websocket-entry
  - rule:fasthttpbind-requestctx-lifetime
  - policy:problem-details
```
