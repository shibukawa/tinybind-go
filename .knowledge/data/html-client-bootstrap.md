---
id: data:html-client-bootstrap
type: data
title: HTML Client Bootstrap
---
Per-document runtime configuration injected outside reusable component output.

```yaml
source: requirement:html-runtime-bootstrap
fields:
  protocol_version: generated client and server compatibility
  runtime_asset: generated or configured same-origin module URL and integrity metadata
  update_manifest: initial data:component-update-manifest location or inline safe payload
  csrf:
    token: optional policy:html-update-csrf-protection synchronizer or signed double-submit value
    header: X-CSRF-Token by default
  csp_nonce: optional application-provided nonce for unavoidable inline bootstrap
transport:
  metadata: collision-resistant framework meta names or inert JSON resource
  script: one external module tag preferred
  token_example: '<meta name="tinybind-csrf-token" content="escaped opaque token">'
cache:
  - per-request token and nonce are injected after reusable document or layout frame rendering
  - exclude request-specific bootstrap values from component cache keys and frame content validators
  - a complete response containing user-specific token follows application private-cache or no-store policy
excluded:
  - the decision:update-manifest-transport data-attribute prefix, which the framework-owned runtime hardcodes
safety:
  - HTML-escape all metadata values
  - never write token to URL, logs, localStorage, or sessionStorage
  - same-origin JavaScript can read meta token; XSS can defeat this protection, so preserve CSP and escaping
```
