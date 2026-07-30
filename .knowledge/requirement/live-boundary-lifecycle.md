---
id: requirement:live-boundary-lifecycle
type: requirement
title: Live Subscription Lifecycle And Cost
---
Bound what one client's live subscriptions may hold open on the server, so an always-connected screen has a known cost in goroutines, renders, and authorization lifetime.

```yaml
priority: should
source:
  - concept:live-boundary-updates
  - requirement:live-boundary-rendering
  - user design discussion 2026-07-30
review_gate: proposed
status: not implemented; blocked on requirement:component-delta-rendering, which supplies the response form and the validators every part of this depends on
why:
  new_shape: every earlier render in the model is bounded by a request; a live subscription is bounded by how long a browser tab stays open, which is not a duration the server chose
  per_client_cost: one subscription is at least one goroutine, one source, and one rendered subtree per delivery, and requirement:live-boundary-rendering multiplies that by the boundaries on the screen
  authorization_drift: a session that expires or a permission that is revoked must stop a stream that was authorized minutes or hours ago
bounds:
  per_response_boundaries: a cap on how many live boundaries one live-mode response may serve, reported rather than truncated silently
  per_client_responses: a cap on concurrent live-mode responses per session, so a client cannot multiply subscriptions by reopening
  max_response_duration: a maximum lifetime after which the server closes the response with the rule:stream-termination-marker retry record and expects requirement:live-boundary-resume to re-establish it
  idle_close: a response with no delivery and no client activity for a configured period is closed
  min_interval: an optional floor on how often one boundary may re-render, so an event-paced source cannot render faster than a screen can use
  reason_for_max_duration: a bounded response buys back authorization re-checks, deploy rollover, and load rebalancing
  jitter_required:
    concern: a fixed lifetime re-synchronizes clients every cycle, so one restart produces a herd that then repeats forever rather than dispersing
    fix: spread each response's lifetime around the configured value, per response, so the first cycle desynchronizes the population permanently
    not_client_fixable: a client cannot choose when the server closes it, so this cannot be delegated to backoff
    status: deferred with the rest of this requirement
  cost_of_max_duration: each rollover re-executes the page, per decision:live-transport-boundary execution_is_the_reconstruction, so the lifetime is a tradeoff against that cost rather than a free knob; a live-mode plan slice would make it cheaper
authorization:
  on_open: full authentication and authorization, never inherited from the document request
  on_resume: the page's own checks run again because the page runs again, which is what makes max_response_duration a security control rather than only a resource one
  no_capability_to_expire: there is no continuation token to age out, since decision:live-transport-boundary reconstructs by execution
  mid_stream_revocation: the server may end a subscription when it observes revoked access; the client sees a closed subscription rather than stale content it believes is live
  no_privilege_carry: live mode grants no capability a document request for the same URL would not have granted
cancellation:
  client_gone: the request context cancels, every subscription on that response breaks its pull loop, and each source observes the cancellation through its context
  boundary_removed: a navigation delta or an enclosing re-render that removes a live boundary cancels its subscription before the replacement renders
  shutdown: server shutdown closes live-mode responses with the retry record rather than dropping them, so clients resume against the next instance instead of retrying a dead connection
  client_abort: the client aborting its own live-mode request is ordinary request cancellation, and is the expected teardown before a same-document navigation per rule:live-boundary-delivery
  leak_rule: no source goroutine may outlive its subscription context, which is why decision:live-external-signature makes the context mandatory
load_control:
  status: concern recorded, deferred; nothing here is implemented
  split:
    module: per-boundary and per-response bounds, because only the module knows what a boundary is, plus the cost signal below
    caller: process-wide and per-tenant admission control, because only the deployment knows the budget and has to count every route rather than the live ones
    reason: the module already declines to own transport, routing, and fan-out topology; owning admission policy would be a larger claim than any it has made
  cost_signal:
    unit: live subscriptions, and page executions per interval
    why_unique: it is derived from the template's boundaries and each source's cadence, so no load balancer or middleware can compute it
    capacity: measure and limit executions per second rather than connections, because the expensive part sits behind the proxy
    reassurance: a reconnect is an ordinary page execution, so downstream capacity planning already covers it; a reconnect storm looks like a page-load spike to the database
  degradation:
    unusually_safe: both dials cost freshness rather than correctness, which is rarer than it sounds
    refuse_live_requests: the screen stops updating and stays a valid document, because the committed content is the same output a client with no JavaScript already receives; shedding live load never produces an error page
    raise_min_interval: skipping deliveries is safe by construction under the decision:live-boundary-syntax snapshot model, so the interval is a load dial
  circuit_breaker:
    placement: inside the live source, shared across subscriptions, not at the front door
    problem_it_solves: a failing upstream currently has every client retrying its own source independently, which is the pile-on a breaker exists to prevent
    fits_here: a breaker needs a fallback its callers can live with, and this design has one structurally — the boundary keeps its last rendered content and the page stays correct
    consistent: fan_out below already makes sharing one upstream the application's job, so the breaker lives where that sharing already does
fan_out:
  render_is_per_client: reconstructed inputs and authorization differ per client, so the module renders per subscription and shares nothing by default
  source_is_shareable: one upstream feeding many subscriptions is the application's job inside the source, per decision:live-external-signature
  output_sharing: requirement:component-output-cache is the only sharing the module offers, and it applies when inputs and delivery agree
  cost_statement: N clients watching one gauge cost N renders per tick unless the cached output hits, which is a number an operator must be able to predict from the interval and the client count
observability:
  counters: open live-mode responses, live subscriptions, page executions spent on reconnects, deliveries rendered, deliveries suppressed as unchanged, and subscriptions closed by reason
  per_boundary: render duration and suppression rate, because a boundary that never suppresses is one whose output is not stable enough for its interval
  errors: source failures and render failures go to the render error hook, as async boundary failures already do
  reason: a periodic render is a background load whose regressions no request latency metric will show
backpressure:
  mechanism: the pull sequence of decision:live-external-signature blocks the source until the renderer is ready, so nothing queues server-side
  slow_client: a client that cannot keep up slows the pulls, which coalesces deliveries by rule:live-boundary-delivery instead of growing a buffer
  hidden_tab: pausing or closing the response is client policy; resume makes closing it attractive, at the price of one page execution when it returns
  reconnect_storm: a restart or a rolling deploy makes every client reconnect at once, and each reconnect costs a page execution, so jittered backoff is required of a conforming client rather than left to taste
compression:
  flush_available: per-delivery flushing through a compressing encoder already works, per requirement:suspense-html-streaming, so nothing is blocked
  residual_cost: flushing every few seconds keeps a long-lived stream at a poor compression ratio and emits a sync marker per delivery, which is a bandwidth tradeoff rather than a correctness question
  secrets: a long-lived compressed stream mixing personalized content with anything request-influenced deserves the ordinary compression-oracle caution, since it offers far more samples than one document does
tinygo: requirement:tinygo-wasm constrains nothing here beyond what generated rendering already assumes, because the subscription machinery is a goroutine and a context rather than reflection
acceptance:
  - one screen's live subscriptions have a stated ceiling in responses, boundaries, and renders per interval
  - a client disconnect leaves no source goroutine running
  - a live-mode response that reaches its maximum lifetime is closed and resumed without the user seeing a repaint of anything static
  - revoked access ends an open subscription instead of continuing to render
  - deliveries suppressed as unchanged are counted, so a mispaced boundary is visible in metrics
  - a slow client coalesces deliveries and never grows an unbounded server-side queue
  - the cost of one reconnect is stated in page executions, not only in connections
open_questions:
  - default values for every bound, and whether they are render options, generator options, or both
  - whether a per-boundary interval floor is declared in the template or configured per deployment
  - whether the module offers any shared-render fan-out beyond requirement:component-output-cache, or leaves it entirely to the source
```
