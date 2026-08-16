---
id: decision:message-hole-lowering
type: decision
title: How A Message Hole Is Lowered
---
Three shapes for requirement:message-hole-binding, differing in who drives the interleaving and therefore in who owns escaping; the segment-list form keeps both here and is the recommendation.

```yaml
source:
  - requirement:message-hole-binding, the request's open contract 2
  - read against htmlbind/ops.go Slot and execSlot, and templates/htmlbind/emit.go BindWrapper, 2026-08-16
review_gate: approved 2026-08-16 by the owner
as_built:
  status: implemented 2026-08-16, parse through render
  syntax:
    chosen: a block, with each hole named by the bound element's own tag; a hole attribute overrides it only where two holes share a tag
    owner_decision: 2026-08-16, over the request's inline form and over a marker attribute on every hole
    why_the_tag: a translation spells a hole `<a>`, and a template writes `<a href="/start"></a>`; the two line up on the page with nothing in between to explain, so the hole-name concept stays invisible in the common case
    written: '{t agree}<a href="/start"></a>{/t}'
  the_ambiguity_it_had_to_solve:
    problem: `{t id}` opens both the plain form and the block, so a parser deciding at the opening would need lookahead, and a second keyword would make an author choose between two spellings of one reference
    solved: the block is discovered at its closer, not opened; on `{/t}` the format parser hands its node list to CloseMessageBlock, which finds the last reference and makes the siblings after it holes
    precedent: decision:value-binding-form desugaring, where a flat statement is rewritten into the subtree it means before analysis
  lowering:
    runtime: htmlbind.MessageSegment, the Message op, and MessageInner; the catalog returns text runs and hole markers, this module interleaves and escapes
    hole_content: the bound element is written empty and its children position becomes MessageInner, so the template supplies the element and the catalog supplies what sits between its tags
    unbound_hole_at_render: the translated text is written without the markup, which keeps the sentence readable and loses only the element
  proven_by_word_order: one template renders `利用規約に同意の上、<a href="/start">開始</a>してください` and `Please <a href="/start">get started</a> after agreeing to the terms`; the segment order is the translation's and nothing about the template changes
  found_while_building:
    node_walkers_again: a MessageBlockNode is a node rather than an expression, so the import collector and the render-context detector both missed it; the same shape as the Expr walkers, and the second time in one feature that a new AST kind was not additive
    the_printer_was_the_third: the formatter refused the block outright, which an author meets on their first save rather than at generation; the node switch there has a default that errors, so it failed loudly rather than silently, and it is the one walker that does
  tests: templates/htmlbind/message_hole_test.go, plus an end-to-end render in two languages
decided: 2026-08-16 by the owner; the segment list, as recommended below
consequence_for_the_request: item D no longer emits closures at all, so the reporter's open contract 2 is answered by removing the thing it asked about
what_must_be_true_whichever_wins:
  - the template supplies the markup and the catalog supplies only hole names
  - this module never reads the catalog
  - a translator cannot introduce a tag, an attribute, or script
what_the_existing_continuation_actually_is:
  finding: the html type lowers to htmlbind.Fragment, which is a bound plan plus its params — a self-rendering value, not a wrapper closure
  wrapping_form: BindWrapper sets a Fragment into a component's children field, which is how a layout wraps a page
  consequence: reusing it means the inner run of message text must itself become a Fragment, and a Fragment carries head contributions, assets, a validator, and an instance id that a run of text has none of
  therefore: the request's assumption that the existing continuation is the obvious candidate does not survive contact with what a Fragment is
candidates:
  fragment_continuation:
    shape: the hole is a wrapper whose children is a Fragment the message package builds
    benefit:
      - one continuation concept in the language rather than two
      - the inner content is an ordinary subtree, so requirement:boundary-decomposed-render spans, requirement:component-output-cache, and head merging all keep working with no new rule
      - escaping stays under rule:template-context-safety with nothing moved
    cost:
      - the message package must construct render plans, which is a far heavier dependency than the reporter is asking to take on for a run of plain text
      - a Fragment models things a text run does not have, so most of the type is dead weight at every hole
      - it makes the generated catalog code depend on this module's plan API, which the reporter's own boundary keeps it clear of
    verdict: correct in the language and wrong in the coupling; it exports this module's hardest type across the boundary
  writer_closure:
    shape: the request's own form, one closure per hole taking a writer and an opaque inner continuation
    benefit:
      - the message package only writes bytes, which is the smallest thing it can be asked to do
      - the generated function already holds the segment table, so it drives the interleaving with no data crossing back
      - smallest amount of new code in this module
    cost:
      - a second continuation shape beside Fragment, and the only form in the language that writes to the stream
      - the writer type becomes public API and has to be versioned like one
      - the literal text between holes never passes this module's escaper, so the rule:template-context-safety obligation is discharged downstream; requirement:message-hole-binding records this and it is the one place the concept:template-message-surface boundary claim about escaping stops being true
      - a stream write is opaque to span recording, so the region is one range, the reading requirement:component-output-cache already reached for a cached unit
    verdict: cheapest to build and the only one that moves a safety obligation across the boundary
  segment_list:
    shape: inverted — the generated function returns its segments, text runs and hole markers, and this module interleaves them with its own ops
    benefit:
      - escaping stays here, so every message form is escaped by the template and the boundary claim stays literally true with no exception
      - the hole markup stays ordinary template ops, so spans, decomposition, caching, and head all work by construction rather than by argument
      - no writer type in public API, and no stream-writing form in the language at all
      - the closure signature question disappears, because there is no closure
    cost:
      - the generated message API is slightly larger: it exposes segments rather than only rendering
      - one slice per reference per render, where the writer form allocates nothing
      - the interleaving code lands in this module instead of in generated code, which is more work here and less downstream
    boundary_check: choosing between plural forms and resolving a fallback chain still happen downstream when the segments are produced; interleaving a list is no more i18n knowledge than rendering children is
    verdict: the recommendation
decision:
  chosen: segment_list, approved 2026-08-16 by the owner
  reason: it is the only candidate under which requirement:message-hole-binding stops being an exception — to escaping, to span recording, and to the claim that this module returns strings and never writes markup it did not compile
  what_would_reopen_it: a measurement showing the per-reference slice matters on a page carrying many rich-text messages, which would favor writer_closure and accept the escaping handoff explicitly rather than by omission
  what_does_not_reopen_it: implementation cost here, since the cost is paid once and the exception would be read by every future reviewer of rule:template-context-safety
  moot_now: the closure signature, and the question of who declares generated message code a trusted-output producer under requirement:explicit-output-control; with no stream write there is no trusted output to declare
generated_api_this_implies:
  returns: the resolved message as an ordered list of text runs and hole markers, each marker naming a bound hole
  does_not_return: markup, a writer, or anything carrying this module's types
  string_form_unaffected: decision:message-reference-syntax keeps returning a plain string for a message with no hole, so the segment API is the rich-text path only and an ordinary message costs no slice
  argument_interpolation: stays inside the generated function, so a text run is already complete when this module escapes it
  who_reports_a_mismatch: still downstream, per requirement:message-hole-binding, since a marker naming an unbound hole is visible when the segments are produced
```
