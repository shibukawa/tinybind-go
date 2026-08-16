---
id: requirement:message-hole-binding
type: requirement
title: Rich Text Hole Binding
---
Let a reference bind named holes to markup written in the template, so a translation carries structure without carrying tags.

```yaml
source: concept:template-message-surface, request item D
review_gate: approved 2026-08-16 by the owner
priority: the request's own sequencing puts it last and says it can be designed after something runs
form:
  written: '{t terms.agree, a: <a href="/start">}'
  translation_side: the catalog text names `a` as a hole; a translator writes no tag, no attribute, and no script
  therefore: rule:template-context-safety needs no exception for translator-supplied markup, which is the whole point of the form
lowering:
  decided: decision:message-hole-lowering, 2026-08-16; the segment list, which inverts the drive direction the request proposed
  what_this_module_knows: nothing about the message text, so it cannot know where a hole opens or closes
  what_it_asks_for: the resolved message as an ordered list of text runs and hole markers
  who_interleaves: this module, rendering its own ops around each marker, exactly as it renders children
  what_the_request_proposed_instead: one renderer closure per hole, with the generated function driving; recorded in that decision with the reasons it lost
  no_stream_write: the earlier reading that this is the only form writing to the stream no longer holds; nothing in the language writes markup this module did not compile
escaping_stays_here:
  fact: a text run arrives complete, with the message's own arguments already interpolated, and is escaped by this module for the position it lands in
  consequence: rule:template-context-safety needs no exception, and the concept:template-message-surface boundary claim that escaping is unchanged is true of every form with no carve-out
  what_this_replaced: an earlier reading in which the literal text between holes never passed this module's escaper and the obligation was discharged downstream; the segment form removes the handoff rather than documenting it
  markup_inside_a_hole: ordinary template markup, analyzed under the existing rules, because that is what it is
positions:
  rule: a reference binding holes produces structure, so it is legal where structural children are legal and not inside an attribute value
  contrast: decision:message-reference-syntax stays an ordinary string expression, legal anywhere an expression is
  why_the_split_is_fine: an author reaching for markup in an attribute has no valid form to reach for in the first place
mismatches_are_downstream:
  unbound_hole: a hole named in a translation and not bound at the reference is reported at generation
  unused_binding: a hole bound at the reference and named by no translation is reported, since the element would never render
  where: downstream, because both need the catalog
  what_this_module_supplies: the hole bindings it parsed, through requirement:template-parse-introspection, and nothing else
  settled: the reporter states both directions are errors and no longer open
interactions_the_segment_form_settles:
  cached_component: the hole markup is ordinary ops, so decision:cache-component-declaration eligibility is unaffected and the earlier question about an html parameter does not arise
  decomposed_render: requirement:boundary-decomposed-render records spans over ops, and these are ops, so the region decomposes like any other rather than being one opaque range
  cache_key: the segments depend on the binding decision:implicit-binding-cache-identity already keys on, so a rich-text message needs no further key part
acceptance:
  - a reference binding one hole renders the translation's structure around the template's markup
  - the markup inside a hole is analyzed as ordinary template markup under existing context rules
  - a text run carrying markup characters is escaped by this module for its position
  - a translation naming no hole and a reference binding none behaves exactly as the string form
  - a reference with holes reports its bindings through the parse surface
  - a rendered rich-text message decomposes into spans like ordinary markup
```
