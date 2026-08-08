---
id: requirement:live-boundary-rendering
type: requirement
title: Live Boundary Rendering
---
Re-render one committed component boundary once per delivery from a server-paced source, for as long as the client holds a subscription, sending only what changed.

```yaml
priority: should
source:
  - concept:live-boundary-updates
  - user design discussion 2026-07-30
review_gate: proposed
status:
  shipped:
    - a boundary holding a live binding re-renders its subtree once per delivery, for as long as the subscription lives
    - several bindings in one clause, each writing its own scope field, with every render reading all of them
    - one clause mixing a settle-once binding and a live one, which is what removing the separate clause keyword unlocked
    - generated output for a boundary with no live binding is byte-identical to before, because it still compiles to the settle-once join
    - Content.AppendJSON, the record form a delivery takes once no parser is reading it
    - the fallback commits first and the placeholder is written exactly as an await boundary's is
    - the sync entry renders the first delivery in place, and keeps the fallback when nothing was delivered
    - a first-delivery deadline on the answering entries, so a quiet source cannot hold a response open
    - a failure delivery renders recover and a later value replaces it; a clause with no recover ends the subscription with UnrecoveredError
    - HasLiveBlock on Plan, Fragment, Wrapper, and over a chain
    - boundary ids repeat across executions of the same chain, which is what a resume needs
    - ids are positional: each boundary's subtree is its own namespace, so a nested boundary is tb-1-1 rather than a number from a shared counter
    - a live boundary's subtree hands out the same ids on every delivery instead of minting new ones
    - a superseded delivery's nested boundaries are cancelled before their ids are reused, so stale content cannot land in the replacement's placeholder
    - the error reporter is called after the delivery lock is released, per requirement:live-error-report-off-lock, so a reporter that blocks no longer holds the clause's other bindings
  known_gaps:
    - no bound on live subscriptions: the await concurrency limit deliberately excludes them, so nothing caps how many one render opens; the useful unit is per process, which requirement:live-boundary-lifecycle owns
    - the live entry does not enforce that the body is discarded, so a caller passing a real writer gets the endless document response decision:live-transport-boundary rejected
    - a live boundary is indistinguishable from a settle-once one, both in the placeholder it writes and in the delivery record it yields, so a client cannot classify a region and a live-mode consumer cannot separate the two kinds of delivery; requirement:live-boundary-liveness-signal owns it
    - a source must not mutate a value it already yielded, since the boundary renders from the scope it was written into; that contract is not stated in the Go documentation
  downstream_round: decision:live-integration-seams records the 2026-07-31 findings against this shipped runtime and the order they are taken in
tinygo:
  covered: the htmlbind suite runs under TinyGo natively and on wasip1, plus a smoke command that renders a live boundary on both, so the goroutines, the context selects, and the pull sequences are exercised on the cooperative scheduler
  panic_limitation:
    finding: TinyGo's wasm targets do not run a deferred recover at all, confirmed against a plain function with no goroutine or library involved
    consequence: a panicking async external or live source ends the program there instead of becoming a data:async-render-error, so requirement:async-external-functions failure normalization does not hold on wasm
    scope: the platform's, not this project's; the tests that depend on it return early and say why, and htmlbind.panicRecovery names the condition
    not_workable_around: nothing in the runtime can catch a panic the runtime will not deliver
  skip_limitation: TinyGo implements neither testing.T.SkipNow nor the runtime.Goexit it needs, and reports a skipped test as a failure, so a conditional test logs and returns instead of skipping
  deferred:
    - reason: requirement:component-delta-rendering is not implemented, so there is no data:component-delta-response to carry a delivery and no validator to compare against
    - consequence: a delivery is currently the existing id-and-html Content, so nothing is suppressed as unchanged and nothing advances a revision
    - migration: the runtime seam is the yielded value, so the delta form replaces it without touching the clause, the source signature, or generated code
syntax: decision:live-boundary-syntax
source_signature: decision:live-external-signature
transport: decision:live-transport-boundary
delivery: rule:live-boundary-delivery
runtime_flow: flow:live-boundary-stream
boundary:
  primary: component subtree re-rendered per delivery, reading every binding of the clause
  fallback: synchronously renderable subtree committed before the first delivery
  recover: synchronously renderable failure subtree receiving data:async-render-error
  update_boundary: a live boundary is a requirement:partial-update-boundaries boundary, so it already has an instance identity, a manifest entry, and a revision
initial_response:
  - commit the fallback subtree, or the first delivery when decision:live-transport-boundary first_delivery_inline applies
  - publish a data:component-update-manifest entry marking the instance live, so the client knows a live mode request is worth issuing
  - the document response then ends normally; no live delivery arrives on it
subscription_execution:
  - authenticate the decision:response-mode-header live-mode request as the ordinary page request it is
  - execute the route handler, layouts, and page for the same URL, and discard the body output instead of transferring it
  - start each live source under the request context, per decision:live-external-signature
  - on each delivery, re-render only that boundary subtree with the delivery bound
  - compare descendant content validators and emit a data:component-delta-response scoped to that subtree
  - advance and return the boundary revision on every accepted delivery, including one that produced no operation
  - keep ranging until the source ends, the client disconnects, or requirement:live-boundary-lifecycle bounds the response
purity:
  requirement: boundary output is a function of the delivery plus the reconstructed inputs, with no state carried between deliveries
  reason: requirement:partial-update-boundaries already requires that boundary rerendering be side-effect-free and safe to repeat; a live boundary repeats it on the server's clock instead of on a request
  consequence: a coalesced or missed delivery cannot corrupt the screen, which is what lets rule:live-boundary-delivery drop intermediates
cache:
  reuse: requirement:component-output-cache applies per delivery, so two clients whose reconstructed inputs and delivery agree can share one rendered output
  keying: a delivery value participates in the input validator, so a cached entry is never reused across differing deliveries
  request_bound: a boundary carrying a requirement:awaitable-parameters handle is request-bound and therefore not cacheable, which is unchanged
not_in_a_redraw:
  settled: 2026-08-08; a reloadable component may not own a live boundary, and declaring one should fail at generation
  reason: decision:live-transport-boundary reconstructs a subscription by executing the page, and a redrawn region's arguments came from the client, so page execution would subscribe against different arguments
  silent_otherwise: the author gets a fallback that never updates, with no diagnostic, which is the failure mode decision:cache-component-declaration rejects await for
  condition_is_already_computed: Plan.HasLiveBlock covers the call graph, and Fragment.HasLiveBlock exposes it
  supersedes: the patch clause below, whose restart reading assumed a redraw could own the subscription it restarts
navigation_is_where_live_arrives:
  shipped: renderDelta sets the live field when the composition owns a live boundary, and markLive sets the response header on the document and delta paths alike
  reason_in_code: a navigation can reach a route whose composition owns a live boundary while the client reused its document shell, so the body is the only place that can say so
  client_side: requirement:update-wire-contract live_handoff_sequence owns what a client does with the marker
parameter_interaction:
  patch: api:client-component-update redraws a live boundary with caller-supplied inputs per requirement:component-redraw-endpoint, restarting the subscription
  restart: the old source is cancelled before the new one starts, and the boundary's next render comes from the new source's first delivery
  no_interleave: a delivery from a cancelled source is discarded rather than applied, per rule:live-boundary-delivery
identifiers:
  positional: a boundary id names its position in the render tree, not the order it was allocated in; each boundary's subtree is a namespace under its own id
  reused: a live boundary's subtree renders again per delivery and hands out the same ids, because minting one per delivery grows them without bound and leaves a client holding placeholders nothing will replace
  determinism_gained: a nested boundary's id no longer depends on the order sibling boundaries happen to settle in, which is what makes the same chain executed again address the placeholders already on screen
  superseded_work: the previous delivery's nested boundaries are cancelled before the next delivery reuses their ids; without that a slow one would settle into the replacement's placeholder and put stale content on screen
  breaking: a project with a boundary nested inside another one sees different ids than before, because they were previously drawn from one flat counter
nested:
  inside_live: a nested live boundary inside a live primary subtree is discarded and re-subscribed when the outer boundary re-renders, so an outer delivery must not leak the inner subscription
  inside_navigation: a navigation delta that removes a live boundary must end its subscription; one that inserts a live boundary is picked up by the next live mode request, because that request subscribes to whatever the page renders
  await_inside: an await clause inside a live primary subtree re-runs its work per delivery, which decision:live-boundary-syntax permits and flags rather than forbids
safety:
  - rendered deliveries follow rule:template-context-safety and are context-checked in an isolated buffer, as async completions already are
  - no delivery carries script, so the client applies operations through its own trusted runtime
  - recover content receives only safe public error fields
  - a live boundary may not hold browser-owned state, per rule:live-boundary-content, because a delivery replaces its subtree without asking the user
  - a client validator is an optimization hint and never bypasses authorization or typed reconstruction, which is the requirement:component-delta-rendering rule unchanged
failure:
  transient: a failure delivery renders recover and a later value replaces it, per decision:live-external-signature
  terminal: the source ending leaves the last rendered content and ends the subscription; the client is told the subscription closed rather than left waiting
  render_error: a delivery whose render fails keeps the current content, advances nothing, and reports through the render error hook
  incompatible_render_version: instruct a reload rather than applying operations from a different generated version
no_javascript:
  behavior: the committed content stays as the final render, because the client never issues a live mode request
  alternative: the decision:live-boundary-syntax sync entry renders the first delivery and stops, so one template serves both clients
capability: requirement:fragment-capability-introspection reports a live flag, so a caller adds a live client script only for responses that need one
compatibility: requirement:html-rendering-compatibility; a template with no live binding renders byte-identically and answers a live mode request with nothing to deliver
acceptance:
  - a boundary bound to a source yielding every few seconds re-renders on that cadence with no browser polling
  - a delivery whose rendered output is unchanged sends no HTML and only advances the revision
  - a delivery that appends one list item sends an insert operation rather than the whole list
  - a delivery re-renders only its own boundary subtree; sibling boundaries and ancestors are not rendered again for it
  - restarting a subscription after a parameter patch cannot apply a delivery from the previous source
  - a client that loads no script keeps the content committed by the document response
  - a source failure renders recover, and a later successful delivery restores primary content
open_questions:
  - whether a live boundary may be declared inside a cached component, given the cache stores output the boundary is about to replace
  - whether the requirement:live-boundary-resume deferred validator header is worth carrying, given the first milestone always sends a resumed subscription's first delivery
  - how a live boundary reports a closed subscription to the client, and whether the client is expected to retry it
```
