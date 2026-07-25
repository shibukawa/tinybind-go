---
id: requirement:suspense-html-streaming
type: requirement
title: Async Boundary HTML Streaming
---
Stream pending HTML immediately, then replace it with either resolved content or recover content.

```yaml
source: concept:html-render-runtime-extensions
runtime_flow: flow:suspense-html-render
syntax: decision:async-boundary-syntax
boundary:
  primary: component subtree that may read requirement:async-external-functions results
  fallback: synchronously renderable HTML subtree
  recover: synchronously renderable failure subtree receiving data:async-render-error
initial_response:
  - render a unique placeholder element or boundary marker containing fallback output
  - omit Content-Length and flush available bytes when transport and encoding support it
  - rely on net/http streaming semantics; do not require HTTP/1.1 chunk framing on every protocol
completion:
  success:
    - render resolved primary subtree into an isolated buffer with normal HTML context checks
    - serialize it in a uniquely identified template element
    - append an inert update record consumed by requirement:html-runtime-bootstrap instead of requiring per-update inline script
  error:
    - normalize returned error, panic, or timeout as data:async-render-error
    - render the recover subtree into an isolated checked buffer
    - replace the same placeholder through the fixed bootstrapped update runtime
  common: serialize response writes through one coordinator
ordering:
  - replacements may be sent in completion order
  - each boundary updates at most once
  - exported renderer remains active until all request-owned boundaries finish or cancel
safety:
  - generated IDs are unique, opaque, and safe for HTML and script use
  - resolved content is emitted as template content, never interpolated into script source
  - recover content receives only safe public error fields; raw Go errors remain server-side
  - update helper is fixed trusted runtime code loaded through requirement:html-runtime-bootstrap
  - fallback and resolved primary trees follow rule:template-context-safety
  - flushing remains correct when the caller wraps the writer in a compressing encoder
failure:
  before_commit: return the existing rendering error path
  after_fallback_commit: render recover update; if recovery rendering fails, keep fallback and apply outer or server policy
  http_status: once fallback commits the response, failure cannot change the already-sent status; report through recover UI and server observability
  cancellation: do not render recover UI for expected request cancellation or superseded boundary revision
acceptance:
  - a slow external does not delay the initial fallback bytes
  - success appends primary template and update instructions without rewriting earlier bytes
  - async failure replaces fallback with recover content without exposing internal error details
  - client disconnect cancels pending request work
open_questions:
  - exact placeholder markup and update helper delivery
  - Content Security Policy nonce or external-script integration
  - default behavior when recover clause is omitted
  - multiple dependency failure selection and aggregation
  - browser behavior without JavaScript
```
