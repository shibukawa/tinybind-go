---
id: decision:component-style-delivery
type: decision
title: Component Style Delivery
---
Author styles inline in the component file, then extract them into a generated stylesheet the document links.

```yaml
source:
  - requirement:scoped-component-style
  - user delivery decision 2026-07-25
review_gate: approved 2026-07-25
authoring:
  form: style block inside the component file, the single-file component shape
  not_react_import: policy:frontend-convention-alignment allows this divergence because a template file, not a JS module, is the authoring surface
  benefit: markup and its styles stay in one file, so requirement:scoped-component-style can rewrite both sides together
delivery:
  mechanism: requirement:static-asset-extraction owns file emission, naming, and the head reference
  form: one stylesheet artifact referenced by one link element
  not_inline: a hoisted inline style block would resend every byte on every response and defeat client caching
  granularity: one bundle per generation unit in the first milestone
head_effect:
  contributes: one link element per generation unit
  static: the link is known without inspecting the composition, so it needs no per-request collection
  remaining_merge: requirement:head-merging still collects title, meta, and script contributions
tradeoff:
  cost: the bundle ships rules for components a given response never renders
  gain: one cacheable request replaces repeated inline bytes across every page
  deferred: per-route or per-chain splitting once payload measurement justifies it
open_questions:
  - cache-busting behavior when only one component style changes in a shared bundle
  - source map or original-position reporting for extracted styles
```
