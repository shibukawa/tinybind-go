---
id: requirement:suspense-html-streaming
type: requirement
title: Await Boundary HTML Streaming
---
Stream pending HTML immediately, then replace it with either resolved content or recover content.

```yaml
source: concept:html-render-runtime-extensions
runtime_flow: flow:suspense-html-render
syntax: decision:async-boundary-syntax
signature: decision:async-component-signature
boundary:
  primary: component subtree that may read requirement:async-external-functions results
  fallback: synchronously renderable HTML subtree
  recover: synchronously renderable failure subtree receiving data:async-render-error
initial_response:
  - write a unique placeholder element or boundary marker containing fallback output to the io.Writer argument
  - omit Content-Length and flush available bytes when transport and encoding support it
  - flushing goes through the api:render-html-chain writer check, because writer mode has no HTTP handle
  - rely on net/http streaming semantics; do not require HTTP/1.1 chunk framing on every protocol
completion:
  success:
    - render resolved primary subtree into an isolated buffer with normal HTML context checks
    - yield data:async-boundary-content holding the boundary ID and that fragment
    - caller serializes it in a uniquely identified template element
    - caller appends an inert update record consumed by requirement:html-runtime-bootstrap instead of requiring per-update inline script
  error:
    - normalize returned error, panic, or timeout as data:async-render-error
    - render the recover subtree into an isolated checked buffer
    - yield it as data:async-boundary-content for the same boundary ID
    - caller replaces the same placeholder through the fixed bootstrapped update runtime
  common: only the ranging caller writes the response; goroutines send completions to one coordinator
  chain: requirement:chain-render-pipeline merges boundaries from every chain member into one chunked stream
ordering:
  - replacements may be yielded in completion order
  - each boundary updates at most once
  - the returned sequence stays open until all request-owned boundaries finish or cancel
safety:
  - generated IDs are unique, opaque, and safe for HTML and script use
  - resolved content is emitted as template content, never interpolated into script source
  - recover content receives only safe public error fields; raw Go errors remain server-side
  - update helper is fixed trusted runtime code loaded through requirement:html-runtime-bootstrap
  - fallback and resolved primary trees follow rule:template-context-safety
  - flushing remains correct when the caller wraps the writer in a compressing encoder
failure:
  before_commit: yield zero data:async-boundary-content with the error and end the sequence
  after_fallback_commit: yield recover content; if recovery rendering fails, keep fallback and apply outer or server policy
  http_status: once fallback commits the response, failure cannot change the already-sent status; report through recover UI and server observability
  cancellation: do not yield recover content for expected request cancellation or superseded boundary revision
acceptance:
  - a slow external does not delay the initial fallback bytes
  - success yields primary content the caller appends without rewriting earlier bytes
  - async failure replaces fallback with recover content without exposing internal error details
  - client disconnect or early consumer stop cancels pending request work
open_questions:
  - exact placeholder markup and update helper delivery
  - Content Security Policy nonce or external-script integration
  - default behavior when recover clause is omitted
  - multiple dependency failure selection and aggregation
  - browser behavior without JavaScript
```
