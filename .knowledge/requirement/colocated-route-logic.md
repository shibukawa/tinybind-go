---
id: requirement:colocated-route-logic
type: requirement
title: Colocated Route Logic
---
Serve a page from a template alone or from a plain net/http handler beside it, with one input rule.

```yaml
priority: must
source:
  - concept:filesystem-html-routing
  - user logic-placement decision 2026-07-27
  - user three-mode decision 2026-07-27
review_gate: proposed
shape_decision: decision:route-handler-shape
notation: decision:route-segment-notation
package_model: decision:html-route-go-package-model
location:
  template: page.tb.html in the route directory
  logic: page.go in the same directory, optional
  package: ordinary Go package whose clause matches the directory name
rung_2_is_replaceable_2026_08_14:
  demonstrated: a route serving one record with no page.go entry point at all — the component takes the path parameter, binds its own loader, and the loader's error answers 404 or a redirect
  what_made_it_possible: requirement:template-value-binding for the name, decision:value-binding-hoisting for a failure that lands before the first byte, and requirement:redirect-error for the value that carries the intent
  what_rung_2_was_for: decision:route-handler-shape says a page that needs Go to decide, combine, or fail; all three now have a rung 1 spelling
  the_typed_check_moves_rather_than_disappears: rung 2 compared the function's results against the component's parameters, and a component taking its own inputs is checked by its own parameter list instead
  what_rung_3_keeps: streaming, downloads, and conditional statuses, which the generated handler does not model
  retired: 2026-08-14, with no deprecation period, because the owner confirmed nothing downstream had adopted it; the three fixture pages using it moved to a component parameter plus a {val} binding first, so the replacement was demonstrated before the shape was removed
bug_found_on_the_way:
  what: the registry filled a component's parameter struct using this package's initialism-aware ExportedName, so a rung 1 page declaring a parameter named id emitted ID where the template compiler emits Id, and the route package did not compile
  why_it_was_never_seen: no fixture had a rung 1 page taking an initialism path parameter, because such a page was written at rung 2
  why_it_mattered_now: retiring rung 2 makes every page rung 1, and id is the most common path parameter there is
  fix: the two structs are named by their own owners — the decoded route by ExportedName, the component's parameters by the template compiler's exported FieldName
input_rule:
  applies_to: the component parameter list, which is the only list left since decision:route-handler-shape removed the typed rung
  order: every dynamic path segment first, in route order, then the query parameters
  path_prefix:
    required: the leading parameters must be exactly the route's dynamic segments, in order
    check: a missing, reordered, or extra leading path parameter is a generation error naming the route and the declaration
    reason: position carries the mapping, so no per-parameter annotation is needed
  query_tail: every remaining parameter is a query parameter, keyed by its own name
  optional_query: requirement:optional-query-parameter spells one as a pointer, closing the question of how an always-present Go parameter expresses an absent value
  context_parameter: requirement:typed-page-context-parameter proposes a leading context.Context at rung 2, excluded from this list because it is not a URL input; it answers the open question this requirement carried from 2026-07-27
  types:
    allowed: scalars the generated decoder can bind, per requirement:typed-html-route-parameters
    rejected: a struct or other complex type is a generation error, because a page input is a URL value and a URL carries no object
  invalid_value: the generated handler maps a failed decode to the configured status before rendering
rung_1_template_only:
  files: page.tb.html and no page.go
  handler: fully generated, with no application Go on the request path
  component_parameters: the input list of input_rule
  data_source: external declarations called from the template, dispatched through data:html-route-dependencies
  for: a page whose data needs no Go beyond the calls the template already makes
  acceptance_note: adding a route directory with one template file is a complete, working page
rung_2_typed_page:
  files: page.tb.html and page.go declaring a typed func Page
  parameters: the input list of input_rule
  results:
    rule: the return list must equal the component parameter list of page.tb.html, in order and type, followed by an error
    shape: 'func Page(id string, page int) (User, []Order, error)'
    check: a mismatch in count, order, or type is a generation error naming both lists
    reason: the function produces exactly what the page renders, so nothing has to be assembled or named in between
  async: a returned value may be an htmlbind.Pending handle where the component declares an async parameter, per requirement:awaitable-parameters
  error_path:
    display: a returned error produces the configured error response instead of the page
    redirect: an error carrying a redirect target sends the browser there, per requirement:redirect-error
    timing: the function runs before any byte is written, so both outcomes can still choose the status
  for: a page that needs Go to decide, combine, or fail
rung_3_handler_page:
  files: page.tb.html and page.go declaring func Page as an http.HandlerFunc
  shape: 'func Page(w http.ResponseWriter, r *http.Request)'
  generated: registration only
  owns: decoding, loading, rendering, status, headers, body, negotiation, and errors
  rendering: the handler calls the generated component and the generated composer itself
  available: the generated component, its parameter type, the composer, and the route decoder, all named symbols in the same package
  for: streaming, server-sent events, downloads, conditional statuses, and anything the generated handler does not model
no_duplicate_declaration:
  principle: the template declares the page inputs once, and requirement:html-component-api already generates a Go struct from that declaration
  rung_1: no Go is written at all, so nothing can drift
  rung_2: the function's lists are checked against the template's lists, so a drift is a compile-time or generation-time failure rather than a runtime one
  rung_3: the author uses the generated parameter type directly
generated_surface:
  component: the page component function from requirement:html-component-api
  params: its generated parameter struct
  composer: a per-route Render that applies the discovered layout chain and decision:html-document-shell
  route_decoder: a typed decoder for requirement:typed-html-route-parameters
  visibility: every one is a named symbol in the route package, callable explicitly
customization:
  owner: data:generator-options, extended by api:generator-call-registration
  rationale: requirement:custom-framework-generation-profile requires a downstream framework to reproduce its own developer experience without working around the generator
  customizable:
    template_pattern: requirement:configurable-template-file-patterns already covers the template file glob
    component_name: the reserved page component name, defaulting to Page
    function_name: the reserved Go function name, defaulting to Page
    render_call: the composer and render entry a generated handler emits, so a framework can substitute its own, through the render block and composer entry settings of requirement:framework-render-entry
    bind_call: the binding operation used for path and query decoding, through the existing Calls pattern set
    error_call: the error response operation, so a framework's own writer is used
    generated_suffix: the generated file naming, per requirement:per-source-generation-artifacts
  post_generation:
    need: a framework may emit its own types beside the generated ones, after generation
    mechanism: the artifact list of api:generator-artifacts, so the framework writes them from the same run
    constraint: rewriting generated Go source after generation stays a prohibited workaround under requirement:custom-framework-generation-profile
import_direction:
  generated_per_route: the route's component, parameter type, composer, and route decoder are emitted into that route's own package
  ancestors: a route package may import its ancestor route packages to reach their layout components, which is acyclic because a tree has no upward cycle
  registry: a distinct generated package imports every route package and installs handlers
  rule: no route package imports the registry, which is what lets a rung 3 handler call its own composer without a cycle
  shared: a component or helper used by several routes lives in an ordinary package outside the route tree
constraints:
  - no reflection; every binding is resolved and type-checked at generation time
  - page.go is ordinary Go, so gopls, go test, go vet, httptest, and linters apply unchanged
  - generation reads the package Go sources to find Page and check its signature, the way requirement:async-external-functions already reads context parameters
  - a logic file is compiled by the Go toolchain, not re-emitted or copied by the generator
acceptance:
  - a directory holding only page.tb.html serves a working page
  - a page whose template declares a leading parameter that is not the route's dynamic segment fails generation naming both
  - a page declaring a struct parameter fails generation with an actionable message
  - a typed Page whose returns do not match the component parameters fails generation naming both lists
  - a typed Page returning an error carrying a redirect target sends the browser there with nothing written first
  - a handler Page owns its whole response and is testable with httptest and no registration
  - moving a page from rung 1 to rung 2 changes no template parameter spelling
  - a route package compiles and tests on its own with go test ./...
open_questions:
  - whether a layout has the same ladder or only the first two rungs
  - whether the generator can scaffold a rung 2 file from a rung 1 template
```
