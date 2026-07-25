---
id: requirement:nested-layout-composition
type: requirement
title: Nested Layout Composition
---
Compose a page through a generated or runtime-selected ordered layout chain while keeping every template precompiled.

```yaml
source: concept:html-render-runtime-extensions
model:
  chain_order: outermost layout to innermost layout to page
  layout_contract: each layout exposes one typed html content slot through requirement:html-slot-syntax
  insertion: page fills innermost slot; each completed child fills its parent slot
  selection:
    convention: requirement:layout-chain-discovery
    explicit_page_override: page selects auto, none, or an explicit compatible layout chain
    runtime_override: optional advanced application-supplied data:html-render-route-plan
  execution: generated render functions compose through typed html continuations or equivalent generated callbacks
  pipeline: requirement:chain-render-pipeline classifies and runs the assembled chain
constraints:
  - no runtime parsing or string evaluation
  - validate component existence, slot compatibility, missing slots, duplicate slots, and cycles before response commitment
  - preserve requirement:html-rendering-compatibility and rule:template-context-safety
  - layout and page parameters remain statically typed
  - convention and explicit chains resolve deterministically at generation time
acceptance:
  - the same compiled page can run under different valid parent chains
  - parent layout markup precedes and follows child output at its declared slot
  - a chain of zero layouts renders the page through the existing path
resolved:
  layout_declaration: ordinary component with one unnamed requirement:html-slot-syntax slot; no distinct layout keyword
open_questions:
  - explicit page layout syntax
  - cross-package layout discovery and visibility
```
