---
id: requirement:typed-server-action
type: requirement
title: Typed Server Action
---
Admit a second server function shape, a declared function of an arbitrary signature, and generate the glue that decodes a JSON call, calls it, and writes its result.

```yaml
priority: should
status: implemented 2026-08-13, asks 1 to 4; the registry side and rule:typed-action-call-only are not built
as_built:
  annotation: httpbind.ServerAction, taking the function symbol and an optional published name, returning a zero-size Declaration so the call can be a package-level var
  discovery: routetree/typedaction.go, matching the annotation's import against the file's own import list, beside the shape filter rather than replacing it
  the_package_identifier_is_declared_not_derived:
    found: 2026-08-13, while building
    what: importName falls back to the import path's last element, which is tinybind-go where the package is httpbind, so nothing matched
    why_it_never_surfaced: the handler shape check resolves net/http and the context check resolves context, and in both the last element is the package name
    fix: ActionDeclaration carries the identifier, so a framework declaring its own annotation states it too
  wrapper: generator/serveraction.go, emitted into the declaring package by the phase that type-checks, naming the generated encoder directly
  argument_struct: synthesized as an ast.StructType from the declared parameters, each type parsed back from its source text, then run through analyzeStruct so a parameter is classified by the rules a hand-written field is
  usage_source: the declaration marks the result type encoded and the argument struct decoded, which is the item_key_exception shape of rule:usage-directed-generation
  error_only_answers_no_content: nothing was produced, and an empty document would claim something was
  published_name: routetree.PublishedName, initialism-aware, with the declaration's optional string overriding it
  tests: routetree/typedaction_test.go over admission, the signature, the override, an unexported function and nine diagnostics; generator/serveraction_test.go over the emitted wrapper, a struct parameter, an error-only action, and a compiled entry point answering a real POST
not_built_yet:
  registry_side: routetree registers no route for a typed action yet, so the entry point exists and nothing addresses it
  ordering: the registry-last sequencing that ordering describes
  template_refusal: rule:typed-action-call-only
source:
  - downstream framework change request 2026-08-13, asks 1 to 3
  - requirement:template-server-functions
  - requirement:typed-page-context-parameter
review_gate: proposed
model:
  authored: 'var _ = pw.ServerAction(GetUser) beside func GetUser(ctx context.Context, id string) (User, error)'
  called: 'await actions.getUser({ id: "3" }), one JSON post to the direct entry point, answered with the encoded result'
  reached_by: client script only; a template naming one is refused by rule:typed-action-call-only
  runtime_side: nothing new; the reporter's runtime already posts a JSON body to an action address and returns a non-update response to its caller, so the client half is built
  not: a replacement for the handler shape, which stays what a form reaches and what a redirect, a status, a download, or a stream needs
why_two_shapes_can_coexist:
  admission_problem: the current rule is the signature, which is unambiguous only because nothing else has that shape; every function has a signature, so a rule over arbitrary ones cannot exist and something outside the signature has to say which functions are actions
  response_problem: requirement:template-server-functions signature.reason fixed the shape because a form action legitimately answers in ways no fixed return covers
  the_scope_it_was_about: a caller holding a document waiting for a page
  narrowing: a caller holding the answer has no use for a redirect and no regions to apply, so with one caller the response is not a choice
  therefore: the argument that fixed the shape is not refuted, only scoped, which is what makes a second shape possible without disturbing the one that ships
reverses:
  clause: requirement:template-server-functions resolution.exposure.rejected_declaration
  was: a compile-time assertion costs a declaration for every action to restate what the package boundary already says
  still_holds: for the handler shape, where a declaration buys only intentional exposure and the route package boundary already bounds exposure
  does_not_hold_here: with an arbitrary signature the declaration carries the one fact no boundary can, that this function is an action
  written_back: yes, into that clause, because the rejection is what a reader of it would otherwise apply to this
admission:
  second_rule: beside the shape filter rather than replacing it; an exported handler-shaped function stays an action by existing, and a declared one is an action by being declared
  input: a package-level declaration naming the function by symbol, per decision:typed-action-declaration
  symbol_not_string: a declaration naming something that does not exist fails to compile before generation reads it, so that case needs no diagnostic
  address: the same /_action/<hash>/<Name> as a raw action, with the same routetree.ActionHash over the declaring directory and the Go name
  table: one entry beside the raw ones, so requirement:external-action-resolution and the registry emitter read one list
  collision: a typed and a raw action on one name is the hash collision routetree already refuses
  reserved: Load stays the page entry point of decision:route-handler-shape and is never an action, typed or raw
  export_is_not_required:
    considered: 2026-08-13, on the reading that the glue lives in the registry and calls the function through its import path, as a raw action is called
    resolved: it does not, because the wrapper is emitted into the declaring package, per where_the_wrapper_is_emitted below, and the registry names the wrapper
    consequence: a declared function may be unexported, so the lower-case opt-out of requirement:template-server-functions stops meaning anything under a declaration; what publishes is the declaration and only the declaration
    stated_because: the request's example is exported and a reader would take that for a rule
signature:
  parameters: bound by name from the call's JSON payload
  one_source: the direct entry point holds no path parameter, so every argument comes from the caller and no precedence rule is needed
  context: a leading context.Context is optional and trimmed, on the terms requirement:typed-page-context-parameter already implements for a typed Load; non-leading keeps the ordinary not-an-input error
  results: one value and an error, or an error alone
  read_from: the AST, as decision:route-handler-shape rung 2 parameters already are, because generation runs before the package compiles
glue:
  emitted: an http.HandlerFunc wrapper whose body decodes the payload, calls the function, and writes either the error or the result, plus the POST registration that installs it
  expanded_not_dispatched: the wrapper names the generated encoder of the result type directly, per the_glue_calls_the_generated_encoder_rather_than_the_registry below
  error_path: the error writer of routetree.Symbols, which a framework already repoints
  success_path: written by the wrapper itself, which is what the expansion means and what the seam question below is about
the_glue_calls_the_generated_encoder_rather_than_the_registry:
  decided: 2026-08-13, by the maintainer, correcting the first reading below
  rule: the wrapper is expanded generated code that names the emitted encoder of the result type directly, in the way emitWriter already names appendUserJSON, and reaches no runtime lookup
  why_it_is_available: the registry of registry.go exists to serve an author-written generic call, which cannot name a function the generator has not written yet; a call site the generator writes itself has no such problem and can name it
  what_it_removes: the missing-writer failure mode entirely, along with the sync.Map lookup on every call and the need for the result type to register a public dispatch entry
  what_remains: rule:usage-directed-generation, which emits a type's encoder only when a discovered call asks for one, so the declaration still has to put the result type in the plan
  the_first_reading:
    was: routetree reports the types and a run feeds them to the codec generator, so httpbind.Write finds a registered writer
    why_it_was_worse: it kept a runtime lookup whose only failure mode is a 500 on the first call, to reach a function the emitter could have named
    kept_because: the discovery analysis below is why either shape is needed at all, and it is the part that is easy to assume away
the_result_type_still_needs_a_usage:
  found: 2026-08-13, reading the request against generator/emit.go and generator/plan.go
  what: a type's encoder is emitted only when its TypePlan carries the matching DirectUsage, per rule:usage-directed-generation, and usage comes from a discovered call
  where_usage_comes_from: rule:response-model-discovery reads the generic argument of an author-written httpbind.Write call site
  why_that_misses_this: the only such call for a typed action is the one the generator emits, and rule:generated-source-not-discovered skips generated source by design, so nothing marks the result type as encoded
  without_it: no appendUserJSON exists for the wrapper to name, and generation fails on an undefined identifier rather than compiling
  precedent_for_the_fix: rule:usage-directed-generation item_key_exception already lets a declaration rather than a call add usage, where a partitionkey tag gets ItemKey with no discovered call
  fix_shape: the typed action declaration is a second such source, adding encode usage for the result type and decode usage for the argument struct
  generalized: requirement:declared-json-codec makes that a mechanism rather than a special case, so this feature consumes it instead of building its own usage source
  failure_moved: from a 500 on the first call to a generation error, which is the gain the maintainer's shape buys beyond the lookup it removes
  same_family_as: requirement:template-server-functions binder_gap 2026-07-30 and rule:generated-source-not-discovered transform_was_missed 2026-08-11, both being a pass meeting its own output
  contradicts: the request's claim that no type checking is needed holds for the glue and not for the feature; the glue reads no fields, and the encoder the glue names is built by reading them
  input_side_is_easier: the generator declares the argument struct, so it plans that struct itself and needs to discover nothing for the decode half
where_the_wrapper_is_emitted:
  constraint: it names the unexported encoder of the declaring package and the declared function, so it belongs in that package rather than in the registry
  emitted_by: the binding phase that emits the encoder, rather than routetree, so one artifact holds the wrapper and everything it names
  registry_side: routetree registers the address and references the wrapper as an exported symbol of the route package, which is the direction it already imports
  consequence_for_export: the declared function needs no export, because the registry names the wrapper rather than the function; what must be exported is the generated wrapper
  ordering:
    chain: components into each route package, then the binding phase, then the wrapper, then the registry that names it
    each_link: a route package type-checks only once the compiled component is in it, per decision:route-handler-shape not_page; the binding phase needs that type-check; the wrapper needs the encoder that phase emits; the registry needs the wrapper
    nothing_can_move_earlier:
      asked: 2026-08-13, whether generating the action first resolves it
      answer: no; the wrapper names an encoder no earlier phase has produced, and the binding phase that produces it cannot itself precede the component emission
      what_moves_instead: the registry, which is the far end of the chain rather than the near one
    the_registry_moves_last:
      already_separate: routes_gen.go is its own artifact in the route root, distinct from the per-route decoder and the per-template component, so deferring it splits no file
      free_because: rule:generated-source-not-discovered requires the binding phase to skip the registry, so writing it before that phase is writing a file that phase is forbidden to read
      costs_no_rediscovery: the action list is resolved before any template is compiled, so the registry emission needs nothing the later phase produces except the wrapper's name
      caller_surface: routetree Result returns one flat Files list today, so the registry entry has to become separately addressable for a caller to order the writes
      no_intermediate_dangling_state: with the registry last, every package type-checks at every point a phase reads it
a_parameter_type_is_whatever_the_codec_generator_can_plan:
  corrected: 2026-08-13, replacing a claim that a parameter must be a predeclared scalar
  the_wrong_claim: that routetree parses rather than type-checks, so only a predeclared type name is readable and a struct parameter is a diagnostic
  why_it_was_wrong:
    reading: routetree Value.Type already carries the type expression as source text, composites included, and an action needs nothing resolved from it
    validating: bindableType and scalarTypes check a page's parameters because those come from the URL, per decision:route-handler-shape parameter_rule; a JSON payload gives no such reason
    planning: the argument struct is declared and planned by the binding phase, which type-checks, per where_the_wrapper_is_emitted
    net: the limit sits on what the codec generator can plan rather than on what routetree can read, and those are different layers
  works:
    scalars: string, int, int64, bool, float64
    same_package_struct: the ordinary nested case the codec generator is built around, so a struct parameter is the natural shape rather than the refused one
    other_package_struct: through requirement:json-codec-interface, which is what makes it reachable at all
    collections: a slice or map of those, except a foreign element type
  a_named_non_struct_type_is_the_real_gap:
    measured: 2026-08-13, generating over a struct holding a field of type UserID where UserID is a named string
    what_happens: fieldTypeKind maps every same-package identifier to KindStruct, so emission names appendUserIDJSON, decodeUserIDBytes, and decodeUserIDJSON
    none_are_defined: analyzeStruct collects only struct type declarations, so a named scalar never enters the plan and none of those functions is emitted
    result: generated source that does not compile, with no diagnostic, the failure being a missing identifier inside a DO NOT EDIT file
    predates_this: reachable from any request or response model holding such a field, so it is a defect of the codec generator rather than of this feature
    consequence_here: a typed action taking such a parameter meets it, which is the one part of the original claim that survives
  asymmetry_gone: a result type and a parameter type are governed by the same question now, both being planned by the phase that type-checks
the_result_type_must_be_declared_in_the_route_package:
  found: 2026-08-13
  what: rule:same-package-convention lists a model declared in another package as out of scope for analysis, so the codec generator plans a type it finds beside the handler and not one imported from elsewhere
  effect_here: 'func GetUser(...) (User, error) works when User sits in the route package, and produces no encoder when User is a shared domain type from another one'
  weight: this is the constraint most likely to be felt, because a shared domain type is the natural thing to return and a route package is the least natural place to declare one
  not_new_but_newly_binding: the same rule already governs a flat handler's response model, where an author feels it as a convention; here it decides whether the feature works at all for a given signature
  options:
    state_it: make it a diagnostic naming the type and its package, which is honest and cheap and leaves the author to declare a route-local type and convert
    widen_it: plan a type the declaration names wherever it is declared, which is cross-package type planning and is a much larger change than the rest of this feature
    contract_it: requirement:json-codec-interface plus requirement:declared-json-codec, so the declaring package generates its own codec and advertises it, and this module encodes a type it never analyzed
  preferred: the third, because it dissolves the constraint rather than widening the analysis that produced it, and because the analyzer keeps its same-package scope intact
  asymmetry_with_parameters: none; a_parameter_type_is_whatever_the_codec_generator_can_plan settles both sides on one question
there_is_no_success_writer_symbol:
  claimed: the request treats the error writer and the response writer as settings this module already takes
  half_true: routetree.Symbols.WriteError is an emission selector a generated body writes through, and the WriteAPI a framework configures is a generator ResponseWriteCall pattern, which is a call analysis reads rather than one an emitter writes; nothing in Symbols writes a success response
  what_expansion_changes: the seam is no longer a writer taking a value, since the wrapper encodes the value itself; what a framework repoints is the call taking the encoded bytes and a status, which emitWriter already spells as httpbind.WriteJSONBytes
  needed: one selector on routetree.Symbols for that call, defaulting to the runtime's own, so a framework wrapping every success body in its own envelope has one place to do it
  narrower_and_better: a bytes-and-status seam cannot change what the codec produced, so an envelope stays the framework's and the field mapping stays generated
  cheap: the field sits beside WriteError, and decision:action-lowering-profile surface needs nothing else
the_generator_writes_a_response_body:
  was: every decision in requirement:template-server-functions kept the generator out of the response body, stated as a constraint
  now: the glue encodes a result, which is a response body
  precedent_offered: routetree already decodes a route, calls Load, and renders for a page entry point, so the line moves rather than breaks
  the_difference: a rendered page goes through the runtime's own renderer, while an encoded result needs a registered codec that exists only if something discovered the type
  accepted_because: one caller means one response to write and no case analysis, which is the whole reason the shape can be fixed at all
  constraint_amended: requirement:template-server-functions constraints, whose no-response-body line stays true of the handler shape
not_asked:
  raw_shape_replacement: it is load-bearing and it is what a form reaches; a page may declare both, as different functions with different signatures
  result_union: enumerating value, redirect, and regions in one return type makes every author write a framework type where only one member is possible; it stays the shape to return to if a typed action ever needs a redirect
  regions: a handler answering with update regions belongs to the raw shape, and admitting them here would reopen the response question this design just closed
  openapi: unchanged; rule:generated-source-not-discovered already excludes both entry points, and a typed action is still one page's implementation detail
  context_elsewhere: a context in any position but the first keeps the ordinary not-an-input diagnostic
constraints:
  - no reflection at runtime; the codec for every type crossing the boundary is generated
  - the handler shape, its hash, its prefix, and its response rule are unchanged
  - a project declaring no typed action regenerates byte for byte
  - a declared function stays ordinary Go, callable from a test with no registration and no wrapper
acceptance:
  - a declared function of an arbitrary signature gets an entry in the route table and a POST registration at its hashed address
  - a JSON post naming each parameter reaches the function with those arguments and answers with the encoded result
  - a declared function opening with a context.Context receives the request context and does not count that parameter as an input
  - a declared function returning only an error answers with no body on success
  - the wrapper names the generated encoder of the result type, and the emitted body performs no runtime codec lookup
  - a result type reachable from no author-written call still gets its encoder emitted, because the declaration asked for it
  - an error return is written through the same error writer a raw handler's failure uses
  - a declaration naming a non-function, or a function of another package, fails generation naming the position
  - a declaration naming an unexported function generates and answers, since the wrapper sits beside it
  - a parameter the decoder cannot bind fails generation naming its position
  - a typed and a raw action colliding on one hash fails generation, as two raw ones already do
sequencing:
  together: admission, signature reading, and glue emission are one feature with no useful half
  before_all_of_it: the usage source above, since without it the wrapper names an encoder generation never emitted
  separable: requirement:typed-action-published-name and rule:typed-action-call-only, in the reporter's own order
related:
  - requirement:template-server-functions
  - decision:typed-action-declaration
  - rule:typed-action-call-only
  - requirement:typed-action-published-name
  - requirement:action-request-binding
  - rule:response-model-discovery
  - rule:generated-source-not-discovered
  - decision:action-lowering-profile
open_questions:
  - whether a typed action may return a stream, which api:write-stream makes representable but whose callback shape no result list can hold, and which no caller has asked for
  - whether the argument struct is a named generated type, which would let an author's check tags apply, or an anonymous one, which keeps it out of the package's namespace
  - whether a typed action is reachable from a layout package, as a raw one is
```
