---
id: decision:websocket-core-conn-interface
type: decision
title: The Core Names A Message Connection Interface
---
Let bindcore hold the whole typed socket over a small interface both driver Conn types already satisfy, rather than duplicating it per transport, because here an interface call costs one dispatch per message rather than one per field.

```yaml
status: implemented 2026-08-10
problem:
  stream_had_none: Stream[T] writes to an io.Writer, so bindcore named no transport and both surfaces aliased one type
  socket_does: a socket needs the Conn, and the two Conn types are different types in different packages
  forbidden_shortcut: bindcore importing either websocket package, which would pull fasthttp into every net/http build and break decision:fasthttpbind-runtime-package's forbidden edges
shape:
  interface: |
    type MessageConn interface {
        ReadMessage() (int, []byte, error)
        NextWriter(int) (io.WriteCloser, error)
        WriteControl(int, []byte, time.Time) error
        SetReadLimit(int64)
        SetReadDeadline(time.Time) error
        SetWriteDeadline(time.Time) error
        SetPongHandler(func(string) error)
        Subprotocol() string
        Close() error
    }
  satisfied_structurally: both driver Conn types match with no adapter and no wrapper, verified in system:tinygodriver-websocket
  core_owns: Socket[In, Out], Read, Write, the write mutex, the lifecycle, and the options
  shell_owns: the upgrade call, the origin check, and the failure response
answers_the_standing_objection:
  objection: decision:fasthttpbind-no-transport-interface refuses a Request and Writer interface both transports implement
  its_reason: an interface call per field access reintroduces the indirection fasthttp exists to remove, spending the whole budget it was built to save
  why_it_does_not_reach_here:
    granularity: one dynamic call per WebSocket message, beside a JSON encode of the same message, rather than one per bound field
    not_the_author_shape: the interface is internal; no application signature names it, so the framework-facilities objection about moving what the author writes does not apply
    honest_common_subset: both Conn types are the same upstream API, so the interface describes a real shared contract rather than a least common denominator invented to join two unlike things
  scope_of_the_answer: this argument licenses an interface at message granularity over an already-shared API; it does not reopen the binder question
why_not_duplicate_the_socket_per_package:
  precedent_against: bindcore holds the stream whole because two implementations of one wire contract are two chances to disagree
  here: the read discipline, the deadline placement, the close handshake, and the write serialization are all behaviour a parity test would have to police twice
  cost_of_the_interface: one interface value per connection and one dynamic call per message, which is not measurable beside the JSON work on the same path
not_exported:
  where: internal/bindcore
  why: an exported MessageConn invites an application to supply its own, which would put an untested framing implementation behind the typed API
  escape_hatch: the socket exposes the concrete driver Conn from each shell instead, per api:socket-read-write, so an application needing Subprotocol or a custom ping handler reaches its own transport's type
related:
  - concept:typed-websocket
  - decision:fasthttpbind-no-transport-interface
  - decision:fasthttpbind-runtime-package
  - decision:runtime-package-boundaries
  - system:tinygodriver-websocket
```
