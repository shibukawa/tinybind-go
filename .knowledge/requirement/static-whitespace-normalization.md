---
id: requirement:static-whitespace-normalization
type: requirement
title: Static Whitespace Normalization
---
Strip authoring indentation and newlines out of generated static byte runs without changing how the page renders.

```yaml
source:
  - requirement:html-template-v1
  - user generation-size observation 2026-07-27
review_gate: approved 2026-07-27; implemented 2026-07-27 in templates/htmlbind/whitespace.go
observed: emitted decision:generated-render-plan Static runs reproduce template indentation and newlines verbatim, one leading run per authored line
cost:
  generated_source: every indentation byte becomes a Go string literal byte
  binary: the same bytes ship in the binary, which requirement:tinygo-wasm counts
  response: the same bytes leave on every render
  compression: transfer encoding hides part of the wire cost and none of the source or binary cost
timing:
  when: generation time, in the HTML compiler
  order: after parsing and context classification, before the decision:generated-render-plan static coalescing step
  never: request time; the coordinator writes plan bytes unexamined
scope:
  applies_to:
    - static text nodes from requirement:html-template-v1
    - fixed bytes from requirement:builtin-element-lowering
    - requirement:head-merging contributions and decision:html-document-shell markup, which are emitted as static fragments too
  excludes: rule:whitespace-preserving-contexts contexts
  excludes_runtime:
    - text a typed expression produces, since the value is not known at generation time
    - requirement:explicit-output-control trusted output
tag_interior:
  status: already satisfied
  reason: the emitter writes exactly one space before an attribute name rather than copying the source run, so a start tag never carried authoring whitespace
policy: decision:whitespace-collapse-policy
compatibility:
  baseline: requirement:html-rendering-compatibility preserves rendered body bytes for unchanged templates
  exception: this requirement changes those bytes deliberately; rendered result, not byte identity, is what stays preserved
  affected: requirement:component-output-cache entries, requirement:component-delta-rendering payloads, and any test asserting exact markup
control:
  default: on
  escape: the preserve-whitespace attribute from decision:whitespace-collapse-policy, per subtree
  switch: data:generator-options PreserveTemplateWhitespace, surfaced to the compiler as GenerateOptions.PreserveWhitespace
  hash: Options is marshalled whole into the fingerprint, so rule:generation-input-hash covers the switch without further work
  always: the reserved attribute is stripped even with the switch on, so it never reaches output
acceptance:
  - a component authored with nested indentation emits static runs carrying none of it
  - inline siblings separated by a newline still render with one separating space
  - pre, textarea, script, and style content is byte-identical to source
  - an escaped subtree is byte-identical to source
  - expression holes keep their adjacent separating whitespace
  - two runs over identical source emit identical bytes, per the existing determinism check
  - diagnostics still report original source positions, unshifted by the rewrite
  - enabling the switch reproduces the pre-normalization bytes exactly
measured:
  examples/demo: generated template file 4603 to 4241 bytes, about 8 percent, on a page whose bulk is a preserved pre block plus a style and a script body
  note: a page without those preserved blocks loses proportionally more, since indentation is one run per line
open_questions:
  - whether HTML comment removal belongs in this pass or stays a separate opt-in
  - whether requirement:static-asset-extraction stylesheet output gets its own whitespace treatment
  - whether the droppable position list should grow beyond the table scope, which needs a cited parsing rule per rule:whitespace-preserving-contexts
```
