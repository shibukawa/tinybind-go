---
id: rule:form-state-reconciliation
type: rule
title: Form State Reconciliation
---
Reconcile form control state by value rather than by node, because a control whose options the server rewrote still has a user choice worth keeping.

```yaml
source:
  - rule:preserved-client-subtree
  - user select case 2026-08-01
two_kinds_of_client_state:
  node_keyed: focus, text selection, IME composition, scroll offset, media position, and custom element internals; they belong to a specific element and survive only if that element survives
  value_keyed: a chosen option, a checked box, and typed text; they are meaningful independently of the node that carried them, so they can survive replacement
  consequence: rule:preserved-client-subtree islands protect the first kind and cannot address the second
why_islands_fail_here:
  case: a select whose options depend on an upstream choice
  conflict: the option list must change, so the subtree cannot be preserved, yet the current selection should often survive
  resolution: patch the options and transfer the selection separately
dirty_detection:
  mechanism: HTML already separates what the server said from what the user did
  pairs:
    - value against defaultValue
    - checked against defaultChecked
    - selected against defaultSelected
  rule: a control is user-modified when the two differ before the patch
  benefit: no extra bookkeeping, no author markup, and no snapshot to keep between updates
comparisons:
  note: four distinct comparisons, not one; naming them separately keeps the procedure unambiguous
  dirty_detection: an element's current property against its own default property, before the update
  control_identity: a recorded control against the controls present after the update, matched by name and falling back to position
  applicability: a recorded user value against the new set of options or radio inputs
  assertion_change: the old default against the new default, which decides the conflict
procedure:
  1: before applying, run dirty detection and record the user-modified controls and their values
  2: apply the update, which rewrites the attributes and therefore the defaults
  3: locate each recorded control again through control identity
  4: restore its value where applicability holds and assertion_change says the server stayed silent
  applicability_detail:
    select: the recorded value still exists among the new options
    radio_group: the recorded value still exists among the new inputs
    text_and_checkbox: always applicable, because no option set constrains them
conflict_rule:
  server_asserts: when a control's default changed between the two renders, the server is stating a new value and wins
  server_silent: when the default is unchanged, the server expressed no opinion and the user's value is kept
  reason: an unchanged default cannot be distinguished from an unexamined one any other way, and treating silence as an assertion would discard input on every unrelated update
  strategy_independence: the comparison needs the old and new defaults, which every decision:dom-application-strategy stage can supply; the static-dynamic split only makes the silence explicit rather than inferred
control_identity_detail:
  primary: its name within the region
  fallback: its position in the region when unnamed
  limit: a control that is neither named nor stably positioned cannot be matched, and the server render stands
  relation: the same job decision:list-item-key does for repeated markup, applied to controls
activation:
  default: on, for every region an update touches
  reason: losing typed text is the failure users notice, and requiring an opt-in means every author pays attention to it or every author's forms break
  opt_out: none in the first cut
  reset_directive: none in the first cut
first_cut_scope:
  decided: 2026-08-01
  shape: always on, no way to turn it off, no way for one response to discard user state
  rationale: three mechanisms cannot be designed well before the base behavior has been used, and the base behavior is the part that carries the user-visible benefit
known_gap:
  case: the server wants to clear a control back to a default that did not change, such as emptying a form after a successful submit
  why_it_fails: the tie-break reads an unchanged default as silence, so the user's value is kept
  not_solved_by: comparing defaults harder, because the two situations are identical in the markup
  why_it_rarely_bites:
    - a form submit is not a GET, so requirement:client-navigation leaves it to the browser and the response is a complete document with no region to reconcile
    - post-redirect-get therefore clears a form through an ordinary page load, outside this rule entirely
  remaining_case: clearing a form through a GET update, which the first cut cannot express
  future: an opt-out marker, a reset directive, or both, once real use shows which is needed
dropped_value:
  first_cut: report it through a browser alert
  intent: a dropped choice is silent data loss, and making it impossible to miss during development is worth more right now than a polished channel
  provisional:
    status: deliberately temporary, not the intended end state
    problem: a value falling out of the option set is often ordinary application behavior rather than a defect, so a blocking modal will fire during normal use in production
    replacement: an event application code can subscribe to, so the application decides whether to warn, re-select, or ignore
  distinct_case: a control that could not be matched at all is an authoring defect rather than ordinary behavior, and stays worth reporting loudly
unrecoverable:
  file_input: a selection cannot be restored by value and must not be replaced; it belongs in an island or outside the region
change_events:
  rule: restoring a value does not dispatch input or change events
  reason: a restore returns the control to the state the user already produced, so replaying events would double-apply application logic
acceptance:
  - changing an upstream select rewrites the downstream options and keeps the current choice when it still exists
  - a choice that no longer exists yields to the server's render rather than persisting invisibly
  - an unrelated update to a region never clears typed text
  - a server-changed default replaces a user value, because the server is asserting it
  - a file input is never silently emptied
open_questions:
  - when to replace the alert with an application-facing event
  - whether a multi-select restores partial matches or nothing
```
