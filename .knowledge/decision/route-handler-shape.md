---
id: decision:route-handler-shape
type: decision
title: Route Handler Shape
---
Offer three page shapes on one ladder, from a template with no Go file, through a typed Load function, to a plain net/http handler.

```yaml
source:
  - concept:filesystem-html-routing
  - user shape discussions 2026-07-27
  - user three-mode decision 2026-07-27
review_gate: proposed
method_scope:
  serves: GET for the page
  post_slot: the same pattern also carries POST, registered by requirement:template-server-functions as the form entry point; leaving pages GET-only is what keeps that slot free
  other_methods: a JSON API endpoint is registered manually outside the route tree
ladder:
  principle: each rung adds control and adds work, and a page moves up only when it needs to
  rung_1:
    name: template only
    files: page.tb.html and nothing else
    handler: fully generated
    data: external declarations called from the template
    for: a page whose data needs no Go beyond the calls the template already makes
  rung_2:
    name: typed Load function
    files: page.tb.html and page.go declaring a typed func Load
    handler: generated around the function
    data: whatever the function returns
    for: a page that needs Go to decide, combine, or fail
  rung_3:
    name: handler Load function
    files: page.tb.html and page.go declaring func Load as an http.HandlerFunc
    handler: none generated beyond registration
    data: the handler's own business
    for: streaming, downloads, conditional statuses, and anything the generated handler does not model
one_name:
  rule: the Go function is named Load at every rung where it exists
  gain: the naming chain stays page.tb.html, component Page, func Load, with one word to remember
  cost: rung 2 and rung 3 share a name and are told apart by signature
  not_page:
    discovered: implementation 2026-07-28
    why: the template compiler already emits func Page for the component into the same package, so a func Page beside it is a Go redeclaration
    evidence: the first generated fixture failed to compile with two Page declarations in one route package
    kept: the file stays page.go and the component stays Page; only the Go entry point moved aside
selection:
  rung_1: no page.go, or a page.go declaring no Load
  rung_2: Load whose first parameter is not http.ResponseWriter
  rung_3: Load whose parameters are http.ResponseWriter and *http.Request
  mismatch: a Load that matches neither shape is a generation error naming the signature it has and the two it could have
signature_detection_reversal:
  earlier: this decision rejected selecting between shapes by inspecting the parameter list, and used distinct names instead
  now: reinstated for rung 2 and rung 3, at user direction 2026-07-27
  why_defensible_now:
    - three rungs exist, and two of them are distinguished by the presence or absence of the file rather than by any signature
    - an http.ResponseWriter first parameter is unmistakable, unlike two typed shapes that differed only in their struct arguments
    - keeping one name preserves the naming chain, which was the reason method prefixes were dropped in the first place
  residual_cost: a reader still has to look at the parameter list to know which contract applies, which is exactly the objection that produced the earlier rule
  recorded_because: this is a reversal, and the argument that produced the original rule has not been refuted, only outweighed
parameter_rule:
  shared_by: rung 1 component parameters and rung 2 function parameters
  order: every dynamic path segment first, in route order, then the query parameters
  names: the parameter name supplies the path segment name at rung 1 and the query key at both rungs
  types: scalars that the generated decoder can bind; a struct or other complex type is a generation error
  reason: one rule covers both rungs, so moving from rung 1 to rung 2 does not change how inputs are spelled
component_parameters:
  rung_1: the component parameter list is the input list, because there is no function between the request and the render
  rung_2: the component parameter list is the function's return list, because the function is what produces what the page renders
  rung_3: the handler chooses; it calls the component itself
  consistency: at every rung the component parameter list is what the page renders with, and only its source changes
rejected:
  handler_only:
    shape: rung 3 as the sole option
    why: every route, including a static page, pays for decoding, parameter assembly, and a render call
  typed_only:
    shape: rung 2 as the sole option
    why: rung 1 removes a file entirely for the common case, and rung 3 covers responses no typed return can express
  method_named_functions:
    shape: Get and GetHandler, extending to Post and PostHandler
    why: the method prefix costs every name something to buy an extension the tree no longer needs
    recorded_because: it is the natural form to return to if non-GET page routes are ever added
open_questions:
  - whether rung 2 may also accept a leading context.Context before the path parameters
  - whether a layout has the same ladder or only rungs 1 and 2
  - how optional query parameters are spelled, given that a Go parameter is always present
```
