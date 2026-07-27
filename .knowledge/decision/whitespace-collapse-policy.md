---
id: decision:whitespace-collapse-policy
type: decision
title: Whitespace Collapse Policy
---
Collapse a whitespace run to one space instead of deleting it, and delete only where the HTML parser itself discards it.

```yaml
source:
  - requirement:static-whitespace-normalization
  - user removal proposal 2026-07-27
review_gate: approved 2026-07-27; implemented 2026-07-27
problem: removing whitespace outside pre is not rendering-neutral, because a run between two inline boxes renders as exactly one space
forcing_case: two span siblings separated by a newline render as "a b"; deleting the newline renders "ab"
forcing_case_second: display and white-space are CSS properties, so no compiler pass can prove a given element is a block box and its surrounding whitespace invisible
rules:
  collapse: a maximal run of space, tab, carriage return, line feed, and form feed in a normalizable text position becomes one U+0020
  drop: a whitespace-only run in a rule:whitespace-preserving-contexts droppable position is removed
  preserve: everything rule:whitespace-preserving-contexts names is copied byte for byte
  edges: a collapsed space kept at the start or end of a block box is trimmed by the user agent at line-box layout, so no extra edge handling is needed
  document_body: a component body containing a doctype or an html element drops its outer runs entirely, because the parser discards whitespace before the doctype and around the html element; a fragment body keeps one space instead, since its caller may place it between two inline boxes
  silent_siblings: a whitespace-only run touching a sibling that writes no bytes is dropped, because collapsing both sides of a lifted-out node would emit two spaces where the source had one break
  silent_nodes: a requirement:head-merging contribution, and a named requirement:html-slot-syntax fill template under a component call
saving: authoring indentation is one run per line, so a typical line loses its whole leading indent and keeps one byte
rejected:
  full_removal:
    proposal: delete every whitespace run outside pre
    why_rejected: silently changes rendering wherever inline formatting meets authored line breaks, and the failure is invisible in generated Go and only shows in the browser
  css_driven_preservation:
    proposal: read white-space declarations to decide which elements to preserve
    why_rejected: rule:whitespace-preserving-contexts partial_knowledge; correctness would depend on stylesheets the compiler never sees
  runtime_normalization:
    proposal: collapse while writing the response
    why_rejected: costs per request forever, defeats decision:generated-render-plan static byte runs, and grows requirement:tinygo-wasm binaries instead of shrinking them
escape:
  form: the reserved boolean attribute preserve-whitespace on an element, preserving that element and its descendants
  valued_form: rejected with a generation error, because preserve-whitespace="false" reads as a disable yet would enable preservation
  reserved: the name is never emitted into output, matching how requirement:html-template-v1 reserves the slot element
  reason_over_annotation: decision:template-annotation-syntax attaches to declarations, so it cannot mark the one div a stylesheet made whitespace-significant
  covers: the examples/demo pre-wrap div, ASCII art, and generated markup fed to a whitespace-sensitive client script
whole_run_switch: a data:generator-options field, for a project that wants byte-identical output against pre-existing golden files
open_questions:
  - whether a declaration-level decision:template-annotation-syntax annotation is also wanted for a whole component
  - whether the unnamespaced attribute name should move under a reserved prefix if a framework attribute ever collides
```
