---
id: requirement:redraw-cache-policy
type: requirement
title: Redraw Cache Policy
---
Let the caller supply the cache policy for a redraw response, because the fixed no-store the handler writes forbids the conditional request this catalog already asked for.

```yaml
priority: should
source:
  - downstream framework runtime ownership report 2026-08-01, against v0.3.0
  - requirement:component-redraw-endpoint response
review_gate: proposed
shipped_today:
  redraw.go: sets the HTML content type, 'Cache-Control: no-store', and the echoed render mode header on every redraw response
  reason_in_code: the URL alone identifies the response and the content is usually per-user, so a shared cache must never hold it
contradiction:
  requirement:component-redraw-endpoint says:
    cache: private by default, because a redraw usually renders per-user content
    conditional: an ETag over the rendered bytes lets an unchanged redraw answer 304, reusing HTTP instead of a bespoke manifest
    acceptance: an unchanged redraw can answer 304
  handler_does: no-store, which forbids the conditional request entirely
  reading: the code took the strictest reading of privacy and dropped the revalidation half; private and no-store are not the same answer
  found_by: the downstream framework, reading the requirement against the handler
not_contested:
  - the content type on every update path
  - the echoed mode header, which requirement:render-mode-negotiation needs so a proxy-substituted body is detectable
  - 'no-store on a delta and immutable on the runtime asset, both of which the reporter calls correct'
ask: the caller supplies the cache policy for the redraw response
constraints:
  - the default stays private rather than shared, per requirement:component-redraw-endpoint and requirement:render-mode-negotiation delta_privacy
  - a redraw response still varies on the render header, so a document request can never be answered from it
  - a caller relaxing the policy takes responsibility for what rule:redraw-input-trust makes attacker-controlled input
acceptance:
  - an unchanged redraw answers 304 to a conditional request
  - a per-user redraw defaults to a policy no shared cache may hold
  - a caller supplying nothing gets a private, revalidatable response rather than today's no-store
as_built:
  shipped: 2026-08-01
  default: 'private, no-cache, plus an ETag over the rendered bytes'
  why_no_cache_not_no_store: no-store would forbid the conditional request the ETag exists for, because a browser that may not keep the bytes can never ask whether they changed
  conditional: If-None-Match is honoured, including the list form and the weak prefix, and an unchanged redraw answers 304 with no body
  vary: the response now varies on the build header, which the fixed no-store had made unnecessary and a cacheable response makes required
  vary_becomes_load_bearing: this policy assumes the URL identifies which component the bytes belong to; requirement:caller-addressed-redraw removes that assumption, so the kind and instance headers join the vary set or a cache may answer one component with another's fragment
  override: Options.RedrawCacheControl
  open_question_resolved: the ETag is computed by the module over the rendered bytes, which is free because the render is already buffered
  keyed: the tag is an HMAC under Options.Key, because a 304 confirms a guess and a redraw usually renders low-entropy per-user content; this is rule:update-validator-computation reasoning arriving at a second surface
  found_while_building: an unkeyed content digest would have made the endpoint a guess-confirmation oracle, which the module already refuses for frame validators and had no reason to accept here
related:
  - requirement:component-redraw-endpoint
  - requirement:render-mode-negotiation
  - requirement:component-output-cache
open_questions:
  - whether the same seam covers the delta and stream paths, whose no-store the reporter accepts
```
