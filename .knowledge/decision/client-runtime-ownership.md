---
id: decision:client-runtime-ownership
type: decision
title: Client Runtime Ownership
---
Keep the wire protocol in the module and move the browser script that implements it to the caller, because streaming completion and navigation replacement need different client logic.

```yaml
source:
  - requirement:suspense-html-streaming
  - api:render-html-chain
  - user lifecycle discussion 2026-07-27
review_gate: proposed; requires user approval
problem:
  hardcoded: api:render-html-chain async entries prepend a fixed update runtime, so every async response carries the module's own script
  mismatch: that script serves the initial streaming completion only; a page navigation replaces existing content through requirement:component-delta-rendering, which is a different mechanism
  extensibility: a framework wanting its own navigation behavior cannot replace a script the entry point injects
split:
  module_owns:
    - boundary identifier generation, uniqueness, and opacity
    - the data:async-boundary-content sequence of id and rendered html
    - the placeholder element written during the initial pass
    - the normative protocol contract the client script must implement
    - the guarantee that no completion chunk carries script, so CSP needs no nonce
  caller_owns:
    - the browser script implementing the protocol
    - the choice of when to include it, using requirement:fragment-capability-introspection
    - injection, through requirement:render-time-script-contribution rather than an entry-point prepend
    - any navigation or history behavior layered on top
  already_true: data:async-boundary-content already excludes transport framing, so the caller already wraps id and html into the template element and update record
protocol_modes:
  streaming_completion:
    when: the initial response, while the document is still parsing
    transport: template element plus the commit marker element that follows its closing tag
    normative: the swap is triggered by the marker, which the byte stream places after the template's closing tag; triggering on the template itself is forbidden
    what_broke: requirement:suspense-html-streaming records an observed failure from watching the template, whose start tag can arrive in its own chunk; an observer then swapped in empty content and destroyed the fallback
    visibility: the bug only appears once a proxy, TLS record, or compressing encoder splits the bytes, so it survives development
    not_the_mechanism: the failure was the trigger source, not the observation API; a script watching the marker with a mutation observer is conforming, because the marker cannot appear before the template is complete
    recommended_driver: the marker element connected callback, which runs during parsing and is therefore as prompt as an inline script; an observer is correct but one microtask later
    consequence: the protocol contract states the trigger-source rule normatively, so a caller-written script cannot reintroduce the failure by accident whatever API it uses
  navigation_delta:
    when: a page transition after the document is live
    transport: data:component-delta-response operations, not a byte stream being parsed
    driver: ordinary script applying operations to existing DOM
    identity: rule:component-instance-identity, not boundary placeholders
    distinct_logic: no parser is running, so there is no marker to connect and nothing to observe; the two modes share only the boundary identity vocabulary
    head: requirement:client-managed-head makes retiring and installing head tags part of this mode, which the streaming mode never has to do because the shell wrote the head
  consequence: one script cannot be assumed to cover both, which is the concrete reason the module stops shipping the runtime automatically
reference_runtime:
  provided: the module still ships a conforming streaming client as an opt-in requirement:framework-script-contribution registration
  not_automatic: it is registered and selected like any other contribution, never prepended by an entry point
  reason: a project using the module directly should not have to write protocol code, while a framework must be able to replace it
  versioning: the protocol contract carries a version so a mismatched script fails loudly rather than silently misapplying
migration:
  supersedes: the api:render-html-chain bootstrap prepend and the requirement:html-runtime-bootstrap automatic selection for the streaming case
  unchanged: requirement:html-runtime-bootstrap keeps owning bootstrap metadata such as protocol version and data:html-client-bootstrap values
  breaking: an async render with no selected script commits its fallback and never applies completions
  diagnostic: rendering an async chain with no client script selected is reported through the render error hook rather than failing the response, because the fallback is still valid output
constraints:
  - the module never generates script text into a response
  - the protocol contract is documentation plus a version constant, not a Go interface the caller implements
  - a no-JavaScript client still sees committed fallback content, per requirement:suspense-html-streaming
rationale:
  - the caller already owns framing, so owning the script that consumes it removes the last asymmetry
  - the two protocol modes have different drivers, so bundling one runtime into the async entry hardcoded a choice that does not generalize
  - moving injection to requirement:render-time-script-contribution means one mechanism covers module, framework, and application scripts alike
open_questions:
  - whether the protocol contract lives in documentation, a versioned schema file, or a conformance test suite
  - whether the reference runtime covers navigation delta as a second opt-in registration
  - how the protocol version is surfaced to the client script, given data:html-client-bootstrap already carries one
```
