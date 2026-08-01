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
  - proposed requirement:live-boundary-rendering is the one boundary kind that updates repeatedly, and it does so on a separate request rather than by extending this one
safety:
  - generated IDs are unique, opaque, and safe for HTML and script use
  - proposed requirement:client-managed-head raises that uniqueness to the document lifetime, because a navigation inserts boundaries into a document that may still hold earlier ones
  - resolved content is emitted as template content, never interpolated into script source
  - recover content receives only safe public error fields; raw Go errors remain server-side
  - update helper is fixed trusted runtime code loaded through requirement:html-runtime-bootstrap
  - fallback and resolved primary trees follow rule:template-context-safety
  - flushing remains correct when the caller wraps the writer in a compressing encoder
failure:
  before_commit: yield zero data:async-boundary-content with the error and end the sequence
  after_fallback_commit: yield recover content; if recovery rendering fails, keep fallback and apply outer or server policy; a clause with no recover subtree yields the unrecovered failure and ends the sequence
  http_status: once fallback commits the response, failure cannot change the already-sent status; report through recover UI and server observability
  cancellation: do not yield recover content for expected request cancellation or superseded boundary revision
acceptance:
  - a slow external does not delay the initial fallback bytes
  - success yields primary content the caller appends without rewriting earlier bytes
  - async failure replaces fallback with recover content without exposing internal error details
  - client disconnect or early consumer stop cancels pending request work
markup:
  placeholder: one custom element carrying the opaque boundary ID and holding the fallback subtree, laid out transparently so it adds no box
  completion: an inert template element referencing the same boundary ID, written after the initial document; the caller writes this framing around the fragment the module yields
  commit_marker: an empty custom element written immediately after the template's closing tag, naming the same boundary ID
  terminal_marker: one inert element written as the last bytes of the response when the sequence exits, naming whether anything more is coming, per rule:stream-termination-marker
  runtime: a client script defining the marker element and applying a boundary from its connected callback; decision:client-runtime-ownership makes supplying it the caller's responsibility instead of an api:render-html-chain prepend, and makes the marker rule a normative protocol requirement rather than a property of one bundled script
  no_runtime: a response whose client never loads that script keeps its committed fallback as the final content
commit_marker_rationale:
  problem: an HTML parser inserts an element when it reads the start tag, so a runtime watching for the template could read one whose content had not arrived
  observed: with the template start tag delivered in its own network chunk, a mutation-observer runtime replaced the placeholder with empty content and removed the template, losing the fallback as well as the result
  invisible_in_development: a small completion arrives in one chunk and parses in one task, so the failure only appears once a proxy, TLS record, or compressing encoder splits the bytes
  fix: drive the swap from a marker that follows the closing tag in the byte stream, so the template is complete however the bytes were chunked
  promptness: the marker's connected callback runs during parsing, so the swap is as immediate as an inline script would be
  csp: the completion chunk still carries no script, so no nonce and no unsafe-inline is required
  truncated_stream: a completion whose marker never arrives is simply not applied, leaving the committed fallback
  truncation_is_invisible: the page cannot detect that outcome, because a truncated chunked document parses to end of file and fires DOMContentLoaded and load with no error; rule:stream-termination-marker adds the terminal marker precisely so its absence is the signal
no_javascript:
  behavior: the committed fallback stays visible and completions are inert templates
  alternative: the sync entry in decision:async-component-signature renders the same template settled, for callers that must serve non-JavaScript clients
recover_omitted: decision:async-boundary-syntax ends the sequence with the unrecovered failure carrying the committed placeholder's boundary ID; the fallback stays on screen until the caller's runtime replaces the document, and the render error hook still sees the original error
multiple_dependencies: the first failing binding of one clause decides the boundary; siblings are cancelled and not aggregated
open_questions:
  - Content Security Policy nonce or external-script integration
  - serving the update runtime as a cacheable external module instead of an inline head script
```
