---
id: requirement:template-parse-introspection
type: requirement
title: Parse Surface For External Template Tooling
---
Make a parsed template readable from outside: the declared message scope, every message reference, and a source range for the text and attribute values a rewriting tool must replace.

```yaml
source: concept:template-message-surface, request items G and H
review_gate: approved 2026-08-16 by the owner
priority: G blocks catalog reconciliation, H blocks extraction; neither blocks rendering
as_built:
  status: both halves implemented 2026-08-16
  api: MessageRefs(filename, source) returning scope, written id, resolved id, argument names and position per reference
  channel_chosen: a package-level report beside ActionRefs, rather than a generator hook, because a caller needs it before it has anything to pass to Generate
  unresolvable_is_reported_not_refused: a bare reference in a file declaring no scope comes back with an empty ID, so a caller reconciling a tree sees all of it rather than the first mistake
  walker: walkNodeExprs covers element children, attribute values, component arguments, control headers, val and await bindings, and slot defaults
  offsets:
    where: Start and End on TextNode and on AttributePart, file-global through the parser's existing baseOffset
    not_on_position: Position stays Line and Col; putting a range there would have moved every parser fixture and still not carried an end
    serialization: json:"-", so the published AST shape and every golden fixture are unchanged; the range is a tool-facing detail rather than part of the parse's contract
    merged_text: adjacent runs merge into one node, so the range grows to cover everything the merged text came from
    range_is_source_not_content: an escaped brace contributes one character to Text and two to the range, so a rewriter replacing the range replaces the escape too, which is what an extractor wants
    proven_by_a_real_rewrite: TestTextNodeByteRanges and TestAttributeValueByteRanges splice `{t id}` over the reported range and assert the result parses and reports exactly one reference
  tests: templates/htmlbind/message_test.go
consumers:
  reconciliation: undefined-key detection, unused-key reporting, and reconciling templates against catalogs
  extraction: a downstream command that rewrites marked source text into references in place
  both_downstream: each needs the catalog, which this module never reads
references_half:
  expose:
    - the requirement:message-scope-declaration scope, if declared
    - every reference: resolved id, arguments, and requirement:message-hole-binding bindings
  channel: parser output or a generator hook, whichever fits the existing API
  our_reading: this is the same shape requirement:script-block-reporting and requirement:fragment-capability-introspection already have, so the precedent for reporting a parse fact to a caller exists and the question is only which of the two it resembles
offsets_half:
  requested: byte offsets for text nodes and attribute values
  requested_on_the_belief: that parseposition_test.go shows positions already exist internally and only need exposing
  verified_2026_08_16:
    where: templates/internal/syntax/position.go, and the TextNode construction in templates/htmlbind/html.go
    finding: a node carries Position, which is Line and Col only; byte offsets exist during parsing as StartOffset and ContentOffset and are converted to Line and Col and discarded
    therefore: this is not exposure of an existing field. It needs an offset carried onto the node
  second_correction: a rewrite needs a range, not a position; only a start is available even in principle today, so an end offset is a second addition rather than the same one
  serialization_note: Position carries JSON tags, so a new field is additive on the wire but will move any golden output that serializes a node
  still_small: both additions are mechanical, and rule:generation-input-hash and the golden fixtures are the only things that notice
the_alternative_the_request_does_not_consider:
  what: a parse, an edit to the tree, and a print, rather than a byte splice against offsets
  why_it_is_available: templates/htmlbind/print.go exports Printer and RootPrinter, and requirement:template-source-formatting with rule:template-format-fidelity already commits this module to printing a parsed template back
  what_it_would_avoid: the rewriting tool reimplementing quoting, escaping, and attribute-value shape at the byte level, which is where an in-place rewriter goes wrong
  what_it_costs: the printed file is normalized by the formatter, so a project not already formatted sees unrelated diff noise on extraction
  reading: offsets are the right ask if the reporter wants a minimal diff, and the printer is the right ask if it wants correctness; they are not exclusive, and the reporter should be told the printer exists before it builds a splicer
extraction_marks_need_nothing:
  form: '<p i18n>...</p> and <input placeholder="..." i18n="placeholder">'
  why_free: they are ordinary HTML attributes, transient, and removed by the rewrite
  check_worth_making: an unknown attribute passes through today, so the marks survive to the generated output if extraction is not run; that is the reporter's to notice, but a template shipped with a stray i18n attribute renders it
acceptance:
  - a parsed template reports its declared scope and every reference with arguments and hole bindings
  - a text node and an attribute value each report a start and end offset into the original source
  - the reported range, replaced verbatim, produces a file that parses
  - a project using no message feature sees no change in generated output
```
