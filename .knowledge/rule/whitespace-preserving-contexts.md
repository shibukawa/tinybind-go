---
id: rule:whitespace-preserving-contexts
type: rule
title: Whitespace Preserving Contexts
---
Rewrite static whitespace only where neither the HTML parser nor a reader can observe the difference.

```yaml
source: requirement:static-whitespace-normalization
preserve_verbatim:
  raw_text:
    elements: [script, style]
    reason: a newline is semantic; JavaScript automatic semicolon insertion and line comments depend on it, and rule:template-context-safety already treats these as distinct insertion contexts
    out_of_scope: minifying these bodies is a separate concern with its own parser
  whitespace_significant:
    elements: [pre, textarea]
    extent: the element and every descendant, including nested code and expression holes
    reason: the user agent stylesheet applies white-space: pre, and textarea content is the control's value
    recognition: the compiler knows these names only for this pass; they are otherwise ordinary elements
  escaped: any subtree carrying the decision:whitespace-collapse-policy escape
  trusted_output: requirement:explicit-output-control raw HTML, CSS, and JavaScript pass through unchanged
  runtime_values: text a typed expression produces; this pass rewrites source, never rendered output
css_defeats_inference:
  problem: white-space pre, pre-wrap, pre-line, and break-spaces make any element whitespace-significant, and display inline makes whitespace around any element visible
  evidence: examples/demo/index.tb.html styles a plain div with white-space pre-wrap
  partial_knowledge: requirement:scoped-component-style parses only a component's own style block, never linked sheets, framework CSS, or decision:html-document-shell markup
  conclusion: the compiler cannot prove an element is a block box, so preservation is declared by the author, never inferred from CSS
droppable_entirely:
  meaning: a whitespace-only run the HTML parser never keeps as rendered text may be deleted rather than collapsed
  positions: [html, head, table, thead, tbody, tfoot, tr, colgroup]
  reason: table-scoped character data is foster-parented out of the table, and html and head hold no rendered text
  growth: an allowlist extended only by citing a parsing rule, never by reasoning about default display values
  excluded_deliberately: [ul, ol, dl, select, optgroup, picture, video, audio]
  exclusion_reason: the parser keeps those text nodes; only CSS hides them, which returns to css_defeats_inference
tag_interior:
  status: nothing to do
  reason: the emitter writes one space before each attribute name and never copies the source run, so a start tag carries no authoring whitespace to begin with
never:
  - normalize at request time
  - alter bytes inside a preserved context to save space
  - derive a preserved context from a CSS rule
  - shift the source positions diagnostics report
```
