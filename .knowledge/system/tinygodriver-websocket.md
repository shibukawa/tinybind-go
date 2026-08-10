---
id: system:tinygodriver-websocket
type: system
title: tinygodriver WebSocket Packages
---
Two sibling forks in the tinygodriver module supply the WebSocket protocol, one per HTTP transport, with identical Conn APIs and different upgrade shapes.

```yaml
released: both in tinygodriver v1.2.3, alongside system:tinygodriver-httpserver
dependency_bump_needed:
  this_module_2026_08_10: tinygodriver v1.2.1
  required: v1.2.3, which is where all three packages first exist together
packages:
  net_http:
    import: github.com/shibukawa/tinygodriver/websocket
    upstream: gorilla/websocket v1.5.3
    upgrade: "func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*Conn, error)"
    shape: synchronous; the Conn is hijacked and outlives the handler
    server_requirement: system:tinygodriver-httpserver, because TinyGo's net/http cannot complete an upgrade
  fasthttp:
    import: github.com/shibukawa/tinygodriver/fasthttpwebsocket
    upstream: fasthttp/websocket v1.5.12, itself a gorilla fork
    upgrade: "func (u *FastHTTPUpgrader) Upgrade(ctx *fasthttp.RequestCtx, handler FastHTTPHandler) error"
    shape: callback; returns once the 101 is queued, the handler runs after the request handler returned
    lifetime: fasthttp closes the connection when the callback returns unless Server.KeepHijackedConns is set
    package_name: still websocket, so an aliased import reads as upstream does
conn_api_is_identical:
  verified_2026_08_10: twelve methods compared across both conn.go files, signatures equal
  list:
    - "ReadMessage() (messageType int, p []byte, err error)"
    - "NextReader() (messageType int, r io.Reader, err error)"
    - "WriteMessage(messageType int, data []byte) error"
    - "NextWriter(messageType int) (io.WriteCloser, error)"
    - "WriteControl(messageType int, data []byte, deadline time.Time) error"
    - "SetReadLimit(limit int64)"
    - "SetReadDeadline(t time.Time) error"
    - "SetWriteDeadline(t time.Time) error"
    - "SetPingHandler(h func(appData string) error)"
    - "SetPongHandler(h func(appData string) error)"
    - "Subprotocol() string"
    - "Close() error"
  consequence: decision:websocket-core-conn-interface, which is what lets one core serve both without importing either
  not_identical: the two Conn types themselves, the two Upgrader types, and the CheckOrigin and Error hooks, which name their own request type
upgrader_writes_its_own_failure_response:
  both: a refused handshake produces an HTTP error response from the library, before any 101
  hooks: Upgrader.Error on net/http, FastHTTPUpgrader.Error on fasthttp
  consequence: policy:problem-details is reachable only by installing those hooks, per api:websocket-entry
tinygo_constraints_inherited:
  deadline_is_by_value: rule:websocket-deadline-discipline
  serving: decision:websocket-tinygo-serving
  concurrency: netdev serializes bookkeeping and permits concurrent socket calls, so a reader goroutine and a writer goroutine coexist; proven by the driver's own examples/websocketserver clock handler under TinyGo
json_helpers_unused:
  what: both forks carry ReadJSON and WriteJSON built on encoding/json
  why_refused: decision:reflection-free, and encoding/json is what requirement:tinygo-wasm exists to keep out of the graph
  instead: jsonbind, per decision:websocket-message-typing
related:
  - system:tinygodriver-fasthttp
  - concept:typed-websocket
  - decision:websocket-core-conn-interface
  - decision:websocket-tinygo-serving
```
