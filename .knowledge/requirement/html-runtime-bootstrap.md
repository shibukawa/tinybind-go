---
id: requirement:html-runtime-bootstrap
type: requirement
title: HTML Runtime Bootstrap
---
Inject exactly one capability-selected browser runtime into every full document that needs streaming or partial updates.

```yaml
source: concept:html-render-runtime-extensions
owner: decision:html-document-shell
input:
  route: data:html-render-route-plan
  capabilities: data:component-render-capabilities reachable from document layouts and page
  state: data:html-client-bootstrap
selection:
  no_client_features: omit runtime script and update metadata
  async_boundary: include streamed-template observer and replacement runtime
  partial_update: include manifest, delta application, and api:client-component-update redraw runtime
  client_navigation: include requirement:client-navigation interception, history handling, and api:client-navigate
  shared_consumer: delta records and async completions use one record consumer, per requirement:streaming-delta-response
implementation:
  owner: each framework ships its own browser runtime; the generator does not synthesize update logic per project
  hardcoded: protocol details including the decision:update-manifest-transport prefix are compiled into that runtime rather than negotiated
  compatibility: the protocol version in the mode header remains the only negotiated axis
  script_invoked_action: emit the CSRF token metadata even when no async or partial update capability is present, because a decision:server-action-lowering element posts from script and cannot read a hidden form field
  script_free_mode: emit neither the runtime script nor the token metadata under decision:script-free-render-mode, where the hidden field carries the token instead
  async_boundary: include the boundary replacement runtime, which per decision:client-runtime-ownership the caller supplies rather than an api:render-html-chain entry point
  partial_update: include manifest, delta application, and api:client-component-update runtime
injection:
  - emit one same-origin external module script after document content or at validated body-end slot
  - emit collision-resistant metadata in head only when required
  - prefer inert template or JSON update records over repeated inline scripts
  - avoid global function names; expose one namespaced module API
csrf:
  - read optional escaped meta token from data:html-client-bootstrap
  - attach it as policy:html-update-csrf-protection custom header to protected update requests
  - accept refreshed token only from same-origin validated response metadata or header
compatibility:
  - full static HTML without client features remains script-free
  - clients without JavaScript retain ordinary full navigation behavior
  - existing non-route exported component rendering does not gain document tags automatically
acceptance:
  - nested layouts and pages cannot duplicate runtime tags
  - all emitted update records are processed by the matching protocol version
  - CSP can allow the runtime without unsafe-inline under the preferred external-module path
open_questions:
  - runtime asset serving and versioned URL API
  - exact meta namespace and manifest transport
  - token refresh signaling and CSP nonce provider API
```
