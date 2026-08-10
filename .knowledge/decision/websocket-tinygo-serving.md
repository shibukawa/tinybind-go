---
id: decision:websocket-tinygo-serving
type: decision
title: The Application Supplies The Upgrade-Capable Server
---
Document that a net/http socket under TinyGo has to be served through the driver's httpserver, rather than re-exporting it, because the import edge would land in every net/http build to serve one that does not need it.

```yaml
status: implemented 2026-08-10
implemented: the Hijacker assertion, which turns the silent hang into a websocket_hijack refusal; covered by TestANonHijackableWriterIsRefusedRatherThanHanging
documented_2026_08_10: docs/httpbind.md and docs/httpbind.ja.md 'Serving it under TinyGo', with the fasthttp contrast in docs/httpbind_fasthttp.md
example_2026_08_10: examples/websocket, a chat room on one port beside an ordinary Write route, whose only unusual line is the httpserver.Serve call
example_doubles_as_the_end_to_end_check:
  what: "regenerating it yields exactly three registrations — a decoder for the inbound type, an encoder for the outbound one, and a writer for the REST response"
  why_it_is_evidence: "generate.go deliberately omits -generate-all, so the output is discovery's alone; anything more means discovery widened, anything less means the socket stopped being found"
  tested: examples/websocket/chat_test.go covers the broadcast, the protocol refusals, the shared port and the cross-origin refusal
constraint:
  fact: "TinyGo's net/http cannot complete a protocol upgrade; it starts a background read before the handler and cancels it by moving the read deadline into the past, which netdev cannot do to a recv already in flight"
  symptom: "the handshake hangs with no error, no panic and no log line; the client times out and the server logs nothing"
  fix: "serve through github.com/shibukawa/tinygodriver/httpserver, which reads the request head itself and hands upgrades a hijackable writer while everything else goes to a real http.Server"
  under_host_go: "httpserver.Serve calls srv.Serve and nothing else, so the line is correct on both compilers"
  fasthttp_unaffected: "RequestCtx.Hijack is a synchronous handoff with no background read, so the fasthttp backend needs none of this"
chosen: document it, ship an example, and do not re-export
why_not_re_export:
  import_graph: "requirement:tinygo-wasm exists because import graphs decide what is compiled; an httpbind.Serve would put the driver's server in the graph of every net/http build, including the ones with no socket"
  precedent: "decision:caller-writes-the-response and htmlbind both stop short of owning the server, and this package sets no status and chooses no encoding for the same reason"
  cost_of_documenting_only: "one line in the application bootstrap that this package does not check; a caller who omits it gets the silent hang above"
  mitigation: "the socket entry can detect that its ResponseWriter is not an http.Hijacker and refuse with a named error, which turns the hang into a diagnostic without taking the import edge"
  worth_noting: "that check costs one interface assertion at handshake time and is the whole remedy, so it is proposed rather than optional"
application_bootstrap:
  already_two_copies: "decision:backend-build-tag-mode says the tagged server bootstrap is the one application file expected per transport, so the socket adds a line to a file that already differs"
  net_http: "httpserver.Serve(ln, srv) in place of srv.Serve(ln)"
  fasthttp: unchanged
open_question:
  what: whether the fasthttp backend should set Server.KeepHijackedConns
  today: "fasthttp closes the connection when the upgrade callback returns, which is what decision:websocket-callback-shape wants, so the default is right for the callback shape"
  revisit_if: an application wants to hand the connection to a goroutine that outlives the callback, which the callback shape does not offer on either transport
related:
  - system:tinygodriver-websocket
  - decision:backend-build-tag-mode
  - requirement:tinygo-wasm
  - decision:websocket-callback-shape
  - rule:websocket-deadline-discipline
```
