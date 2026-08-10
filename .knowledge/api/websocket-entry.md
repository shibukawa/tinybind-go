---
id: api:websocket-entry
type: api
title: WebSocket Callback Entry
---
Upgrades a request to a WebSocket, runs the caller's callback against a typed socket, closes it, and returns only the handshake error.

```yaml
status: implemented 2026-08-10, per decision:websocket-callback-shape
signatures:
  httpbind: "func WebSocket[In, Out any](w http.ResponseWriter, r *http.Request, fn func(*Socket[In, Out]) error) error"
  fasthttpbind: "func WebSocket[In, Out any](ctx *fasthttp.RequestCtx, fn func(*Socket[In, Out]) error) error"
  with_options: "the same, taking a SocketOptions value; the two-argument form takes the process defaults"
name_reason: Socket is the type, so the entry cannot share the name, the collision WriteStream already resolved for streams
example: |
  func chat(w http.ResponseWriter, r *http.Request) {
      user := auth.From(r)                       // captured before the callback
      _ = httpbind.WebSocket(w, r, func(s *httpbind.Socket[ClientMsg, ServerMsg]) error {
          for {
              in, err := s.Read()
              if err != nil {
                  return err
              }
              if err := s.Write(ServerMsg{Type: "message", From: user, Text: in.Text}); err != nil {
                  return err
              }
          }
      })
  }
behavior:
  - check the origin, per decision:websocket-origin-check-seam
  - upgrade; a refusal writes policy:problem-details through the driver's Error hook and returns
  - build the socket over the driver Conn, applying the read limit and the deadlines
  - run the callback
  - send a close frame and close the connection when the callback returns, whatever it returns
  - route a non-nil callback error to the installed post-commit sink
return_value:
  is: the handshake error, and nothing else
  is_not: the callback's error, which is post-commit on both transports
  response_already_written: a non-nil return means the refusal response has been sent; the caller logs or counts, it does not answer
transport_difference_hidden_from_callers:
  net_http: "Upgrade returns a hijacked Conn and the callback runs before the handler returns"
  fasthttp: "the callback is installed through FastHTTPUpgrader.Upgrade and runs after the handler returns; fasthttp closes the connection when it returns"
  identical: the callback body, which is the point
lifetime_note: on fasthttp the callback outlives the handler, so rule:fasthttpbind-requestctx-lifetime forbids reading ctx inside it; the identity, the peer address, and anything else from the request are captured before the entry returns
hijacker_check:
  what: the net/http entry asserts http.Hijacker before upgrading and refuses with a named error when it is absent
  why: decision:websocket-tinygo-serving, where the alternative is a silent hang under TinyGo
related:
  - concept:typed-websocket
  - api:socket-read-write
  - decision:websocket-callback-shape
  - decision:websocket-origin-check-seam
  - api:write-stream
  - policy:problem-details
```
