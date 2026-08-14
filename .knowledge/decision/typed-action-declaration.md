---
id: decision:typed-action-declaration
type: decision
title: Typed Action Declaration
---
Recognize the declaration that admits a typed server function as a configured call target, matched syntactically in the route package, resolving one identifier argument to a function of that package.

```yaml
source:
  - downstream framework change request 2026-08-13, ask 1
  - requirement:typed-server-action
  - requirement:custom-framework-generation-profile
review_gate: proposed
spelling_is_the_callers:
  fact: the example is `var _ = pw.ServerAction(GetUser)`, where pw is the framework package, and the reporter claims the spelling as theirs
  consequence: this module cannot fix the identifier and should not try; what it fixes is the shape it recognizes and the argument it resolves
  existing_vocabulary: generator CallRegistry already carries a framework's configured wrapper calls as CallOperation values, so a declaration is one more target rather than a new mechanism
  default: the module ships a name of its own for a project using no framework, on the terms decision:framework-integration-seams already sets for every other call
the_declaration_must_compile:
  found: 2026-08-13, not stated in the request
  what: 'var _ = pw.ServerAction(GetUser) is ordinary Go, so pw.ServerAction has to be a real function or the route package does not build'
  consequence: this is a runtime API and not only a generation-time pattern, so the module owes one under its own name and the framework owes one under theirs
  shape_needed: 'something like func ServerAction(fn any, name ...string) any, taking any function value and the optional published name of requirement:typed-action-published-name'
  it_does_nothing: the body is empty; the value is the declaration itself, which generation reads and the compiler checks
  variadic_or_second_parameter: the override has to ride somewhere, and a variadic string is what lets one function serve both forms without two names
  runtime_footprint: a package-level var with a call initializer runs that call at init and keeps the function symbol referenced, which is small and worth stating rather than discovering
  needs_deciding: the name, the package it lives in, and whether the parameter is any or a type parameter
form:
  chosen: a package-level declaration whose value is a call taking the function symbol
  blank_var: '`var _ = ...` costs no name, runs no init, and needs no framework type in the signature it is about'
  rejected_assertion:
    shape: 'var _ tinybind.ServerAction = GetUser'
    why_not: an interface or func type that every action must satisfy fixes the signature, which is the one thing this shape exists not to fix
  symbol_not_name_string: a string would resolve at generation and fail there; a symbol fails to compile, which is earlier and reports better than any diagnostic this module could write
  second_argument: the published name override of requirement:typed-action-published-name rides on the same call, so one declaration carries the whole decision
resolution_must_be_syntactic:
  why: routetree parses with go/parser and never type-checks, because its output is written before the package it describes can compile
  precedent: isHandlerSignature resolves net/http from the file's import list and takesLeadingContext resolves context the same way, per requirement:typed-page-context-parameter as_built.alias
  therefore: the declaration package is resolved from the file's imports and matched by alias, which is what rule:go-types-symbol-identity buys elsewhere with a type checker
  weaker_than_the_type_checker: another package imported under the same alias would be accepted, which is the weakness both shape checks already carry
  new_weight_on_that_weakness:
    what: under the handler shape a misread costs nothing, because exposure is bounded by exported plus handler-shaped plus in a route package
    now: the declaration is the only bound, so a misread declaration publishes a function nobody asked to publish
    mitigation: the argument must resolve to a function declared in the same package, so a misread call still cannot reach outside it
diagnostics:
  not_a_function: a declaration whose argument is not a function declared in this package, naming the position
  not_unexported: not a diagnostic; the wrapper is emitted beside the function, so an unexported one is reachable, per requirement:typed-server-action admission.export_is_not_required
  load: a declaration naming Load, which decision:route-handler-shape already owns
  duplicate: two declarations naming one function, which is a hash collision by another route
where_it_is_read:
  same_pass: routetree DiscoverActions already reads every non-test Go file of the package and walks its declarations, so the var decls are in front of it
  reported_onward: what discovery reads here is consumed by two phases rather than one, since the address is routetree's to register and the wrapper is the binding phase's to emit, per requirement:typed-server-action where_the_wrapper_is_emitted
related:
  - requirement:typed-server-action
  - requirement:template-server-functions
  - rule:go-types-symbol-identity
  - decision:route-handler-shape
  - decision:framework-integration-seams
```
