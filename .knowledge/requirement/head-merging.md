---
id: requirement:head-merging
type: requirement
title: Document Head Merging
---
Let any component declare head content and merge every reachable contribution into the single root document head.

```yaml
source:
  - decision:html-document-shell
  - user lifecycle decision 2026-07-25
model:
  root: only decision:html-document-shell emits doctype, html, head, and body
  contributor: any component may declare a head element holding metadata, style, and script links
  effect: a contributed head never appears at the component position; it is hoisted into the root head
allowed_content:
  - link, meta, style, script, and title nodes
  - nothing that belongs in the body
  - style and inline script blocks are authored here but leave through requirement:static-asset-extraction rather than merging inline
static_requirement:
  rule: head contributions are statically known markup, not values computed from request data
  reason: the root head must be written before body streaming, so contributions cannot wait for render results
  dynamic_values: attribute expressions are allowed; which nodes exist is not conditional on request data
collection:
  static_composition: the generation-time call graph yields the contribution set for a fixed composition
  runtime_composition: a slot filled at runtime carries its contributions on its decision:generated-render-plan component value
  timing: the coordinator merges before writing the root head, so no body byte is buffered
  async: a contributor reachable only through an await boundary still contributes upfront, because its markup is static
merge:
  order: root contributions first, then contributors in deterministic composition order
  dedup: identical nodes collapse to one; identity uses element name and normalized attributes
  singleton:
    title: the innermost contributor wins; the root value is the default
    charset_and_viewport: exactly one survives; a conflicting value is a generation error
  bootstrap: requirement:html-runtime-bootstrap injects after merging and is never deduplicated away
  partial_update: requirement:delta-head-sync installs contributions a delta introduces, because a delta reuses the already-written head
  extracted_assets: requirement:static-asset-extraction replaces style and inline script content with link and script reference tags before merging
constraints:
  - merged nodes keep rule:template-context-safety escaping from their declaring context
  - a head declaration outside a document composition is a generation error
  - merging never reorders nodes within one contributor
  - a contribution cannot be added after the root head is written
acceptance:
  - a leaf component can ship its own stylesheet link without the page or layout knowing
  - two components declaring the same stylesheet emit one link
  - the root head is written before any body byte, so streaming is unaffected
  - a conflicting charset fails at generation time rather than emitting two values
open_questions:
  - whether a component may declare a head contribution that only applies when it renders
  - contribution ordering guarantees users may rely on for cascade-sensitive stylesheets
  - deduplication identity for script nodes carrying inline content
```
