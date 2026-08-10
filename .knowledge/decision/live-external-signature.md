---
id: decision:live-external-signature
type: decision
title: Live External Signature
---
Declare a live source as an external returning iter.Seq2 over its value type, with a required leading context, so one Go shape covers a ticker, a fan-out subscription, and a channel an existing service already returns.

```yaml
source:
  - concept:live-boundary-updates
  - requirement:async-external-functions
  - user iter.Seq2 proposal 2026-07-30
review_gate: proposed
status: shipped. `external live` parses, the mandatory leading context is enforced by the generated call shape, the sequence-ending-is-terminal rule and the failure-delivery rule both hold, and the pull-based coalescing falls out of ranging the sequence as designed.
declaration:
  shape: `external live WatchMetrics(id: string): MetricPoint`
  placement: `live` follows the `external` keyword, beside `async`, because it changes the required Go signature rather than annotating behavior, which is the decision:template-annotation-syntax test requirement:async-external-functions already applied
  ambiguity: none, because an external name must be PascalCase
go_signature:
  live: func Name(ctx context.Context, args...) iter.Seq2[T, error]
  async: func Name(args...) (Result, error), unchanged
  sync: func Name(args...) Result, unchanged
  result_type: the sequence element type is the declared template result type; the second position is the error
  no_slice_confusion: a live source declared to yield `MetricPoint[]` yields whole slices, one per delivery, which is the decision:live-boundary-syntax snapshot model rather than one element per delivery
context_required:
  decided: the leading context.Context is mandatory for a live source, unlike the optional context of an async external
  reason: an endless source cannot be abandoned the way a blocking async call can; without a context the pull loop has nothing to make the source return and the goroutine outlives the subscription
  detection: no source inspection is needed, because the parameter is part of the declared shape rather than an option the implementation chooses
  diagnostic: a live external whose Go function omits the context is an ordinary Go compile error at the generated call site
why_seq2:
  symmetry: decision:async-component-signature already returns iter.Seq2[Content, error], so a live response is the same shape that does not end, which is the property the user asked for
  backpressure: a pull sequence blocks the source in its own yield until the renderer is ready, so a fast source cannot queue deliveries the screen will never show
  natural_coalescing: a ticker source blocked in yield simply misses ticks, which is exactly the rule:live-boundary-delivery latest-wins behavior with no buffer to size and nothing to discard
  several_sources: the same property extends to a multi-binding clause, because the runtime serializes deliveries and a source blocks in its own yield while another is rendering
  termination: the sequence ending is one unambiguous end signal, where a channel needs a closed-versus-cancelled convention
  stop: the consumer's break is the stop signal, so cancelling a subscription is the loop the runtime already writes
  no_new_type: nothing in the module's public surface has to name a subscription object
channel_adapters:
  rule: the module exposes no channel-taking form; a service that already returns a channel is adapted by ranging it inside the returned sequence
  reason: this repeats the requirement:awaitable-parameters no_channel_constructor argument, which is that adopting a goroutine the package did not start reopens who closes it and who recovers its panic
  cost: a few lines per adapter, accepted because the adapter is where the service's own close and cancellation convention belongs
failure_delivery:
  model: a yielded non-nil error is a delivery of a failure, not the end of the source; the sequence ending is the only terminal signal
  third_kind: proposed decision:signal-in-the-error-slot makes the error slot carry a signal as well, classified ahead of this path, so a yielded error is one of failure, signal, or cancellation rather than one of failure or cancellation
  transient: the boundary renders the decision:live-boundary-syntax recover subtree from data:async-render-error, and a later value replaces it with primary content again
  terminal: the sequence returning leaves whatever the boundary last rendered and ends the subscription
  no_recover_clause: the decision:async-boundary-syntax omitted_recover rule applies per delivery, so the failure leaves the boundary and reaches the caller, and the subscription ends
  panic: the runtime recovers a panic from the source's own goroutine and normalizes it as data:async-render-error, as requirement:async-external-functions does
  normalization: returned error, recovered panic, and configured per-delivery timeout all become data:async-render-error, so the recover subtree reads one type
execution:
  goroutine: the runtime ranges each live source in its own goroutine and forwards deliveries to the response coordinator; source goroutines never write the response, which is the requirement:async-external-functions constraint unchanged
  invocation: one invocation per live boundary instance, so two boundaries binding the same source are two subscriptions
  fan_out: sharing one upstream across clients is the application's job inside the source, because the module renders per client and owns no broadcast topology; requirement:live-boundary-lifecycle carries the cost
  first_delivery: the boundary commits its fallback without waiting, so a source that takes seconds to produce its first value delays nothing
cancellation:
  client_gone: the request context cancels, the pull loop breaks, and the source observes the cancellation through the context it was given
  consumer_stop: a caller that stops ranging the response sequence cancels every subscription it owns
  boundary_replaced: a live boundary discarded because an enclosing boundary re-rendered has its subscription cancelled before the replacement renders
  no_recover_output: expected cancellation produces no recover content, which is the decision:async-boundary-syntax rule unchanged
deferred_caller_supplied:
  shape: a `live T` template type modifier and a Go handle, so a caller starts the watch itself and passes it in, the way requirement:awaitable-parameters does for a settle-once value
  handle_name: not Stream, because concept:streaming already owns that noun for typed JSON event delivery; a distinct noun is required if this ships
  why_deferred: the pull sequence covers the declared-source case, and a caller-supplied endless source raises reuse questions a settle-once handle does not, because two boundaries cannot both consume one sequence
open_questions:
  - whether a per-delivery render timeout is configured on the declaration, on the render options, or not at all
  - whether a source may declare a minimum interval the runtime enforces, so an event-paced source cannot render faster than the screen can use
  - whether generation should diagnose an await clause inside a live primary subtree, which re-runs the awaited work per delivery
```
