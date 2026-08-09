---
id: requirement:update-error-hook
type: requirement
title: Update Endpoint Error Hook
---
Report every failure an update endpoint produces to the caller instead of writing a fixed plain-text response, so a redraw failing in production is visible.

```yaml
priority: must
source:
  - downstream framework runtime ownership report 2026-08-01, against v0.3.0
review_gate: proposed
distinguishing: the only item of its round whose workaround is not a copy; there is none, and the information is gone
shipped_today:
  handler: htmlupdate/redraw.go RedrawHandler writes its own failures with http.Error
  cases:
    stale_page: 409, when the build header does not match the running build
    query_too_large: 414, past MaxQueryBytes
    invalid_arguments: 400, when the generated decoder rejects the query
    render_failed: 500, when htmlbind.Render fails
  form: a plain-text body and a status, chosen by the module
what_the_caller_loses:
  reported: RFC 9457 problem responses, application-supplied HTML error pages, request-scoped loggers, and OpenTelemetry spans see none of these
  effect: a redraw failing in production is invisible in the caller's logs
  severity: no wrong content and no leak, but a silent operational blind spot in a request the browser retries
contrast_with_htmlbind:
  htmlbind: sets no status, writes no header, and chooses no encoding, which is why it composes
  htmlupdate: legitimately must write a response, since it owns endpoints
  the_line: writing a response is not the same as deciding what a failure looks like
as_built:
  shipped: 2026-08-08
  body: 'WriteFailure writes application/problem+json with type, title, status, detail, and code, matching what httpbind.WriteError emits so one project has one error format'
  code_carries_the_kind: FailureKind travels as the code field, which is the fact this concept says a caller branches on, rather than being inferred from a status two kinds share
  detail_is_safe_as_is: Message is a constant this package chose and never anything the request supplied, so it needs no blanking at 5xx the way a wrapped cause would
  err_never_travels: verified by TestFailureBodyOmitsTheCause, which registers a component failing with a connection string and asserts the host never reaches the body
  field_level:
    added: htmlupdate.QueryError, carrying the Parameter the decoder refused and the Reason, returned by every Query decoder in place of a formatted string
    surfaces_as: 'one errors entry with field, location query, and the reason, at the location policy:problem-details already lists'
    never_reflects_the_value: TestFailureNamesTheRefusedParameter asserts the refused value does not appear in the response, since it is attacker-supplied
  status_still_directs: unchanged; the body adds diagnosis and a client that cannot read it still falls back on the status alone
  verified: TestFailureDefaultIsProblemDetails, TestFailureNamesTheRefusedParameter, TestFailureBodyOmitsTheCause, plus the existing hook suite
  not_done_here: the streamed in-band failure record, which is after the response commits and therefore cannot carry a status
default_form_2026_08_08:
  decided: the module's own default failure body becomes RFC 9457 problem details, not plain text and not a fourth shape
  by: owner, on the round that moved every update body to JSON
  not_a_new_format: policy:problem-details is already this module's documented default and api:write-error already writes it; the update endpoints are the only paths deviating, which is what this concept's what_the_caller_loses already names first
  downstream_already_speaks_it: the reporter's reproduction of the CSRF failure returned application/problem+json from its own wrapper
  media_type_is_the_discriminator:
    application_json: an update to apply, including a non-2xx one — WriteUpdateStatus returning 422 with the validation errors rendered into a region is a successful update carrying an outcome status, not a failure
    application_problem_json: an update that could not be produced; the client applies nothing and falls back
    why_not_status: the two cases share statuses, so a status-based rule cannot separate them and a client would have to guess
  status_directs_body_describes:
    rule: the existing client rule, that any non-2xx falls back to a full page load, is unchanged; the body adds diagnosis rather than direction
    reason: requirement:client-update-rollout enabling_invariant requires an unrecognized condition to degrade on its own, so a client that cannot parse the body must still land correctly
  before_and_after_commit:
    before: a problem details response, since the status is still choosable
    after: the stream's terminal failure record, since the status is already sent
    already_drawn: requirement:streaming-delta-response commit_consequences draws this line, so nothing new is decided here
  inherits_the_leak_rule: policy:problem-details write_error_behavior already logs the wrapped cause and hides internal detail, so Failure.Err stays out of the response without a rule of its own
  gains: FailureInvalidArguments becomes a structured errors entry naming the parameter, at the query location policy:problem-details error_locations already lists, instead of one line of text
  hook_unchanged: a caller overriding the hook still writes whatever it wants; this changes only what the module writes when it is not overridden
ask:
  options:
    error_hook: a function on Options receiving the request and the failure, letting the caller write the response
    handler_shape: a handler returning an error, so the caller writes the response in its own entry point
  reporter_accepts: either
policy_alignment:
  default: unchanged behavior when no hook is supplied, so a project using the module directly still gets a working response
  house_format: policy:problem-details is this module's own default error format, and api:write-error already resolves a status and hides internal detail
  precedent: requirement:framework-render-entry moved the response entry point for exactly this reason, and decision:framework-integration-seams recorded Symbols.WriteError as the same finding one layer further in
scope:
  covers: every failure path in htmlupdate, not only redraw; a delta, an action, a live reconnect, and a stream error record are the same question
  after_commit: a failure past the first written record stays an in-band error record, per requirement:render-mode-negotiation and requirement:streaming-delta-response
  fallback_unchanged: whatever the caller writes, a non-2xx still leaves the browser on the full-navigation path, per requirement:client-update-rollout
constraints:
  - a caller supplying no hook gets today's bytes
  - the hook never decides whether the request was valid, only how the failure is reported
  - a hook that panics or writes nothing must not leave the response uncommitted
acceptance:
  - a redraw rejected as stale reaches the caller's logger and its trace span
  - a caller answers a redraw failure with its own problem response and its own HTML error page
  - a project supplying no hook sees the current plain-text responses unchanged
as_built:
  shipped: 2026-08-01
  shape: Options.OnFailure, which receives the ResponseWriter, the Request, and a Failure, and writes the response; nil writes what the package wrote before
  failure_value: Kind, Status, Message, Err, KindID, and InstanceID; it satisfies error and unwraps to the cause, so it goes straight to a logger or a span
  kinds: malformed path, unknown component, stale page, arguments too large, invalid arguments, and render failed
  four_o_four_included: the open question below is resolved in favour of including it; an unpublished kind is the version-skew signal, and a sustained rate of it after a deploy has settled is exactly what a caller wants to see
  default_preserved: WriteFailure is exported, so a caller that only wants to observe logs and delegates rather than reimplementing four status codes
  scope: the redraw endpoint, which is the only surface this package owns end to end; every other entry already returns its error to the caller
hook_stops_answering_2026_08_09:
  was: 'OnFailure(w http.ResponseWriter, r *http.Request, failure Failure), which wrote the response, and nil meant the module wrote one'
  now: 'OnFailure(r *http.Request, failure Failure), which observes only'
  where_the_answer_went: the failure travels on the returned Response with its Failure field set, so a caller reads the kind and sends its own error page, or sends the one it was handed
  why: decision:caller-writes-the-response; a hook that writes is a second write path, and the point of the split is that a wrong response has one place it came from
  what_did_not_change: the default body is still RFC 9457 problem details with the kind as the code field
  write_failure_removed: 'WriteFailure(w, failure) wrote a status and a header, which is the thing this round removed everywhere else; FailureResponse survives and a caller sends it with Response.WriteTo'
  default_preserved_differently: 'the as_built note below says WriteFailure is what a caller delegates to; it is now FailureResponse, and a caller raising a refusal of its own still answers in one shape rather than reimplementing five status codes'
  acceptance_reread: a caller answers with its own error page by not sending the Response it was handed, rather than by taking over a writer
  a_constraint_this_dissolves: 'the third constraint below — a hook that panics or writes nothing must not leave the response uncommitted — cannot arise once the hook cannot write'
hook_takes_a_context_2026_08_10:
  was: 'OnFailure(r *http.Request, failure Failure)'
  now: 'OnFailure(ctx context.Context, failure Failure)'
  why: a log line and a span want the trace, the deadline, and what the caller's middleware put there, and none of them want the transport; every use of the parameter this module or its docs ever showed was r.Context()
  what_it_unblocks: the field means the same thing on a backend whose request type is not *http.Request, so requirement:transport-port-surface no longer has to port a callback to port an entry
  caller_migration: one signature edit; a body already reading r.Context() drops the call
related:
  - policy:problem-details
  - api:write-error
  - requirement:framework-render-entry
  - requirement:component-redraw-endpoint
  - decision:caller-writes-the-response
open_questions:
  - whether a failure raised after the response committed should reach the same hook, given requirement:streaming-delta-response can only report it in band
```
