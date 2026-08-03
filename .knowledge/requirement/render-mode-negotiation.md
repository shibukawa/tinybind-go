---
id: requirement:render-mode-negotiation
type: requirement
title: Render Mode Negotiation
---
Select complete-document, navigation-delta, or boundary-delta rendering from an explicit request signal before the response commits.

```yaml
source:
  - concept:html-render-runtime-extensions
  - user render-mode discussion 2026-07-26
review_gate: proposed protocol surface requires user approval
signal:
  form: dedicated request header carrying mode plus client protocol version
  value: 'mode;v=N'; decision:caller-owned-wire-versioning keeps the grammar and makes N the caller's field, parsed and carried through rather than compared
  names: one configurable header prefix yields the render header and the manifest header, so a deployment has one knob rather than several
  endpoints: every framework-owned endpoint shares one configurable path prefix, so routing, caching, and access rules apply to the whole surface at once
  runtime_discovery: the path prefix reaches the browser on the runtime script tag, unlike the header names, which the runtime hardcodes
  default_prefix: 'X-Tinybind'
  manifest_header: carries the data:component-update-manifest hints, capped by decision:manifest-state-ownership; an oversized value is dropped rather than rejected
  default: header absent means complete document, so browsers without the runtime, crawlers, and curl are unaffected
  method: navigation mode is accepted on GET and HEAD only, which keeps it side-effect free and therefore outside policy:html-update-csrf-protection
  rejected_accept_header: shared caches normalize or drop Vary on Accept, and one URL must stay one cacheable document resource
  rejected_query_parameter: a mode in the URL changes canonical, shareable, and logged URLs
modes:
  document: ordinary complete HTML through decision:html-document-shell
  navigation: requirement:component-delta-rendering navigation execution for a path or search-parameter change
  redraw: requirement:component-redraw-endpoint; requirement:caller-addressed-redraw changes this from addressed-by-URL to a request mode on this header, with kind and instance in headers, so the caller chooses the address
  live_reconnect: requirement:live-reconnect resumption of a dropped delivery stream
  action: requirement:action-response-update, where a mutating endpoint returns the regions its action changed
selection:
  timing: resolved before route execution, so requirement:chain-render-pipeline validation can still change status and mode
  version_mismatch: an unknown mode renders a complete document instead of failing; per decision:caller-owned-wire-versioning the module no longer judges the version, so this is a total function on the mode name rather than a comparison
  authorization: mode never relaxes authentication, authorization, typed validation, or request limits
  endpoint: navigation mode targets the same page URL and method as the document; boundary mode targets the generated update endpoint
response_contract:
  header: echo the served mode and server protocol version, so a proxy-substituted body is detectable
  document_mode: unchanged current behavior
  delta_modes: data:component-delta-response, streamed per requirement:streaming-delta-response
  mode_downgrade: a delta request may be answered with a complete document only before the first record is written
redirect_and_error:
  same_origin_redirect: follow normally; the runtime resends the mode header, so the target answers in the same mode
  cross_origin_or_non_route: in-band directive instructing a real browser navigation
  error_status: keep the real HTTP status and let the runtime fall back to full navigation, so server-owned error pages and monitoring stay unchanged
  after_commit: requirement:streaming-delta-response in-band error record only
caching:
  vary: every generated response varies on the mode header, and a redraw additionally on the kind and instance headers once requirement:caller-addressed-redraw stops the URL identifying which component the bytes belong to
  delta_privacy: delta responses default to private or no-store, because they carry per-document validators and tokens
  never_share: a delta body must never be served to a document request, and a shared cache that cannot vary must be bypassed
observability: log the served mode, so delta traffic is distinguishable from document traffic
acceptance:
  - a request without the header is byte-identical to current behavior
  - the same URL serves a document to a crawler and a delta to the runtime without cache cross-contamination
  - an incompatible client protocol version receives a working complete document
  - a delta request answered by an intermediary cache with a document body is detected rather than misapplied
open_questions:
  - record media type and framing shared with requirement:suspense-html-streaming
  - policy for conditional requests and ETag on delta responses
```
