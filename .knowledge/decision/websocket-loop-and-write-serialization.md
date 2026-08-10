---
id: decision:websocket-loop-and-write-serialization
type: decision
title: The Handler Owns The Loop, The Socket Serializes Writes
---
Leave the read loop to the callback and serialize Write inside the socket, because the loop is where every protocol differs and the concurrent-write ban is where every gorilla application breaks.

```yaml
status: implemented 2026-08-10
decided_by: owner, 2026-08-10
loop_ownership:
  chosen: the callback loops, calling Read and Write in whatever order the protocol needs
  rejected_runtime_loop:
    shape: "the entry takes func(context.Context, In) (Out, error) and the runtime loops"
    reads_well_for: request and response over a socket
    cannot_express: a server that talks first — a clock, a notification feed, a subscription that answers once and then pushes for an hour — which is most of what a socket is for
    consequence: it would need an escape hatch to the loop-owning shape anyway, and then the surface carries both
  rejected_shipping_both: refused for now; the sugar is buildable on top later if request-and-response turns out common enough to earn a name
write_serialization:
  chosen: Write takes a mutex, so any goroutine may call it
  what_it_fixes: gorilla permits one concurrent reader and one concurrent writer, and forbids concurrent writers with no diagnostic; the failure is interleaved frames on the wire, seen by the peer and not by the server
  why_the_library_should_hold_it: broadcast and server-initiated push are the ordinary uses of a socket, and both mean a second goroutine writing; an API that hands out an unguarded writer hands out that defect
  covers: the data frames and the runtime's own control frames, which share the lock, so a lifecycle ping cannot interleave with an application message
  cost: one mutex acquisition per message, beside a JSON encode on the same path
read_stays_unguarded:
  what: Read is not serialized and must be called from one goroutine
  why: two readers on one socket is not a use, it is a bug, and a mutex there would hide it as a deadlock instead of surfacing it
  documented_as: a single-reader contract, matching the underlying library's
a_reader_must_run:
  fact: control frames are handled inside the read call, so a socket with no active reader answers no ping and notices no close
  source: the driver's own example says so, and its push-only handler runs a reader goroutine for exactly this reason
  consequence_for_push_only_handlers: a callback that only writes still has to read, or the connection dies silently on the first idle timeout at either end
  library_response: decision:websocket-lifecycle-ownership, whose idle deadline makes the missing reader fail loudly and early rather than at some proxy's whim
related:
  - concept:typed-websocket
  - decision:websocket-lifecycle-ownership
  - api:socket-read-write
  - rule:websocket-deadline-discipline
```
