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
related:
  - policy:problem-details
  - api:write-error
  - requirement:framework-render-entry
  - requirement:component-redraw-endpoint
open_questions:
  - whether a failure raised after the response committed should reach the same hook, given requirement:streaming-delta-response can only report it in band
```
