---
id: requirement:fasthttpbind-parity-scope
type: requirement
title: fasthttpbind Parity Scope
---
Define which parts of the httpbind surface fasthttpbind must reproduce, which need reimplementation rather than adaptation, and which are out of scope for v1.

```yaml
status: proposed 2026-08-08
v1_required:
  - api:fasthttpbind-bind covering query, payload, path, header, cookie, and File fields
  - api:fasthttpbind-write for Write, WriteStatus, and WriteError
  - policy:problem-details bodies byte-identical to httpbind
  - requirement:bind-check-validation, unchanged; it reads bound values, not the transport
  - OpenAPI output identical, since it derives from the field plan and not from the transport
already_transport_neutral:
  htmlbind_render: the runtime writes to io.Writer and its Flush duck-types Flush and Flush error, so a *bufio.Writer from SetBodyStreamWriter already satisfies it
  reading: the HTML rendering half needs no port, only a caller
  scope_effect: filesystem page rendering is closer to portable than the REST half is
needs_reimplementation_not_adaptation:
  status: both done; kept because the reasoning is what the shapes were chosen against
  httpbind_stream: asserted http.Flusher; the fasthttp shape is api:fasthttpbind-stream over SetBodyStreamWriter
  htmlupdate_record_writer: held an http.Flusher field, so the live boundary delivery path was transport-bound in the runtime, not only in generated code; it takes an io.Writer now
  reason: both depend on flushing inside a handler that has not returned, which is precisely what fasthttp inverts
update_surface_split_2026_08_10:
  finding: htmlupdate was not one port but two, and only one of them needed the flusher inversion
  read_only_half: shipped, as decision:update-core-shared-leaf; the twelve entries that read a request and write nothing through it, plus the sending half they return values for
  writing_half: shipped later the same day; see writing_half_shipped_2026_08_10 below
  effect_on_the_deferral_below: 'partial updates, redraw, sequences and CSRF worked on the second backend from this point; live delivery followed'
  recordwriter_note: the recordWriter http.Flusher field was why the writing half was still here, and it was the only reason left
writing_half_shipped_2026_08_10:
  status: implemented; the update surface is whole on both backends
  shape: decision:stream-callback-shape carried through to htmlupdate, on the owner's direction and matching what the downstream framework had already done to its own stream API
  removed: 'OpenStream and OpenLiveStream, the open-then-Close pair'
  added: 'WriteStream and WriteLiveStream taking func(*DeltaStream) error, on both transports'
  removed_not_deprecated: a deprecated held-stream entry is a call site that still compiles and has no counterpart, so the failure moves from the build to the deploy; deleting it puts the error at the one place that can fix it
  one_type_not_two_that_match: 'DeltaStream, ManifestEntry and StreamPlan live in internal/updatecore and both shells alias them; a wrapper renaming one method would make the two handler bodies differ by more than a signature line, which the transform does not rewrite'
  plan_then_run:
    why: fasthttp forbids reading the RequestCtx from a body stream writer, so everything read from the request is captured in a StreamPlan while the handler still owns it
    effect: one loop serves both transports, and the pre-commit failure window exists on both because planning runs in handler scope
  error_routing: post-commit failures go to bindcore's SetStreamErrorHandler, the same installation the typed streams use
  live_termination_as_built:
    signal: a failed record write ends the render, which is the one disconnect signal both transports have
    also_fixed_on_net_http: a client that aborted previously kept the source rendering until the sequence ended; it now stops at the next record
    verified: a live source capped at 500 deliveries delivers at most 8 into a closed socket, and a mutation removing the check makes it deliver all 500
    request_ctx_substitution: 'a *fasthttp.RequestCtx passed as the cancellation context is replaced with one carrying the server shutdown channel, because a rewritten handler reaches the entry with the same identifier in both positions and the delivery outlives the handler'
    residual_difference: fasthttp notices a departed client at the next delivery rather than at the disconnect, and a subscription that never delivers again holds its resources until the server stops; a caller needing a bound passes a context carrying one
writing_half_is_three_sizes_2026_08_10:
  measured: by reading what each entry actually needs, rather than by grouping them under the flusher
  render_is_not_a_streaming_entry:
    fact: 'Options.Render never flushes; delta.CollectChain takes io.Writer and nothing in htmlbind/delta calls Flush, and the delta path buffers into one JSON body'
    consequence: it needs no inversion at all and is portable over ctx.Response.BodyWriter today
    correction: the downstream report and the deferral above both grouped it with the stream entries; that grouping was wrong
  render_stream_is_mechanical:
    needs: recordWriter over io.Writer with a duck-typed flush, and a fasthttp shell wrapping SetBodyStreamWriter
    precedent_built: api:fasthttpbind-stream already does exactly this for WriteStream, including the error routing of decision:stream-callback-shape
  open_stream_and_the_live_path_needed_decisions:
    both_taken_2026_08_10: the API change by the owner, the live termination by the write-failure signal
    api_change: 'OpenStream and OpenLiveStream are open-then-Close, which fasthttp cannot express; the fix is the callback shape already taken for the typed stream, and it is breaking on net/http'
    live_termination:
      fact: 'the fork RequestCtx.Done returns ctx.s.done, closed only on server shutdown; fasthttp has no per-request cancellation'
      what_breaks: 'renderStream ends a live stream on ctx.Err() != nil, which on net/http means the client disconnected, and decides retry over done from it'
      on_fasthttp: 'the only disconnect signal available inside the body stream writer is a write failure on the bufio.Writer, so termination is detected later and the retry-over-done decision has to be made from a different fact'
      character: a behavioural difference to decide, not a transcription to write
      see: rule:transform-rewrite-table seed_caveat_2026_08_10
v1_deferred:
  - the discovered router registry for fasthttp, pending decision:fasthttpbind-generator-backend-selection
  resolved_2026_08_10: live boundary streaming and the streamed navigation, by writing_half_shipped_2026_08_10
server_actions_2026_08_08:
  was: deferred to the adapter side, since they are http.HandlerFunc by definition
  now: decision:backend-build-tag-mode leaves no adapter, so they go through rule:transform-eligibility like any other handler and a refused one fails the build
  consequence: they move from deferred into v1, because there is nowhere else for them to run
out_of_scope:
  - HTTP/2, which fasthttp implements on no toolchain
  - making any deferred item work through decision:fasthttpbind-adapter-boundary and calling it parity
tls:
  host_go: available; the fork is upstream behaviour for behaviour there, so ServeTLS works
  tinygo: impossible, and out of scope by decision:fasthttpbind-tinygo-not-first-class rather than by omission
verification:
  shared_suite: one conformance suite runs the same request set against both backends and compares status, headers, and body bytes
  divergence_policy: a difference is a defect unless it is recorded here or in rule:fasthttpbind-body-limit-mapping
related:
  - requirement:fasthttpbind-product-goals
  - api:fasthttpbind-stream
  - requirement:chain-render-pipeline
  - requirement:component-redraw-endpoint
  - decision:fasthttpbind-runtime-package
```
