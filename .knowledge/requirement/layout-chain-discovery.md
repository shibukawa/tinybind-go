---
id: requirement:layout-chain-discovery
type: requirement
title: Layout Chain Discovery
---
Build each page's default wrapper chain from ancestor route directories, with an explicit page override.

```yaml
source: concept:filesystem-html-routing
convention: decision:html-route-file-conventions
default:
  walk: configured route root through page directory
  collect: one layout file per ancestor directory when present
  order: outermost root layout to innermost page-directory layout
override:
  auto: use discovered chain
  none: render page without route layouts
  explicit: replace discovered chain with a named ordered compatible chain
precedence: explicit page setting overrides convention; absence means auto
generation:
  output: data:html-render-route-plan
  validate: requirement:nested-layout-composition slot, types, visibility, order, and cycles
  layout_shape: ordinary component carrying one unnamed requirement:html-slot-syntax slot
  runtime: execute precompiled components only
acceptance:
  - adding an ancestor layout changes descendant default plans deterministically
  - explicit page selection is stable regardless of unrelated filesystem layouts
  - generated diagnostics show page, candidate layout, and failing contract
open_questions:
  - page-level override syntax
  - additive override versus replacement-only first milestone
  - layout inheritance stop marker for a subtree
```
