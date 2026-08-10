---
id: api:socket-read-write
type: api
title: Socket Read And Write
---
Socket[In, Out] carries one JSON value per WebSocket text frame, decoding into In and encoding from Out through jsonbind, with Write safe from any goroutine.

```yaml
status: implemented 2026-08-10, per decision:websocket-message-typing
type: "bindcore.Socket[In, Out], aliased under one name by both surfaces, as Stream[T] is"
methods:
  read: "func (s *Socket[In, Out]) Read() (In, error)"
  write: "func (s *Socket[In, Out]) Write(v Out) error"
  close: "func (s *Socket[In, Out]) Close() error"
  subprotocol: "func (s *Socket[In, Out]) Subprotocol() string, the one the upgrader negotiated from api:socket-options"
  conn: "each shell exposes its own driver Conn, for anything the typed surface does not carry"
read_behavior:
  - set the read deadline from the idle bound, per rule:websocket-deadline-discipline
  - read one message
  - decode the payload into In through the generated codec
  - a decode failure is returned and does not close the socket, so a protocol error is the application's to answer
  - single reader only, per decision:websocket-loop-and-write-serialization
write_behavior:
  - take the write mutex, so any goroutine may call it
  - set the write deadline
  - open a text frame writer and encode straight into it, so no intermediate buffer holds the message
  - a closed socket returns a named error rather than blocking
frames:
  text_only: JSON travels in text frames; a binary frame is returned as an error rather than decoded
  control_frames: handled inside Read by the driver's own handlers, which is why a socket with no active reader answers no ping
jsonbind_gap:
  found_2026_08_10: "jsonbind decodes from an io.Reader only — DecodeJSON, DecodeJSONLimit, DecodeJSONHint — and a WebSocket read already holds the whole message as bytes"
  needed: "an exported DecodeJSONBytes[T]([]byte) (T, error) that looks the decoder up and calls it, skipping the reader and the copy"
  why_not_wrap_a_bytes_reader: it allocates a reader and re-reads the payload into a second buffer for every message on every connection
  encoding_needs_nothing: "EncodeJSON already takes an io.Writer, which is what NextWriter returns"
errors:
  decode: returned to the caller, with the message left consumed
  read_limit: the driver closes with a protocol code and returns its own error
  idle_timeout: a timeout error from the read, which the callback turns into a return
  post_commit: anything the callback returns goes to the installed sink, per decision:websocket-callback-shape
related:
  - concept:typed-websocket
  - api:websocket-entry
  - decision:websocket-message-typing
  - decision:websocket-loop-and-write-serialization
  - rule:websocket-deadline-discipline
  - api:decode-json
  - api:encode-json
```
