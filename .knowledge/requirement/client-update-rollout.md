---
id: requirement:client-update-rollout
type: requirement
title: Client Update Rollout Order
---
Sequence the client update extensions so the first shipped milestone adds no template syntax, no server state, and no security surface.

```yaml
source:
  - concept:html-render-runtime-extensions
  - user rollout question 2026-07-31
review_gate: proposed sequencing requires user approval
enabling_invariant:
  rule: every unrecognized condition degrades to an ordinary full browser navigation
  covers: version mismatch, malformed manifest, unexpected response, network failure, unsupported structural change
  consequence: later milestones may be deliberately incomplete without becoming incorrect
  detail: rule:delta-consistency-model
retrofit_cost:
  expensive_later:
    - rule:component-instance-identity scheme and its explicit key authoring surface
    - decision:update-manifest-transport attribute prefix and the single-root-element constraint
    - protocol version presence in the mode header and in validators
  cheap_later:
    - rule:update-validator-computation hash algorithm, keying, and length, because a version bump forces a full render
milestones:
  - id: m0
    status: delivered
    goal: server-side manifest computation with no protocol and no client code
    boundaries: api:render-html-chain members, because filesystem routing does not exist yet; a chain member is the layout chain requirement:layout-reuse-boundaries describes
    included:
      - boundary marker emission per decision:update-manifest-transport
      - rule:component-instance-identity derived from chain position
      - canonical input encoding and frame validators per rule:update-validator-computation
      - single root element as a boundary eligibility rule
    verification: render one chain twice with different search parameters and diff the resulting data:component-update-manifest in a Go test
    deferred: every wire format and browser behavior
    deviations:
      eligibility_not_diagnostic:
        behavior: a component without a single root element is silently not a boundary
        reason: boundaries are automatic at this milestone, so a template that never opted in must keep compiling
        becomes_an_error: at m3, where an explicit update flag that cannot be satisfied deserves a diagnostic
      manifest_order: document order, so a parent precedes its children as requirement:streaming-delta-response ordering needs
  - id: m1
    status: delivered
    goal: first end-to-end slice, limited to a search-parameter change on one route
    why_small: composition does not change, so head, layout frames, structure, and history all stay fixed
    included:
      - requirement:render-mode-negotiation navigation mode on GET with the mode header
      - decision:update-protocol-version constant, echoed and enforced on both sides
      - buffered data:component-delta-response with replace operations only
      - topmost-changed selection, so a replaced ancestor never resends its descendants separately
      - minimal browser runtime exposing an update call, plus history replaceState
    deferred:
      - requirement:delta-head-sync, unnecessary while the composition is fixed
      - insert, remove, move, and retain operations
      - policy:html-update-csrf-protection token, because the request is a side-effect-free GET
      - requirement:streaming-delta-response
      - link and form interception, which belongs with cross-route navigation in m2
    verification: a search-parameter change updates the page boundary without resending layout HTML, and every failure path still lands on the target page
    deviations:
      runtime_delivery:
        behavior: the framework embeds and serves the runtime at a content-hashed URL, and the caller places the script tag
        reason: requirement:static-asset-extraction and requirement:html-runtime-bootstrap injection do not exist yet
      transport_package:
        rule: HTTP negotiation lives beside htmlbind rather than inside it, because decision:runtime-package-boundaries keeps the render runtime free of net/http
      client_coverage:
        tested: header construction, version checking, validator bookkeeping, supersession, and fallback, driven under node against a stubbed DOM
        untested: real DOM insertion, which needs a browser
  - id: m2
    status: delivered except structural operations
    goal: cross-route requirement:client-navigation
    included:
      - requirement:delta-head-sync head installation, title, and stylesheet gating
      - layout frame retention across routes
      - history push and replace, scroll reset and restoration, focus and caret restoration
      - link and GET form interception with an opt-out attribute
      - api:client-navigate lifecycle events
      - api:client-navigate navigate entry
      - rule:form-state-reconciliation and rule:preserved-client-subtree islands, which decision:dom-application-strategy stage 1 needed anyway
      - replacing the outermost boundary when one the browser holds disappears
    deferred:
      - structural operations for keyed lists, which need decision:list-item-key and a matching application strategy
    reason_after_m1: a new route introduces components whose head contributions are absent from the live document
  - id: m3
    status: delivered
    goal: explicitly reloadable components
    reshaped_by: decision:author-declared-boundary-id and requirement:component-redraw-endpoint, which replace derived identity and the delta machinery with an author id and a plain GET endpoint
    included:
      - registration of a reloadable component and its generated kind identity
      - decision:author-declared-boundary-id call-site id argument
      - requirement:component-redraw-endpoint and its typed query decoding
      - rule:redraw-input-trust authorization guidance and review point
      - rule:preserved-client-subtree preserve markers
    not_needed_here:
      - data:component-update-manifest entries, validators, and continuations, none of which a redraw uses
      - data:component-delta-response retain holes, which belong with navigation structural changes
  - id: m4
    status: retired
    was: parameter patches applied during handler re-execution
    reason: decision:boundary-update-execution found the third execution mode unnecessary and its contract misleading; m3 covers the remaining need
  - id: m5
    status: delivered
    goal: requirement:streaming-delta-response
    delivered: record framing, per-record manifest entries, the terminator, incremental application, and the truncation rule
    producer: the async render sequence drives the same records, so a region travels with its fallback and its replacement follows
    seam: the open-stream API was the join, and wiring it took a call per completion rather than a redesign
    two_record_kinds: a boundary operation addresses an instance, a completion addresses a placeholder inside one; they share a stream because both mean a region is ready
  - id: m6
    status: delivered
    goal: requirement:live-reconnect
    delivered: a live stream entry that keeps subscriptions open, and a client that reopens a dropped one with bounded backoff before degrading to a reload
    why_simple: reconnecting is the same request again, because a delivery carries whole state and boundary ids are reproduced by position
  - id: m3a
    status: delivered
    goal: requirement:action-response-update
    note: shipped ahead of m3 because it reuses the m1 response shape and needs neither registration nor a public endpoint
independent:
  requirement:component-output-cache:
    order: any time
    note: until it ships, the input validator is diagnostic only, because rule:update-validator-computation forbids skipping a render on input equality alone
blocking_decisions:
  remaining:
    - decision:list-item-key adoption, before structural operations are worth building
  resolved:
    - the manifest stays out of an inert payload; head travels in the delta body instead
    - a reloadable component is declared with a reloadable modifier after export
    - link interception takes plain same-origin GET only, with a data-tinybind-ignore opt-out
    - single-root element as a boundary eligibility rule, revisited at m3
    - data-attribute prefix defaults to 'tb' and is overridable through data:generator-options DataAttributePrefix
    - header prefix defaults to 'X-Tinybind' and is one configurable knob for both update headers
    - protocol version format and mismatch behavior, in decision:update-protocol-version
  deferrable:
    - manifest header size cap and key distribution in decision:manifest-state-ownership
acceptance:
  - m1 ships without adding any template syntax
  - m1 adds no server-side per-document state and no new token
  - each milestone leaves every unsupported case on the full-navigation path
```
