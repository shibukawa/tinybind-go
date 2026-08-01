---
id: requirement:delta-head-sync
type: requirement
title: Delta Head Synchronization
---
Install head contributions and extracted assets required by a delta before applying the content that depends on them.

```yaml
source:
  - requirement:head-merging
  - requirement:component-delta-rendering
  - user asset-consistency analysis 2026-07-26
review_gate: proposed protocol surface requires user approval
problem:
  - requirement:head-merging runs once before the first body byte of a complete document
  - a delta reuses the live decision:html-document-shell, so a component appearing for the first time has no link tag in the current head
  - applying its HTML first shows unstyled content or a component whose requirement:static-asset-extraction script never loaded
derivation:
  set: the merged contribution set of the new composition, already known at generation time per data:html-render-route-plan
  diff: the runtime compares contribution identity against the live head, so the server may send the whole new set
  identity: element name plus normalized attributes, the same identity requirement:head-merging deduplicates by
operations:
  add: link, script, meta, or title node not present in the live head
  replace_singleton: title and other requirement:head-merging singletons
  retain: an already-present contribution produces no operation
  removal:
    initial_policy: never remove a stylesheet or script contribution
    reason: refcounting contributions across independently updated boundaries is error-prone, and extracted assets are content-hashed and cacheable
    consequence: a long-lived session accumulates head links; scoping in requirement:scoped-component-style keeps that inert
gating:
  stylesheets: wait for load or error of newly added stylesheets before applying content operations that depend on them
  timeout: a configured deadline applies the content anyway rather than stalling the update
  scripts: module URLs load once; a duplicate URL is ignored, and a component script may not assume a fresh execution per instance
  streaming: requirement:streaming-delta-response emits head records before dependent content records
excluded:
  - data:html-client-bootstrap tokens, nonces, and the runtime script tag, which belong to the document and are never re-emitted
  - request-specific values, which stay outside requirement:component-output-cache validators
safety:
  - contributions come from the generated route plan, never from client-supplied URLs
  - inserted nodes keep the escaping of their declaring context
  - a Content Security Policy that forbids inline script still works, because contributions are external references
acceptance:
  - navigating to a page whose component was not previously rendered shows it already styled
  - a component script loaded by an earlier navigation is not re-executed
  - the document title changes on navigation
  - a stylesheet that fails to load does not block the update indefinitely
open_questions:
  - whether script contributions need an explicit per-navigation initialization hook
  - preload or early-hints emission for the assets a likely navigation needs
  - head accumulation bound for very long sessions
  - interaction with a future single-binary embedded-asset mode
```
