---
id: decision:websocket-lifecycle-ownership
type: decision
title: The Runtime Owns Limits, Deadlines, Pings And The Close Handshake
---
Give the socket runtime the read limit, the idle deadline, the ping cadence and the closing handshake, and give the application no way to leave any of them unset, because each one unset is a defect that shows up only in production.

```yaml
status: implemented 2026-08-10
decided_by: owner, 2026-08-10, choosing lifecycle over a thinner wrapper
scope_boundary:
  in: read limit, idle deadline, write deadline, ping cadence, pong accounting, close handshake
  out: a connection registry, topic routing, broadcast; those stay the application's, per concept:typed-websocket
  reason_for_the_line: everything in is a property of one connection and has a wrong default; everything out is a property of the application's design
read_limit:
  what: a byte ceiling per inbound message, refused by the library with a close frame
  default: non-zero
  relation_to_json: jsonbind's MaxJSONBodyBytes bounds an HTTP body, and a socket message is not one; the socket takes its own knob so raising one does not silently raise the other
  why_not_optional: an unbounded message is an allocation the peer chooses
idle_deadline:
  what: the bound placed on every read, per rule:websocket-deadline-discipline
  default: non-zero
  cannot_be_disabled: an unbounded read is unrecoverable under TinyGo, so the API offers no way to spell it
  effect_on_a_push_only_handler: it fails loudly on the first idle period rather than surviving as a connection nobody can close
ping:
  what: the runtime sends a ping on a cadence and expects the pong to arrive through the read path
  lock: control frames take the same mutex as data frames, per decision:websocket-loop-and-write-serialization, so a ping cannot interleave with a message
  incoming_pings: answered by the driver's default handler inside the read call, so the reader is what keeps the connection alive in both directions
  cadence_default: shorter than the idle deadline, because a cadence at or above it is a timer that only ever fires after the connection has already been declared dead
  validated: the entry refuses a cadence at or above the idle bound rather than serving a socket that dies on schedule
  the_pong_handler_is_not_optional:
    fact: a pong is consumed inside the read call and never returns from it, so a peer answering every ping still expires on the deadline the read installed
    therefore: the runtime installs a pong handler that pushes the read deadline forward, which is gorilla's own idiom and works under TinyGo because the handler runs between recv calls
    reading: the deadline set before each read bounds a silent peer; the pong handler is what keeps a talking one alive
    implemented_2026_08_10: yes, and covered by TestThePongHandlerPushesTheReadDeadline
close:
  what: the runtime sends a close frame with a normal-closure code when the callback returns, then closes the connection
  unconditional: the same argument decision:stream-callback-shape makes for the trailing bracket of a JSON array — a peer left without a close frame is the socket's version of a truncated document
  application_close: returning from the callback is the way to close; a Close method exists for the goroutine that wants to end someone else's loop, and it makes the reader's next deadline expire rather than trying to interrupt it
  fasthttp_note: fasthttp closes the connection when its callback returns anyway, so the close frame has to be sent before that, which is what the runtime doing it guarantees
configuration_shape:
  process_defaults: package-level setters, matching SetStreamErrorHandler and SetMaxJSONBodyBytes
  per_call_override: an options value passed at the entry, for the endpoint whose cadence differs
  transport_free_part: limits, deadlines, cadence, subprotocols, buffer sizes and compression live in bindcore
  per_transport_part: only the origin check, per decision:websocket-origin-check-seam
post_commit_errors:
  route: the sink installed for streams, since a socket failure after the 101 has the same character — no status left to carry it
  open_question: whether one sink covers both or a socket takes its own installer; one sink is proposed, because a process that wants to tell them apart can read the error
related:
  - concept:typed-websocket
  - rule:websocket-deadline-discipline
  - decision:websocket-loop-and-write-serialization
  - decision:websocket-origin-check-seam
  - decision:stream-callback-shape
```
