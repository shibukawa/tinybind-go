---
id: requirement:message-symbol-resolution
type: requirement
title: Message Symbol Resolution And Argument Checking
---
Resolve a message reference to a generated Go symbol and check its arguments at generation, so a missing or mistyped argument fails the build rather than the render.

```yaml
source: concept:template-message-surface, request item F
review_gate: approved 2026-08-16 by the owner
as_built:
  status: implemented 2026-08-16, argument checking by name and arity
  table: GenerateOptions.Messages, keyed by resolved id, holding MessageSymbol{Package, Alias, Name, Params}
  params_are_load_bearing: a reference names its arguments and Go takes them positionally, so the declared parameter list is what fixes the call's order rather than a convenience
  unknown_id: a generation error naming the resolved id, the rule ServerActions already follows so a template naming a message nobody resolved cannot ship as inert output
  reporting_first: MessageRefs answers before any table exists, which is the order a caller needs since the report is what the table is built from
  import: written only for a package a reference actually resolves into, so a project using none regenerates unchanged
  leading_argument:
    seam: GenerateOptions.MessageContextBinding, naming a requirement:embedder-implicit-bindings entry
    superseded: an interim MessageContext holding a raw Go expression, which lasted one increment
    why_the_interim_failed: the expression was emitted inside a plan value closure whose bound names come from what the template's own expressions reach, so it could name neither a local nor the render context, which left a per-request value unexpressible
    what_the_binding_fixes_at_once: the reference reads the binding, so the cache key, the vary axis and the context-carrying instruction all follow from an ordinary read, and no rule about messages appears anywhere in the cache
    typed_result: BindingProvider.Result lets the provider return the catalog's own locale type; such a binding cannot also be written into markup, because this module has no escaping rule for a type it has never seen
    replaced_by: requirement:embedder-implicit-bindings, which is where the value is meant to come from
  not_built: type checking of argument values against the symbol's Go types, which is the syntactic-versus-go-types choice recorded below; today the value expression is type-checked as an ordinary expression and its fit to the parameter is not
  found_while_building:
    walk_missed_the_arguments: the emitter's walkExpr had no message case, so an argument reaching a context-taking external would have emitted a plain instruction whose closure then names a context it was never given; fixed and covered by TestMessageArgumentReachingAContextExternal
    why_it_was_easy_to_miss: the message itself needs no context, so nothing about the reference suggests the instruction might; it is the argument that decides, exactly as it does for any other expression
  end_to_end: generated output compiles against a real message package, with a hyphenated id resolving to a Go symbol, arguments ordered by the declared parameter list, and an attribute-position message escaped by this module
  tests: templates/htmlbind/message_test.go
priority: must; without it decision:message-reference-syntax is an unchecked string splice
shape:
  reference: '{t item_count, n: count}'
  symbol: 'func ItemCount(loc Locale, n int) string'
  checked: arity, argument names, and argument types
  requested_reuse: the path already taken for external declarations
generation_order:
  chain: catalog, then message package, then template compile
  owner: the reporter sequences it
  our_assumption: the symbols exist when a template compiles
  what_breaks_if_not: the diagnostic degrades to an unresolved name, which is worse than the runtime surprise this requirement exists to remove
the_reuse_is_not_as_direct_as_it_looks:
  finding: requirement:render-context-externals states the external check stays on the parsed Go parameter list, syntactically, because it runs before the package compiles
  therefore: reusing that path gives syntactic argument checking against parsed source, not go/types checking
  is_that_enough: for arity and for named argument matching, yes; for type identity across aliases and named types, no, and requirement:alias-transparent-type-analysis exists because that difference has bitten before
  the_stronger_option: type-check against the compiled message package, which the generation order above makes possible in a way it is not for an application's own externals
  the_cost_of_the_stronger_option: the template compile then depends on a package that compiles, so a broken message package reports as a template error, and requirement:incremental-generation and rule:generation-input-hash gain an input they do not model today
  recommendation: start syntactic, matching the external path exactly as the request asks, and record the limitation rather than discovering it
  symbol_identity: whichever is chosen, requirement:strict-symbol-identity and rule:go-types-symbol-identity say the symbol is identified by resolved identity rather than by selector name, and an aliased import of the message package must not defeat resolution
id_to_symbol_is_a_supplied_table:
  cause: decision:message-reference-syntax permits a hyphen, so an id is not a Go identifier and a symbol name cannot be the id
  rule: the mapping arrives with the symbols, as data, rather than being computed here from a naming convention
  precedent: data:builtin-element-definition already names a Go symbol for an element whose spelling is kebab-case, for the same reason
  what_it_buys: the reporter's slug policy stays downstream, and a renamed symbol is a data change rather than a convention this module has to match
  what_it_costs: an id present in the table and absent from the package is a link error rather than a generation error, unless resolution checks the table against parsed source
what_a_locale_parameter_is_here:
  fact: the generated signature carries a first parameter the reporter calls Locale
  our_position: it is an opaque leading parameter this module passes through, exactly as requirement:render-context-externals passes a leading context it never inspects
  therefore: this module does not learn the type, does not name it, and does not have a rule about it
  where_the_value_comes_from:
    decided: 2026-08-16, as a consequence of decision:implicit-binding-cache-identity rather than on its own merits
    rule: the embedder declares which requirement:embedder-implicit-bindings binding supplies it, and a reference lowers to a call taking that binding
    why_it_was_forced: a cached component must key on what its output depends on, and a reference naming no binding is invisible to the reach walk that computes the key
    side_benefit: the two features stop being independent, and no rule anywhere has to say that a message depends on anything in particular
diagnostics:
  unknown_id: reported at the reference position, naming the resolved id rather than the written one, since requirement:message-scope-declaration may have qualified it
  wrong_argument: reported at the argument, not at the reference
  standard: requirement:analysis-diagnostics and rule:analysis-diagnostics-check apply unchanged
acceptance:
  - a reference to an existing symbol with correct arguments compiles
  - a missing argument, an extra argument, and a misnamed argument each fail generation at the argument
  - an unknown id fails generation at the reference, naming the resolved id
  - a project using no reference compiles with no dependency on any message package
```
