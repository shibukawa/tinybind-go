---
id: rule:transform-eligibility
type: rule
title: Transform Eligibility
---
A function is transformable when every occurrence of its transport-typed values is one the rewriter recognizes; anything else refuses the function, and refusal propagates to its callers.

```yaml
status: proposed 2026-08-08
unit: one function or method declaration
transport_typed_values: parameters and locals of type http.ResponseWriter or *http.Request, conventionally w and r
admission_set:
  seeds: every handler concept:handler-discovery finds, in all three forms of concept:handler-forms
  closure: any same-package function taking a transport-typed parameter that an admitted function calls
  reason_for_the_closure: decision:backend-build-tag-mode removes the fallback, so a refused shared helper makes the build fail rather than one route slow; a per-function pass would refuse most real handler packages, whose error and render helpers take w and r
admitted_occurrences:
  runtime_call_slot:
    form: the value is an argument to a call the generator recognizes, at a position that call declares as its writer or request slot
    covers: Bind, Write, WriteStatus, WriteError, and the stream entry, plus every framework call registered through the generator call patterns
    generator_change: call patterns declare no transport slots today, so the writer and request positions must become part of the pattern; without them the rewriter cannot know which arguments collapse
  transitive_call:
    form: the value is an argument to another same-package function that is itself admitted
  named_selector:
    form: a selector listed in rule:transform-rewrite-table
    principle: enumerated one by one, never a general rule about methods on the request
refused_occurrences:
  - passed to a function outside the package, which is the shape of tracing, metrics, and session libraries
  - passed to a same-package function that is itself refused
  - assigned to a variable, struct field, map, or slice
  - captured by a function literal
  - address taken, returned, compared, or ranged over
  - type-asserted, including the http.Flusher and http.Hijacker assertions
  - a selector absent from the rewrite table
  - any occurrence in a function whose body the parser cannot read, including one reached through a package selector
default: refuse; an occurrence the classifier does not recognize is not analyzed further and never admitted
propagation:
  direction: a refused callee refuses every admitted caller that passes it a transport value
  fixpoint: iterate until no admitted function refuses, because refusal discovered late invalidates callers already admitted
  reporting: the chain from handler to the refusing occurrence is what requirement:transform-diagnostics must print
outcome_of_refusal: a generation error naming the declaration; there is no fallback, per decision:backend-build-tag-mode
implemented_2026_08_08:
  entry: AnalyzeTransform over a type-checked package, returning the admitted set and every refusal
  admission: every same-package function taking a transport parameter, not only discovered handlers, so a shared helper is carried rather than refusing its callers
  classification: unknown_call, unknown_selector, escapes, type_assertion, and inherited, each with its own remedy string
  fixpoint: a refusal found late re-refuses callers already admitted, and the loop repeats until stable
  found_while_building:
    transport_only_calls:
      problem: WriteError and the request accessors take a transport value and name no model, so no call pattern covered them and every handler reporting an error was refused
      fix: OperationTransportOnly, a pattern that carries transport slots and nothing else; discovery ignores it because the parser conversion drops unknown operations
      scope: WriteError, WriteJSON, WriteJSONBytes, and the request accessors, all of which exist under the same names on both runtimes
    blank_assignment: "_ = r" discards the value and appears throughout real handlers, including this module's own; refusing it would refuse most of them, so it is admitted
    closure_precedence: a captured transport value is surrounded by ordinary recognized calls, so the capture refusal has to outrank the per-call admission or the capture is missed
    same_package_is_not_third_party: a helper in the package is a transitive candidate, so modelling an unknown call needs a genuinely external callee
author_remedies:
  - move the offending work behind a function that takes neither w nor r
  - rewrite the handler in the form of decision:transport-neutral-handler, which has no transport to refuse
  - register the operation as a framework call pattern, so its transport slots become known
related:
  - decision:transport-source-transform
  - rule:transform-rewrite-table
  - concept:handler-discovery
  - concept:handler-forms
```
