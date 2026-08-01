---
id: rule:html-template-layout
type: rule
title: HTML Template Body Layout
---
Lay out a component body one element per line wherever the whitespace budget allows it, and keep glued markup glued, because in HTML a line break is content.

```yaml
status: implemented 2026-08-02
applies_to: requirement:template-source-formatting
bounded_by: rule:template-format-fidelity html_droppable, html_run_reshaping, and preserve_option
whitespace_budget:
  free: the rule:whitespace-preserving-contexts droppable positions, where a break may be added or removed outright
  reshapeable: any position where the source already has a whitespace run, since decision:whitespace-collapse-policy makes a break and a space the same output byte
  glued: two nodes the source wrote adjacent; no break may be inserted between them, so a run of inline markup stays on one line however long it gets
  frozen: everything rule:whitespace-preserving-contexts preserves verbatim
head:
  form: one tag per line, each child of head at one level in
  why_available: head is a droppable position, so the layout is invisible to the parser rather than merely believed to be harmless
  applies_to: both a head inside a document shell and a requirement:head-contribution-provenance head declared outside one
  same_treatment: html, table, thead, tbody, tfoot, tr, and colgroup, for the same reason
elements:
  one_per_line: a child that is separated from its siblings by existing whitespace starts its own line at the parent's level plus one
  text_only: an element whose children are one short text or expression stays on one line, opening tag, content, and closing tag together
  opening_tag: attributes stay on the tag line while they fit the width; past it, one attribute per line at one level in
  closing_bracket: the > of a broken opening tag goes on the line of the last attribute, not on a line of its own, so the tag does not look like it closed empty
  closing_tag: at the opening tag's indentation when the children were broken out, on the same line when the element stayed on one
  void_and_self_closing: closed exactly as the source spelled it; normalizing <br> and <br/> to one form is a rewrite, not a layout
  doctype: its own line at column one
components_and_slots: a component call is laid out as an element, with its arguments treated as attributes and its fill children as element children
control_flow:
  form: an if or for inside markup indents its branches one level, with else and the closing marker at the opening's indentation
  constraint: the same whitespace budget applies, so a control node written glued between two inline elements stays glued
attributes:
  order: preserved, never sorted; attribute order is authored meaning to a reader even where it is none to the parser
  quotes: normalized to double quotes, escaping any the value contains
  boolean: kept bare, since adding ="" would change the token the parser sees
comments: an html:comment node is placed like an element and is never reflowed internally
raw_text: a script or style body is copied byte for byte, so its own indentation is the author's; only the enclosing tags are placed
preserve_option: with PreserveTemplateWhitespace on, only the free positions are laid out, because reshaping is neutral only while collapse follows it
related:
  - requirement:template-source-formatting
  - rule:template-format-fidelity
  - rule:whitespace-preserving-contexts
  - decision:whitespace-collapse-policy
  - decision:template-formatter-architecture
  - requirement:html-template-v1
```
