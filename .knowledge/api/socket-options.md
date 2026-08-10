---
id: api:socket-options
type: api
title: Socket Options And Defaults
---
SocketOptions carries the limits, deadlines and negotiation a socket needs, with process-wide defaults every field falls back to and concrete values none of them leave at zero.

```yaml
status: implemented 2026-08-10
shape:
  transport_free: the whole struct lives in bindcore and both surfaces alias it, except the origin check
  process_default: "SetSocketDefaults(SocketOptions), matching SetStreamErrorHandler and SetMaxJSONBodyBytes"
  per_call: the options form of api:websocket-entry, for the endpoint whose cadence differs
  zero_means_default: an unset field takes the process default, and a process default left unset takes the value below; nothing reaches the driver as zero
fields:
  read_limit:
    default: 1 MiB
    reason: the same number as jsonbind's DefaultMaxJSONBodyBytes, so the two limits are unsurprising together
    separate_knob_because: a socket message is not an HTTP body, and raising one should not raise the other
    on_breach: the driver closes with a protocol code
  idle_timeout:
    default: 60s
    is: the read deadline set before every read, per rule:websocket-deadline-discipline
    reason: gorilla's canonical pong wait, so an application moving from gorilla sees the timing it already had
    cannot_be_zero: an unbounded read is unrecoverable under TinyGo, so the API offers no spelling for it
  ping_interval:
    default: 54s
    reason: gorilla's canonical nine tenths of the pong wait
    constraint: must be below idle_timeout, or the ping only ever fires after the connection was declared dead
    validated: the entry refuses an options value that breaks the constraint, rather than serving a socket that dies on schedule
  write_timeout:
    default: 10s
    is: the deadline set before every write, including the runtime's control frames
    reason: a stalled peer must not pin a writer, and 10s is gorilla's canonical write wait
  buffer_sizes:
    default: 4096 read and write
    reason: the driver's own example uses it, and zero would take the HTTP server's buffers, which differ per transport
  subprotocols:
    default: none
    negotiated_by: the upgrader, choosing the first server entry the client also offered
    read_by: "Socket.Subprotocol(), per api:socket-read-write, so the callback never needs the driver Conn for it"
  compression:
    default: off
    reason: permessage-deflate pulls flate into the binary, which requirement:tinygo-wasm cares about, and only no-context-takeover mode is supported so the saving is smaller than the cost
    overridable: yes, per call and per process
  check_origin:
    default: refuse when Origin is present and its host differs from Host
    shape: "func(origin, host string) bool, per decision:websocket-origin-check-seam"
handshake_refusals:
  shape: policy:problem-details, written through the driver's Error hook
  codes:
    - code: websocket_upgrade
      status: 400
      when: the request carries no upgrade tokens, or its method is not GET, which the driver answers 405
    - code: websocket_origin
      status: 403
      when: the origin check refused
    - code: websocket_hijack
      status: 500
      when: "the net/http ResponseWriter is not an http.Hijacker, per decision:websocket-tinygo-serving"
      body_says_internal: "policy:problem-details hides a 5xx code, so this name reaches the returned error and the log rather than the client; a server that cannot hijack is a misconfiguration the client should learn nothing about"
  naming: snake_case short codes, matching json_parse, body_read and multipart_parse
related:
  - api:websocket-entry
  - api:socket-read-write
  - decision:websocket-lifecycle-ownership
  - decision:websocket-origin-check-seam
  - rule:websocket-deadline-discipline
  - policy:problem-details
```
