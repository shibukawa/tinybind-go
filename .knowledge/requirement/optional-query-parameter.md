---
id: requirement:optional-query-parameter
type: requirement
title: Optional Query Parameter
---
Distinguish an unspecified query parameter from a zero value, using the optional type the template language already spells.

```yaml
priority: must
status: implemented 2026-07-30
source:
  - downstream framework integration request 2026-07-30
  - requirement:typed-html-route-parameters
problem:
  decoder: 'the generated query read is `if raw := query.Get(key); raw != ""`, so an absent parameter and a zero value produce the same field'
  already_spelled: an optional declaration such as 'page: int?' already lowers to '*int' in the generated parameter struct of requirement:html-component-api
  rejected_today: the route decoder refuses that type, reporting that no generated decoder can bind it from a URL
  harm: filters and paging cannot tell page=0 from no page at all
  two_semantics: a project also calling api:bind has a tag-level distinction on the same page, so one project would carry two meanings for one URL question
implemented:
  spelling: a pointer to a supported scalar, which is what the existing optional type marker generates
  absent: nil when the key is missing or its value is empty
  present: a non-nil pointer to the parsed value
  invalid: a non-empty unparsable value stays the invalid query parameter error of requirement:typed-html-route-parameters
  applies_to: the query tail only, read from the component's parameter list since decision:route-handler-shape removed the typed rung
  path_segments: unchanged; a missing single segment does not match the route, and a catch-all binds an empty remainder as a string
rejected_presence_by_has:
  shape: query.Has decides presence, so ?q= yields a non-nil empty string
  why_not: an empty form control submits its key with no value, so a blank filter field is the common source of an empty value, and distinguishing it for strings alone buys an inconsistency
rejected_optional_type:
  shape: a generic Optional[T] wrapper
  why_not: it puts a runtime type into a page signature that needs none today, where a pointer needs no import at all
resolves:
  - the optional search parameter declaration question of requirement:typed-html-route-parameters
  - the optional query parameter spelling question of requirement:colocated-route-logic
acceptance:
  - a page declaring 'page: int?' receives nil for /list and a pointer to 0 for /list?page=0
  - /list?page=x still fails with the invalid query parameter error before rendering
  - a component parameter declared 'int?' reaches the generated decoder as '*int', which is the one list left to read
  - a string query parameter without the marker behaves exactly as before
related:
  - requirement:typed-html-route-parameters
  - requirement:colocated-route-logic
  - requirement:html-component-api
  - decision:framework-integration-seams
```
