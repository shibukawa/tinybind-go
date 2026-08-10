---
id: rule:websocket-deadline-discipline
type: rule
title: Every Socket Read Carries A Deadline
---
Set a non-zero read deadline before every read and never expect a deadline to cancel a read already blocked, because netdev takes the deadline by value at call time and an unbounded read cannot be recovered.

```yaml
status: implemented 2026-08-10
verified_2026_08_10: read from netdev source, not inferred
mechanism:
  signature: "netdev Device.Recv(sockfd int, buf []byte, flags int, deadline time.Time)"
  meaning: the deadline is an argument, so each call takes whatever value the connection held when that call began
  wait: "waitFD selects with the remaining time and returns ErrTimeout when it expires"
  zero_deadline: "waitFD returns immediately when the deadline is zero, and the call then blocks inside a plain recv() with no timeout"
three_consequences:
  between_reads_works:
    fact: a deadline set after one read returns and before the next begins is picked up by the next call
    therefore: gorilla's idiom — a pong handler pushing the read deadline forward — works, because the handler runs inside the read path between recv calls
    scope: this is the part that is often assumed broken and is not
  in_flight_cannot_be_cancelled:
    fact: a deadline set from another goroutine cannot reach a call that already passed its own
    therefore: no writer goroutine can unblock a reader, and no shutdown path may be built on trying
    same_root_cause: the defect that makes TinyGo's net/http Hijack deadlock, and that makes SetDeadline-based query cancellation ineffective for the PostgreSQL driver
  zero_is_unrecoverable:
    fact: a read with no deadline blocks forever with nothing able to interrupt it
    therefore: an idle socket with no deadline is a goroutine and a connection held until the peer acts or the process exits
    severity: this is the rule's reason to exist; the other two are the reasoning that gets here
required:
  - the runtime sets the read deadline immediately before each read, from the configured idle bound
  - the idle bound has a non-zero default; a caller may raise it and may not disable it
  - shutdown waits for the reader to wake on its own deadline, rather than trying to interrupt it
  - a write carries its own deadline for the same reason, so a stalled peer cannot pin a writer
applies_to:
  tinygo: where the mechanism is real
  host_go: where the discipline is harmless, and where writing it any other way would produce two behaviours from one source
related:
  - decision:websocket-lifecycle-ownership
  - decision:websocket-loop-and-write-serialization
  - system:tinygodriver-websocket
  - rule:fasthttpbind-requestctx-lifetime
```
