---
id: rule:event-attribute-context
type: rule
title: Event Attribute Context
---
An on-prefixed attribute is a JavaScript insertion context and accepts only trusted_javascript, which is the same rule html:script already carries.

```yaml
source: security review 2026-08-06
review_gate: approved 2026-08-06, owner decision on the requirements review
sibling_of: requirement:url-attribute-scheme-safety, found in the same review and fixed in the same pass, but by a different mechanism
why_not_the_allowlist: an event handler value has no scheme; the whole value is the handler body, so there is nothing to allowlist and the URL escaper does not apply here
the_state_it_replaces:
  what: templates/htmlbind/compiler.go analyzeAttribute has no on-prefixed case, so the value takes the html:attribute path
  consequence: onclick={s} compiles with a plain string and emits htmlbind.Escape(s) into a JavaScript context
  the_inversion: validateInsertion rejects trusted_javascript in html:attribute, so the type that names itself JavaScript was forbidden while the untyped string was allowed; the gate pointed the wrong way rather than being absent
  quoting_still_held: Escape does escape the attribute delimiters, so a value cannot break out of the attribute; what it could do is be the handler body, which needs no breakout
rule:
  context_id: html:event
  accepts: trusted_javascript only
  mirrors: html:script, which validateInsertion already restricts to trusted_javascript and script_json
  not_script_json: a handler body is code rather than data, so the script_json half of the html:script rule does not carry over
  escape_hatch: requirement:explicit-output-control RawJavaScript, unchanged; it is an explicit trust boundary and not a sanitizer
matching:
  form: the literal prefix on, then one or more ASCII lowercase letters, to the end of the name
  why_a_prefix_and_not_a_list: a fixed roster of handler names goes stale as browsers add them, and the safe direction on an unknown name is to treat it as a handler
  not_matched: a hyphenated name such as on-click, which is not an event handler content attribute and belongs to a custom element
  interaction_with_url_roster: no attribute is on both rosters, so the order the two are tested in does not matter
  partly_reserved_2026_08_12:
    what: requirement:template-client-handlers takes the hyphenated on- namespace inside a component declaring a script block, where it names a function that component's script produced
    scope: that context only; everywhere else a hyphenated on- name stays an ordinary attribute and this rule's custom-element reading is unchanged
    why_it_is_written_here: this clause assigned the namespace, so a feature claiming part of it amends the assignment rather than merely relying on it
    this_rule_is_untouched: a hyphenated name is still not an event handler content attribute and still takes no JavaScript insertion context; what changed is that one context now reserves it before it reaches the attribute path
    onclick_unchanged: the unhyphenated form keeps meaning inline JavaScript and keeps requiring trusted_javascript, which is what makes the two spellings safe to give different meanings
recommended_path_is_unchanged:
  what: server-action, which names a Go function statically and lowers to the client library's attribute
  why_it_matters_here: the language already has a vocabulary for behavior, so this rule closes a hole rather than removing the way to attach one
compatibility:
  in_tree: no fixture, example or document uses an on-prefixed attribute, statically or with an expression, so nothing in this module changes
  in_tree_is_not_downstream: noted 2026-08-12 while reserving the hyphenated form; the survey covered this module, and the spelling requirement:template-client-handlers takes is Polymer's declarative event binding, so a downstream template carrying it changes meaning inside a script-block component
  downstream: a template writing onclick={s} stops compiling, which is the intended outcome and the diagnostic names RawJavaScript
  static_handlers: an on-prefixed attribute with no expression is untouched, because the rule governs insertion and not authored text
rejected_alternatives:
  forbid_expressions_outright:
    what: reject any expression in an on-prefixed attribute, with no trusted type accepted
    why_not: it is a policy judgement about whether apps may use inline handlers, where the rest of the language takes the narrower position of making the type honest and leaving the choice to the author
    noted: it remains available later if the escape hatch turns out to be misused, and it is a strict narrowing of this rule rather than a different one
  sanitize_the_handler_body:
    what: parse the value as JavaScript and accept a safe subset
    why_not: no subset of JavaScript is safe to assemble from untrusted fragments, and the language nowhere else pretends to sanitize
verified:
  status: implemented 2026-08-06
  where: templates/htmlbind/compiler.go isEventAttribute and the html:event case of validateInsertion
  tests:
    - TestGenerateDiagnostics, three cases: a string handler, that the message names RawJavaScript, and that a url is refused too because a URL is not code either
    - TestEventAttributeAcceptsTrustedJavaScript, the RawJavaScript escape hatch
    - TestStaticEventAttributeStillCompiles, authored markup with no insertion
    - TestHyphenatedOnNameIsNotAHandler, on-click staying an ordinary attribute
  escaping_confirmed: a trusted_javascript handler still goes through htmlbind.Escape, because an attribute value is HTML-decoded by the parser before the body is compiled, so escaping keeps the value inside its quotes without changing the JavaScript the browser reads
  nothing_else_moved: no fixture changed for this rule, matching the compatibility note above
related:
  - requirement:url-attribute-scheme-safety
  - rule:template-context-safety
  - requirement:explicit-output-control
  - rule:url-bearing-attributes
  - decision:server-action-lowering
```
