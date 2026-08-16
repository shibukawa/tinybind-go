---
id: decision:implicit-binding-cache-identity
type: decision
title: An Implicit Binding Is Part Of Cache Identity
---
Key a cached component on the implicit bindings it reaches, and report those bindings as response vary axes, so output that differs by an embedder-supplied value is neither reused across values nor cached outside the component as if it were one.

```yaml
source:
  - requirement:embedder-implicit-bindings
  - concept:template-message-surface interactions_neither_side_has_raised, cache_key_does_not_see_a_locale
  - owner decision 2026-08-16, both halves: the component cache and every cache outside it
review_gate: approved 2026-08-16 by the owner
as_built:
  status: implemented 2026-08-16, both halves
  inside_the_component:
    field: CachePolicy.Bindings, a func(context.Context) string appending the framed value of every binding the component's call graph reads
    why_a_closure_and_not_a_value: a binding is read through its provider at render time, so the key cannot be built from anything the plan holds statically
    position: after the scope prefix and before the parameters, asserted by a test that the scope stays a key prefix
    order: declaration order, taken from the order the embedder listed the bindings, so a key does not depend on where an author happened to write the first read
    reached_not_declared: transitiveBindings walks the same call graph transitiveVary does
    nil_is_the_old_key: a component reading no binding produces exactly the key it produced before the field existed, asserted rather than assumed
  outside_the_component:
    axis: a binding's VaryAxis is appended to the reading component's vary list, so it folds into Plan.Vary through the path a builtin element already uses
    no_new_machinery: nothing was added to the fold; the axis is one more entry in a list that already folds
    empty_axis_contributes_nothing: which is what an application carrying the value in its URL declares
  what_this_replaced: an interim refusal that made a component reading a binding ineligible for storage; it shipped for one increment as the answer that could not be silently wrong, and keying superseded it in the same session
  message_context:
    built: GenerateOptions.MessageContextBinding names the binding supplying a message symbol's leading argument, and a reference records a read of it
    effect: the chosen design's claim holds in code — a cached component carrying a message keys on the binding with no special case, because the reference is an ordinary reader
    typed_provider: BindingProvider.Result carries the catalog's own locale type across the boundary without this module learning it
  tests: htmlbind/cache_test.go for the key property, templates/htmlbind/binding_implicit_test.go for the emission, plus an end-to-end compile of a cached component's generated policy
naming_discipline:
  rule: nothing here names a language
  reason: concept:template-message-surface states the boundary test as a reader of this module's source being unable to tell the feature exists for translation, and a cache axis called locale fails it in the one place a reviewer looks first
  what_the_module_knows: a declared binding has a value, output that reads it depends on it, and a cache must therefore distinguish it
inside_the_component:
  what_changes: decision:cache-key-derivation gains a section holding the framed value of every implicit binding the component reaches
  framing: the existing length-prefixed form, so a binding value cannot spell out another key part
  position: after the decision:cache-scope-declaration scope prefix, never before it, because scope is first so that deleting a scope stays a prefix range
  order: declaration order, as parameters already use, so the key is stable across regenerations that add no binding
  reflection_free: decision:reflection-free is untouched; a binding value is a string the runtime holds before the render starts
  reached_not_declared:
    rule: a component keys on the bindings its call graph reaches, not on every declared one
    walk: the existing one, beside reachesAwait, reachesPerRequest, and reachesDeclaredPrivate
    why: a project declaring three bindings should not make every cached component miss on all three
  supersedes: the decision:cache-key-derivation constraint that locale variation must be a declared parameter or the component must not be cached, which was written when the only way to carry such a value was a parameter
departure_from_the_vary_precedent:
  precedent: decision:cache-component-declaration settled that a component reaching a builtin element declaring a request property is ineligible for storage rather than keyed
  this_decision: the opposite, and the difference is not a change of mind
  the_line: when the value exists
  provider: requirement:render-value-provider produces its value during the render, from the context, through a symbol this module calls but cannot evaluate while building a key, so there is nothing to key on at key time
  binding: an implicit binding's value is supplied at the render call and is in hand before the first byte, so it is keyable exactly as a parameter is
  consequence: the earlier rule stands unmodified; the two cases differ by a property of the value rather than by policy
  reading: this is why a binding is the right mechanism for something a cached page depends on, and a provider is the wrong one
what_a_message_reference_depends_on:
  problem: decision:message-reference-syntax spells a reference that names no binding, so the reach walk finds nothing, and this module must not learn that a message varies by anything
  chosen: make the dependency structural — the embedder declares which implicit binding supplies the leading argument of every generated message symbol, so `{t title}` lowers to a call taking that binding and the existing walk finds it with no special case
  what_it_also_answers: requirement:message-symbol-resolution what_a_locale_parameter_is_here, which left the origin of that leading argument open; it is a binding, and the two features stop being independent
  rejected_all_declared_bindings:
    what: a component reaching any reference keys on every declared binding
    why_not: it works and needs no new declaration, but it over-keys silently and hides the dependency from every diagnostic, so an author reading a miss rate has nothing to look at
  rejected_a_reference_is_ineligible:
    why_not: it makes the most reused pages the ones that cannot be cached, which is the failure decision:cache-scope-declaration already recorded for its own default
public_becomes_safe:
  before: a component declaring scope public carries no scope prefix, so an embedder composing a language into the scope value protected only the private case
  after: the binding is keyed regardless of scope, so a shared page is correctly distinguished
  consequence: decision:cache-scope-declaration needs no refusal for a public component reading a binding, and the public case stops being the hole
outside_the_component:
  mechanism: the existing vary path — requirement:builtin-element-registration rolls a declared axis up the call graph into Plan.Vary, readable as Fragment.Vary, Wrapper.Vary, and MergeVary
  what_changes: a binding declaration may carry a vary axis, which folds through that same path with no new machinery
  header: written by the caller, per decision:caller-writes-the-response
  the_axis_is_the_embedder_s_to_name:
    reason: whether the value is recoverable from the URL decides whether a header axis is needed at all, and only the embedder knows how it resolved the value
    url_resolved: an application carrying the value in a path prefix declares no axis, because two languages are already two URLs and a vary axis would only fragment an intermediary's cache
    negotiated: an application resolving from a request header declares that header, or nothing downstream can cache the page correctly
    never_inferred: this module does not guess an axis from a binding's presence, because the wrong guess is a correctness bug in one direction and a cache-hit-rate bug in the other
  redraw: requirement:component-redraw-endpoint answers a GET, which an intermediary may cache, so the same axis applies to that response and is folded from the same plan
operational_cost:
  what: entry count multiplies by the number of distinct binding values, on top of the per-scope multiplication decision:cache-scope-declaration already publishes
  where_it_bites: the bundled MemoryCache takes a maximum entry count and evicts in approximate insertion order, so a cap sized before this change thrashes after it
  disposition: a number to publish with the release note, not a reason to key less
acceptance:
  - a cached component reading a binding produces a distinct entry per binding value
  - the same component reads a different value and does not serve the earlier render
  - a component reaching no binding produces the key it produces today, byte for byte
  - a project declaring no binding regenerates byte-identical Go
  - a component declaring public and reading a binding is keyed per value rather than refused
  - a binding declared with a vary axis reaches Plan.Vary through the existing fold, and one declared without contributes none
  - deleting a scope remains a key-prefix range
```
