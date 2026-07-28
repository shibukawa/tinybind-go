---
id: requirement:typed-html-route-parameters
type: requirement
title: Typed HTML Route Parameters
---
Decode dynamic route segments into typed values before rendering, so a page never reads raw path strings.

```yaml
source: concept:filesystem-html-routing
discovery: decision:html-route-file-conventions
notation: decision:route-segment-notation
example: app/users/id_/page.tb.html maps one URL segment to id
typing:
  name: the directory name with its trailing marker removed supplies the parameter name
  type: the page or layout declaration supplies a supported scalar type; string is the simplest default
  position: the declared parameters begin with every dynamic segment in route order, per requirement:colocated-route-logic, so position carries the mapping and no annotation is needed
  query_tail: the parameters after that prefix are query parameters keyed by their own names
  complex_types: a struct or other non-scalar declared parameter is a generation error, because a URL carries no object
  decoder: generation emits a typed decoder for the route, reading the stdlib path values and the query string and validating them
invocation:
  rungs_1_and_2: the generated handler runs the decoder first, so the component or func Page receives validated values and never sees a raw string
  rung_3: the hand-written handler calls the decoder itself, or reads r.PathValue directly; the decoder is a convenience there, not a gate
  reason: the side that owns the response also owns what an invalid value means
  failure_generated: the configured invalid-parameter mapping of requirement:generated-route-registration, before rendering
  failure_raw: the decoder returns an error and the handler chooses the status
scope:
  page: may accept every dynamic segment from route root through page directory
  layout: may accept only dynamic segments from route root through its own directory
  parent_layout: cannot depend on deeper child segment parameters
search_parameters:
  page: declared as the trailing parameters of query_tail, decoded by the same generated decoder
  layout: excluded by default so search changes do not invalidate ancestor wrappers
validation:
  - declaration name must match an in-scope dynamic segment
  - duplicate parameter names in one route are generation errors
  - missing required declaration inputs and incompatible types are generation errors
open_questions:
  - supported non-string scalar decoders
  - declaration form for optional page search parameters
  - generated decoder name and whether it returns one struct or individual values
  - catch-all segment typing, given that it binds a path remainder rather than one segment
```
