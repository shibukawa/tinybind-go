---
id: decision:stream-callback-shape
type: decision
title: Stream Entry Point Is A Callback
---
Replace the open-then-defer-Close stream entry with a callback that receives the stream, on both transports, because fasthttp cannot express the current shape and the callback fixes two defects the current shape allows.

```yaml
status: proposed 2026-08-08
supersedes: api:new-stream
specified_by: api:write-stream
applies_to:
  - httpbind
  - fasthttpbind
forcing_constraint:
  fasthttp: ctx.SetBodyStreamWriter registers a callback and the handler returns before any event is written
  effect: NewStream returning a stream the handler then writes into has no fasthttp transcription
  choice_taken: move net/http to the shape fasthttp requires, rather than ship two stream APIs
defects_the_current_shape_allows:
  truncated_json_array:
    fact: Close writes the trailing bracket for the json-array format, so a handler missing defer stream.Close emits an unterminated document
    severity: the client receives a 200 carrying invalid JSON
    fixed_by: the runtime closing the stream, which a callback makes unconditional
  discarded_write_errors:
    fact: the documented usage is '_ = stream.Write(...)', so a mid-stream failure is invisible
    fixed_by: the callback returning error, which gives the runtime somewhere to route it
no_return_value:
  decision: the entry point returns nothing; the callback's error is the runtime's to handle
  why: on fasthttp the callback runs after the handler returned, so an error cannot travel back to handler code
  consequence: handler source is identical on both transports, which is the point
error_routing:
  as_built_2026_08_08: every error from the callback is post-commit and goes to the handler installed with SetStreamErrorHandler, which bindcore owns so one installation covers both transports
  open_failure: a failure to open the stream still becomes a policy:problem-details response, because nothing is committed yet
  dropped_the_pre_commit_window:
    intended: defer the commit so a callback failing before its first event could still produce a Problem response, which would have caught the common case of a query failing before any event
    why_dropped: on fasthttp the callback runs from the body stream writer, after the handler returned and after the status went out, so the window cannot exist there
    reasoning: keeping it on net/http alone would make the same callback source behave differently per transport, and behavioural symmetry is worth more here than the extra window, since source symmetry was the reason for the shape
    cost_accepted: a callback that fails before writing anything still sends 200 followed by an empty stream
    revisit_if: fasthttp gains a way to replace the status after SetBodyStreamWriter is installed, which it has none today
unchanged:
  - rule:stream-content-negotiation, including the query, Accept, and User-Agent order
  - the three formats and their framing
  - Write and Close on Stream[T], which stay the methods the callback calls
  - rule:openapi-streaming-content output
breaking_change:
  accepted: yes, at v0.4.x
  migration: NewStream deprecated for one release, then removed; the mechanical rewrite is wrapping the body and deleting the defer
discovery_impact:
  today: parser CallStreamCreate reads the type argument of NewStream[T](w, r) at index 0
  after: the same index on WriteStream[T]; when the call is spelled without an explicit type argument, the element type is inferred from the closure parameter
  verified_2026_08_08:
    result: inference from the func literal parameter works; discovery recovers the element type from a call spelling no type argument at all
    how: parser instantiatedTypeArgumentName reads the recorded instantiation out of types.Info Instances
    fixture: testdata/stream_writestream, whose call is WriteStream(w, r, func(s *Stream[ChatEvent]) error)
    also_found: the parser keeps its own DefaultConfig of runtime call names, separate from the generator call patterns, and both had to learn WriteStream; a name added to only one is silently undiscovered
  framework_wrappers: generator StreamCreateCall keeps its shape; only the target signature changes
related:
  - concept:streaming
  - api:stream-write
  - api:fasthttpbind-stream
  - decision:fasthttpbind-no-transport-interface
  - decision:transport-neutral-handler
```
