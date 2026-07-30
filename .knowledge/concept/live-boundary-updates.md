---
id: concept:live-boundary-updates
type: concept
title: Live Boundary Updates
---
Let the server re-render one committed region of a live document repeatedly, on its own clock or on an external event, and resume only that region after a dropped connection.

```yaml
evidence:
  source: user design discussion
  received: 2026-07-30
review_gate: proposed requirements require user approval
status:
  implemented: the language and runtime half, per decision:live-boundary-syntax and decision:live-external-signature; a live boundary re-renders on its source's cadence and the render entry is an iter.Seq2 that does not end
  not_implemented: everything transport, because requirement:component-delta-rendering and the partial-update stack it belongs to are not built yet
  blocked_on_that: decision:response-mode-header, decision:live-transport-boundary, requirement:live-boundary-resume, requirement:live-boundary-lifecycle, rule:live-boundary-delivery, rule:stream-termination-marker, data:live-boundary-subscription
  today: deliveries ride the existing await-boundary stream as id-and-html pairs, and the caller frames them exactly as it frames an await completion
baseline:
  - requirement:suspense-html-streaming
  - requirement:partial-update-boundaries
  - requirement:component-delta-rendering
  - requirement:boundary-parameter-updates
problem:
  one_shot: an await boundary settles once; decision:async-component-signature ends the sequence when every request-owned boundary has settled, and requirement:suspense-html-streaming states that each boundary updates at most once, so nothing on the screen changes again
  client_driven_only: the one later update in the model is requirement:boundary-parameter-updates, which the browser must ask for, so a server that learns something new cannot push a new render of a region it already sent
  consequence: a screen whose content changes on the server's clock or on an external event has no shape here, so the application polls from its own script and re-implements boundary identity, context safety, and delta comparison outside the generated renderer
ask:
  continuous: re-render one committed boundary many times over the life of a subscription
  server_paced: the server decides when a new render exists; the browser does not poll for it
  selective_resume: a dropped connection is re-established for the live boundaries still on screen, and the resume re-renders only those
  same_shape: the Go surface stays the decision:async-component-signature sequence; a live response is one that does not end
motivating_screens:
  metrics: a dashboard chart the server re-renders every few seconds from a sampled source
  chat: a message list a newly arrived message extends
  feed: an RSS or notification list refreshed as items arrive
  mixed: a dashboard holding one live region beside static ones, where a resume must leave the static ones untouched
vocabulary:
  live_boundary: an ordinary await boundary holding at least one live binding, so its primary subtree is re-rendered per delivery instead of once; there is no separate clause, per decision:live-boundary-syntax
  live_source: the requirement:async-external-functions counterpart that yields many values, per decision:live-external-signature
  delivery: one value from a live source, which produces at most one boundary render
  subscription: one live boundary instance being watched for one client, established by executing the page rather than by addressing a handle
  live_mode: the decision:response-mode-header mode in which the page's own route transfers deliveries instead of a document body
  resume: re-requesting the page in live mode after a disconnect, transferring only the live boundaries' renders
extensions:
  - requirement:live-boundary-rendering
  - requirement:live-boundary-resume
  - requirement:live-boundary-lifecycle
syntax: decision:live-boundary-syntax
go_source: decision:live-external-signature
transport: decision:live-transport-boundary
response_mode: decision:response-mode-header
delivery: rule:live-boundary-delivery
authoring_limits: rule:live-boundary-content
subscription_record: data:live-boundary-subscription
runtime_flow: flow:live-boundary-stream
reuse:
  identity: rule:component-instance-identity already names an instance across two executions, which is what a resume needs
  addressing: data:component-update-manifest already carries a per-boundary revision, and decision:response-mode-header removes the need for a handle or a continuation by keeping the render on the page's own route
  comparison: requirement:component-delta-rendering already suppresses unchanged boundary HTML, which is what a periodic re-render needs to stay cheap
  operations: data:component-delta-response already expresses replace, insert, remove, and move, so an appending list needs no new wire form
  safety: rule:template-context-safety and the no-script completion guarantee of decision:client-runtime-ownership carry over unchanged
  capability: requirement:fragment-capability-introspection is the existing place a caller asks whether a response needs a client runtime
  termination: rule:stream-termination-marker generalizes the existing commit-marker reasoning to the end of a stream, which is what lets a client tell a finished render from a truncated one
scope:
  - one primary subtree re-rendered per delivery, with unchanged output suppressed
  - server-paced and event-paced sources, expressed as one Go sequence type
  - resume that re-executes the page but transfers only the live boundaries' renders
  - bounded server cost per subscription, per response, and per reconnect
non_goals:
  - a browser-to-server channel; a live boundary is one-directional and a client change stays api:client-component-update
  - a second endpoint per page; decision:response-mode-header keeps every mode on the page's own route
  - a general pub/sub or presence service; the module renders what a source yields and owns no fan-out topology
  - client-side state a boundary render depends on; a delivery plus reconstructed inputs must fully determine the output
  - replacing concept:streaming, which is typed JSON event delivery for API handlers rather than HTML boundary rendering
milestone: follows the concept:html-render-runtime-extensions async and partial-update extensions; a live boundary is only meaningful once both a committed placeholder and a delta protocol exist
```
