---
id: rule:typed-action-call-only
type: rule
title: Typed Action Is Call Only
---
A template naming a typed server action is a generation error, reporting the template position and the function.

```yaml
status: proposed
source:
  - downstream framework change request 2026-08-13, ask 5
  - requirement:typed-server-action
load_bearing:
  what_it_protects: the fixed response of requirement:typed-server-action, which holds only because one caller reaches a typed action
  effect: a form cannot reach one, so the case where a native submit is shown a JSON document does not arise rather than being handled
  contrast: requirement:native-action-form-submit makes every raw action reachable both ways, and the raw shape keeps branching on the request to serve both callers, because there both are legitimate
scope: every lowering, not only the form one; a bare button naming a typed action is the same error, since a typed action carries no native fallback and nothing else to lower
how_the_name_reaches_the_compiler:
  problem: templates/htmlbind resolves a name through GenerateOptions.ServerActions and falls to ServerActionResolver, so a typed action absent from both is the unknown-name error, which names the wrong cause and points an author at a missing registration
  shape: the compiler is told the typed names, in the way requirement:template-client-handlers takes an explicit unresolved name-to-reason map rather than reading an omission
  why_explicit: an omission is indistinguishable from a map that was never populated, which is the argument that made the client-handler map mandatory
  the_split: this module holds the position, the caller holds the reason, which is the division decision:client-handler-seams already set
deferrable:
  by: the reporter's own sequencing
  cost_until_it_lands: an author can write a form that submits to a typed action and shows a JSON document, with no diagnostic anywhere
  not_silent_at_runtime: the submit would reach the glue, decode nothing it recognizes from a urlencoded body, and answer an error document rather than a page
acceptance:
  - a template naming a typed action from a form fails generation at that position, naming the function
  - a template naming a typed action from a bare button fails the same way
  - the diagnostic says the action is typed rather than that the name is unknown
  - a template naming a raw handler is unaffected
  - a name resolved by requirement:external-action-resolution is unaffected, since a framework's own route table holds no typed action
related:
  - requirement:typed-server-action
  - requirement:template-server-functions
  - decision:server-action-lowering
  - requirement:template-client-handlers
  - requirement:native-action-form-submit
```
