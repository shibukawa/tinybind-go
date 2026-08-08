---
id: rule:dynamic-slot-kinds
type: rule
title: Dynamic Slot Kinds
---
Every dynamic in a requirement:structured-render-output unit names the output position it fills, and every decision that reads the value rather than the position is made before the value leaves; a client applies a value and never judges one.

```yaml
source:
  - requirement:structured-render-output
  - downstream framework partial transfer report 2026-08-08
  - rule:template-context-safety
review_gate: proposed
status:
  deferred: 2026-08-08, with requirement:structured-render-output
  reason: requirement:boundary-decomposed-render ships instead; its client concatenates and reparses, so nothing assigns a value to a node and no kind has to travel
  shipping_design_inverts_one_clause: every value below travels before escaping because a client assigns it; the decomposed design sends values already escaped, exactly as the module writes them, because a client splices them into a string
  what_that_buys: a client needs no escaping knowledge at all, which is stronger than naming an escaper to it, and requirement:url-attribute-scheme-safety stays server-side by construction rather than by rule
  read_this_concept_as: the contract a later split would need, not the one in force
  kept_because: the encoding-versus-policy line below is the durable part, and it is what any later split has to hold; re-deriving it would risk moving the URL check across the boundary requirement:url-attribute-scheme-safety put it on
why_a_kind_and_not_a_string:
  alternative: send assembled markup and let a client parse it, which is what a delivery does today
  cost: the client reproduces the escaping rules, and reproducing them wrong turns a text interpolation into markup injection
  so: the position travels as data, and the escaping decisions the position implies stay where they are
the_distinction_this_rule_rests_on:
  encoding: which characters a position requires be encoded, fixed by the position and knowable without looking at the value
  policy: whether a value may appear in the position at all, read from the value and from a render option
  rule: an encoding may be named to a client, because naming it does not move a judgement; a policy never travels, because moving it moves the judgement
  same_split: decision:url-context-escaper drew it once already, when the scheme check moved out of the value closure into an op that can read the render options
kinds:
  text:
    fills: a child node
    value: the value before escaping, which is what the closure already hands its op
    applied: assigned to a text node as is; a client splicing into markup applies the text escaper the kind names
  attribute:
    fills: one attribute's whole value, author literals included
    value: the assembled attribute text before escaping
    applied: set as an attribute value as is; a client splicing applies the same escaper and the delimiter it chose
    note: the value is one string rather than a per-expression list, because the closure assembles it; splitting it further is emitter work requirement:structured-render-output sizes
  optional_attribute:
    fills: an attribute that may not be written at all
    value: presence, and the attribute text when present
    applied: absence removes the attribute rather than emptying it
    distinct_from_attribute: Attr reports presence and an absent value omits the name and the quotes too, so an empty string is a different render
  boolean_attribute:
    fills: an attribute whose presence is its whole meaning
    value: presence alone
    applied: added or removed
  url_attribute:
    fills: an attribute a browser resolves as a URL
    value: the value after this render's scheme policy ran, or the neutralization marker it substitutes
    applied: as an attribute; the client tests nothing
    policy_stays: requirement:url-attribute-scheme-safety decides what may appear here from an allowlist and a data media-type list that are render options, and both are judgements over the value
  url_list_attribute:
    fills: srcset, imagesrcset, ping, and the rest of the per-entry grammars
    value: the surviving entries, one bad entry dropped and the good ones kept
    applied: as an attribute
  raw:
    fills: a position the template marked trusted, per requirement:explicit-output-control
    value: unchanged
    trust: exactly what it is today; this kind moves no boundary
    the_one_slot_the_fast_path_cannot_serve: a value assignment cannot install markup, so a changed raw value is parsed into its recorded parent
    degrades_locally: that is today's behaviour scoped to one slot rather than to the whole unit, which is the point of the split holding for every other kind around it
  module_owned:
    fills: a position written from render state rather than from parameters, meaning the boundary identity attribute, the CSRF field, and the merged head
    value: none a client may address
    applied: it is part of the unit's fixed output for that render, not a slot
    reason: a caller cannot supply these and must not be able to replace them; the CSRF token in particular reaches the browser only in the field the module already rendered
never_travels:
  - the scheme allowlist and the data media-type policy, because they are a caller's render options and a judgement over the value
  - the CSRF token beyond the rendered field, per policy:html-update-csrf-protection
  - any rule a client would have to re-derive to decide whether a value is admissible
escaper_contract:
  why_it_is_owed: requirement:structured-render-output first_build has the client assemble the skeleton's string and parse it once, so on that one pass the client applies the escaper a kind names
  what_that_is_and_is_not: an encoding of five characters, mechanical and testable, and never the scheme allowlist or any other judgement over a value
  attribute_delimiter: the attribute escaper is defined for a double-quoted attribute, matching what this module writes; a client choosing another delimiter owns the difference
  steady_state: no escaping at all, because a text node assignment and setAttribute take the value as it stands
  harness: the escaper is exactly what a conformance harness can test in both implementations, which is why naming it is acceptable where naming a policy is not
constraints:
  - a kind is a property of the position and is fixed at generation; no value changes its own kind
  - a client that does not recognize a kind takes the assembled form for that unit rather than guessing, so an added kind degrades instead of breaking
  - adding a kind is additive, per requirement:structured-render-output compatibility
  - the values of one unit applied by a conforming client produce the bytes the assembled path would have written
  - a kind never widens what a position accepts; rule:template-context-safety is unchanged and this rule is how it survives the split
related:
  - requirement:structured-render-output
  - rule:template-context-safety
  - requirement:url-attribute-scheme-safety
  - decision:url-context-escaper
  - requirement:explicit-output-control
  - decision:dom-application-strategy
```
