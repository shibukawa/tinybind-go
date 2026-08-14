---
id: requirement:typed-action-published-name
type: requirement
title: Typed Action Published Name
---
Report the name a script calls a typed action by, derived from the Go name in initialism-aware lowerCamelCase and overridable on the declaration.

```yaml
priority: should
source:
  - downstream framework change request 2026-08-13, ask 4
  - requirement:typed-server-action
review_gate: proposed
why_it_cannot_be_the_go_name:
  fact: a script writes actions.getUser, and Go's export rule leaves no choice about the identifier
  consequence: the published name is a second name, so something has to say what it is
  scope: the wire only; the Go name stays the identity everything else keys on, as a struct tag already draws that line
derivation:
  rule: lowercase the leading run of capitals, leaving the last of the run intact when a lowercase letter follows
  examples: GetUser to getUser, GetURL to getURL, URLFor to urlFor, ID to id
override:
  form: an optional string on the declaration of decision:typed-action-declaration
  for: a name the derivation reads wrong, and a published name a Go rename must not move
output:
  where: a field on the route table entry beside Name and Hash
  why_a_field_is_enough: the caller derives its own registration from that table already
two_derivations_in_one_module:
  found: 2026-08-13
  what: generator lowerFirst lowercases the first rune only, and it is what the untagged fallback of concept:standalone-json-codec json_tag already uses for a field, so it spells URLFor as uRLFor and ID as iD
  the_choice: adopt the initialism rule here and hold two derivations, or reuse lowerFirst and publish uRLFor
  recommendation: adopt it here and leave the field rule alone, because a JSON wire name is a shipped default and changing it moves the wire under every existing project, while an action name has no installed base
  worth_revisiting_separately: whether the field rule should follow, as its own question with its own compatibility cost
  not_shared_yet: one helper serving both would be the tidy end state and is only reachable after that question is answered
separable:
  by: the reporter's own sequencing
  without_it: the caller derives the name from the Go name and loses only the override
  cost_of_deferring: a published name that a Go rename moves, and no escape for a derivation that reads a name wrong
acceptance:
  - the route table entry for a typed action carries its published name
  - the four derivation examples above produce those four names
  - a declaration carrying an override publishes that string verbatim
  - renaming the Go function of an overridden action changes no published name
  - a raw handler entry is unaffected, carrying whatever it carries today
related:
  - requirement:typed-server-action
  - decision:typed-action-declaration
  - concept:standalone-json-codec
  - rule:template-name-casing
```
