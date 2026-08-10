---
id: system:tinygodriver-httpserver
type: system
title: tinygodriver httpserver
---
Serves net/http handlers on TinyGo when one of them needs to hijack, by reading the request head itself and routing upgrades around http.Server.

```yaml
import: github.com/shibukawa/tinygodriver/httpserver
released: tinygodriver v1.2.3
entry: "func Serve(ln net.Listener, srv *http.Server) error, and ServeConfig taking a Config"
why_it_exists:
  defect: "TinyGo's net/http starts a background read before calling a handler and cancels it by moving the read deadline into the past; netdev takes the deadline by value when a read begins, so the cancellation never lands and Hijack blocks forever"
  symptom: a handshake that hangs with no error, no panic and no log line
  not_fixable_in_netdev: "the Netdever interface passes the deadline by value per call, so no driver change can interrupt a call already in flight"
  same_root_cause_as: rule:websocket-deadline-discipline
behaviour:
  upgrade_requests: reach the handler through a ResponseWriter implementing http.Hijacker, with no background read in the way
  everything_else: handed to a real http.Server with the head replayed, so keep-alive, timeouts and graceful shutdown keep working
  host_go: "Serve calls srv.Serve(ln) and nothing else"
  shutdown: "it serves the caller's own http.Server rather than a copy, so Shutdown and Close still reach the connections it hands over"
config:
  should_bypass: "which requests need a hijackable connection; nil means IsUpgrade, which matches the Connection: upgrade token and so covers WebSocket without naming it"
  read_header_timeout: "bounds the read of the request head; zero takes http.Server.ReadHeaderTimeout then a 10s default, negative means no limit"
limits:
  bypassed_writer_is_minimal: "Header, Write, WriteHeader and Hijack only — no Flush, no ReadFrom, no CloseNotify, no trailers, no chunked encoding"
  first_request_only: "only the first request on a connection is inspected; a later upgrade on a reused connection is answered 501 rather than deadlocking"
  no_body_on_the_upgrade: "an upgrade request carrying a body or early data is answered 400, because the hijacked reader starts at the connection and would drop those bytes"
  plaintext_only: "TinyGo's http.Server has no ServeTLS and its crypto/tls no Server or X509KeyPair; terminate TLS in front of the process"
becomes_unnecessary_when: TinyGo's net makes deadlines live, or its net/http stops starting the background read; either makes plain http.Server work
used_by: decision:websocket-tinygo-serving, which keeps the dependency in the application bootstrap rather than in this module's import graph
related:
  - system:tinygodriver-websocket
  - decision:websocket-tinygo-serving
  - rule:websocket-deadline-discipline
  - requirement:tinygo-wasm
```
