---
id: rule:boundary-placeholder-parse-position
type: rule
title: Boundary Placeholder Parse Position
---
A boundary placeholder must be an element the HTML tree construction algorithm leaves where it was written, because a hole that moves during parsing is filled in the wrong place and nothing reports it.

```yaml
source:
  - downstream framework defect report 2026-08-09, against v0.4.7, found by opening a page
  - verified independently with golang.org/x/net/html, which implements the tree construction algorithm
shipped_today:
  element: '<prefix>-boundary, an unknown element, written with style="display:contents"'
  two_writers:
    delta_hole: htmlbind/delta/boundary.go collector.placeholder, one per nested boundary
    await_placeholder: htmlbind/ops.go, wrapping the fallback during a progressive render
  why_one_shape: requirement:boundary-decomposed-render chose the await element for the delta hole so a client recognises one hole shape rather than two; the defect is therefore in both at once
the_parse:
  rule: 'in table context an unknown element is foster-parented — the in-table insertion mode processes anything else with foster parenting enabled, inserting it immediately before the table'
  not_a_quirk: it is the algorithm, so every conforming parser does it and no caller markup avoids it
  verified:
    delta_hole_in_tbody: '<table><tbody><X></X></tbody></table> parses to <X></X><table><tbody></tbody></table>'
    await_placeholder_with_fallback: 'the placeholder is foster-parented out and its own fallback rows stay inside the table, so the two are separated'
consequences:
  delta: every hole a table row leaves sits outside the table, the rows filling it land loose on the page, and the list is left empty
  await: worse, because a client settles a boundary by id and therefore writes the finished row where the placeholder ended up — outside the table — while the fallback row stays in the list permanently
  silent: the response is correct as bytes and the resulting DOM is valid, so nothing on either side reports it; the reporter found it by counting rows after a navigation
  await_has_no_workaround: a caller cannot rewrite a document the browser is parsing as it arrives, so the streamed case is not fixable downstream at all
candidates:
  template:
    kept_in_table: yes
    carries_attributes: yes
    renders: nothing, so display:contents becomes unnecessary
    smaller_change: stays an element, so querySelector and every existing lookup are unaffected
  comment:
    form: '<!--tb-hole:id-->'
    kept_in_table: yes
    carries_attributes: no
    cost: a client walks siblings instead of querying
downstream_workaround:
  what: rewrite holes to template before parsing and restore the spelling through the DOM, where insertion is not parsing and nothing is foster-parented
  reporter_position: it works and it is a rewrite of this module's markup in their client, which is the wrong place for it
  agreed: the placeholder shape is this module's to choose, and a shape only correct outside tables is not a shape
as_built:
  shipped: 2026-08-09
  two_shapes_not_one:
    hole: '<template attr="id"></template>, replacing the unknown element and dropping display:contents, which a template never needs'
    await: '<!--prefix:id-->fallback<!--/prefix:id-->, a comment pair around the committed fallback'
    why_they_diverged: 'the reporter proposed template for both; it does not work for await, because a template does not render its content and the fallback has to be visible without JavaScript — which is the whole of that path'
    what_the_one_shape_goal_was_worth: keeping it would have meant comments for the hole too, costing the attribute selector a client already uses to fill one; the operations differ — a hole is one node to replace, an await marker brackets a range — so one shape was covering two things
  client_change:
    hole: none; a template carries the id attribute, so resolve finds it and replaceWith replaces it exactly as before
    await: 'a comment walk to find the pair, then replace the range between them; runtime.js gained fence and settle'
  prefix_now_spells_a_comment: validateNamePrefix rejects a doubled hyphen, which would close the marker early and put the rest of it in the document as markup; nothing else in the naming rules caught that
  sequence_became_prefix_independent: the hole frame no longer contains the prefix, so Plan.Sequence takes no prefix and one address serves every deployment
  verified:
    how: parsed with golang.org/x/net/html, which implements the tree construction algorithm, rather than matched against the expected string
    why_parsed: asserting the hole is spelled a particular way restates the code; the property a page depends on is that a conforming parser keeps it inside the tbody
    confirmed_failing: the test fails on the previous placeholder, naming body as the parent it reached
    tests: TestAHoleInsideATableStaysInsideTheTable, TestAnAwaitFallbackInsideATableStaysInsideIt, TestTheAwaitMarkersBracketTheirOwnFallback
  cost: golang.org/x/net becomes a test-only dependency; htmlbind's own imports stay standard library, so a TinyGo build is unchanged
related:
  - requirement:boundary-decomposed-render
  - requirement:suspense-html-streaming
  - decision:update-manifest-transport
open_questions:
  - whether the boundary element name stays configurable once the shape is template or comment, since neither takes an arbitrary tag name
```
