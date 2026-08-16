---
id: concept:template-message-surface
type: concept
title: Template Surface For Message Catalogs
---
Give a template a way to spell a message reference and resolve it against symbols another tool generated, while this module learns nothing about translation.

```yaml
evidence:
  source: downstream framework request, Popcorn Wave at github.com/shibukawa/popcornwave
  received: 2026-08-16
review_gate: approved 2026-08-16 by the owner
scope:
  stated: the HTML output dialect only; requirement:sql-template-v1 and the DynamoDB dialect are untouched
  enforcement_found_2026_08_16: the recognizer sits in the shared body grammar, so every format parses `{t id}` and only the HTML dialect resolves one; a format that does not has to refuse it by name
  as_built: sqlbind refuses at analysis, naming the reference and its position, rather than reaching the default arm and reporting a Go type at line 1
  general_lesson: a shared-grammar addition is in every dialect the moment it parses, so the scope line is a claim each dialect has to carry rather than a property of where the feature was built
boundary:
  owns: syntax and name resolution
  owns_no: i18n semantics
  test_the_reporter_states: a reader of this module's source who can tell the feature exists for translation means the line was drawn wrong
  consequence: no catalog parser, no plural logic, no locale type, and no change to escaping in any form, which decision:message-hole-lowering preserved by refusing the one design that would have broken it
upstream_items:
  requirement:message-scope-declaration: A and C, the header declaration and how a bare or dotted id resolves
  decision:message-reference-syntax: B, the `{t id}` body form and why it stays a string
  requirement:message-hole-binding: D, rich text whose markup is supplied at the reference
  requirement:embedder-implicit-bindings: E, embedder-named scope values and the path-segment kind
  requirement:message-symbol-resolution: F, resolving a reference and checking its arguments
  requirement:template-parse-introspection: G and H, what a parse must expose to the tooling that reads it
downstream_owns:
  - catalog format, loading, and composition across framework, package, and application
  - what a resolved symbol is, and the code behind it
  - CLDR plural rules and fallback chains, both flattened at generation so no runtime branch survives
  - segment tables and generated typed accessors, so adding a locale emits data rather than code
  - id assignment, slugs, drift detection, and extraction policy
  - rewriting sources in place during extraction, consuming requirement:template-parse-introspection
  - reporting a hole mismatch, and producing the segment list this module interleaves, per decision:message-hole-lowering
  - the names and values of the implicit bindings; this module never learns the word locale
  - locale resolution, URL modes, and Vary
  - link and prefix diagnostics against the route table, as diagnostics only
  - localized assets, language switcher, and hreflang
three_handoffs_rather_than_splits:
  offsets: this module exposes a source range, the reporter performs the rewrite
  bindings: this module provides the mechanism, the reporter supplies the names
  holes: the reporter produces the segments, this module drives the interleaving; inverted 2026-08-16 by decision:message-hole-lowering, and the only handoff whose direction changed
  why_each_stops_there: the other side needs the catalog to go further, and the catalog is never read here
relation_to_the_other_downstream_surfaces:
  same_reporter: concept:framework-template-extensions, which owns builtin elements, script contribution, and requirement:render-value-provider
  why_this_is_not_that: a builtin element is a registered element lowered to a framework symbol, placement-checked and never in template scope; a message reference is an expression an author writes anywhere an expression is legal
  overlap_worth_watching: requirement:embedder-implicit-bindings puts framework-supplied values in template scope, which requirement:render-value-provider deliberately does not; that is the one place the two surfaces disagree by design
sequencing:
  1: requirement:message-scope-declaration, decision:message-reference-syntax, requirement:message-symbol-resolution, and the reference half of requirement:template-parse-introspection
  2: the offset half of requirement:template-parse-introspection, unblocking extraction
  3: requirement:embedder-implicit-bindings, unblocking localized URLs
  4: requirement:message-hole-binding, which depends on 1
  reporter_note: step 1 alone covers the large majority of real messages, so it is enough to start building against
version_pin:
  reporter_side: an html_i18n_baseline key beside its existing html_template_baseline, html_async_baseline, and html_live_baseline
  our_side: nothing; those keys are the reporter's release records, and requirement:html-rendering-compatibility already states the baseline this module tracks
  fact_they_encode: a template using `messages` or `{t ...}` fails to parse on any earlier release
open_contracts:
  answered_here_2026_08_16:
    contextual_keyword: decision:message-reference-syntax verified against the parser; the mechanism the request asks for is the one every existing directive already uses, so `t` need not be reserved
  decided_by_the_owner_2026_08_16:
    header_form_or_annotation: the header form, per requirement:message-scope-declaration
    id_lexical_form: a hyphen is permitted, per decision:message-reference-syntax, which makes the id-to-symbol mapping a supplied table
    cache_identity: an implicit binding is keyed and varied on, per decision:implicit-binding-cache-identity, both inside the component cache and outside it
    consequential: the leading argument of a message symbol is itself an implicit binding, which the cache decision forced rather than chose
    hole_lowering: the segment list, per decision:message-hole-lowering; the closure signature the reporter asked about no longer exists
    binding_declaration_site: per compilation plus a reached set folded up the chain, per decision:implicit-binding-declaration-site, which also settles the shadowing error and one optional vary axis per binding
  hole_syntax: a block whose holes are named by their own tags, per decision:message-hole-lowering as_built; the hole attribute is the escape hatch for two holes sharing a tag
  still_open:
    none_upstream: every contract the request raised is answered, and every item A through H is built
  raised_here_and_not_in_the_request:
    url_type_gate: requirement:embedder-implicit-bindings; a string cannot reach a URL attribute today, which makes E an amendment to requirement:url-attribute-scheme-safety rather than an additive feature
    id_lexical_form: decision:message-reference-syntax; the request's own examples use a hyphen, which the expression lexer reads as subtraction
    who_escapes_a_streamed_message: raised against the request's closure form and resolved by decision:message-hole-lowering, which keeps escaping here rather than documenting a handoff
interactions_neither_side_has_raised:
  found: 2026-08-16, reading this module against the request
  cache_key_does_not_see_a_locale:
    mechanism: decision:cache-key-derivation builds a key from the plan fingerprint and every declared parameter, and requirement:embedder-implicit-bindings is by definition not a declared parameter
    failure: a component carrying a message reference renders in one language and is served from the cache to a reader of another, silently and for the whole ttl
    already_anticipated_and_now_void: that decision's constraints say locale variation must be a declared parameter or the component must not be cached, which was written assuming a locale would be a parameter; this feature exists to stop it being one
    the_partial_escape_hatch: decision:cache-scope-declaration prefixes a scope value the embedder composes, so a private component could carry user and language together; a component declaring public is prefixed with nothing and is exactly the hole
    precedent_for_the_other_answer: decision:cache-component-declaration makes a component ineligible when a builtin element declares a request property, rather than keying it
    decision_needed: whether a reference or an implicit binding enters the key, makes the component ineligible, or is left to the embedder's scope value with the public case refused
    urgency: it is a correctness bug rather than a missing feature, so it has to be settled with step 1 and not after
  redraw_must_not_take_a_binding_from_the_client:
    mechanism: requirement:component-redraw-endpoint contracts that a reloadable component is a function of its declared parameters and that the browser passes all of them
    consequence: an implicit binding is not declared, so the endpoint derives it from the request; rule:redraw-input-trust forbids the other reading
    decision_needed: stated explicitly, because the contract as written says every input arrives in the query string
  every_compile_path_must_be_given_the_seam:
    precedent: requirement:route-package-context-externals, where GenerateOptions.ContextExternals was filled on the templates path and not the route path, so a shipped feature was simply absent on filesystem routes and was found downstream
    applies_to: the message symbols requirement:message-symbol-resolution resolves against, and the binding names requirement:embedder-implicit-bindings declares
    checked_2026_08_16: neither path passed either seam, exactly as predicted; both now do, and a test per path asserts it rather than a reviewer remembering to
    what_the_check_also_found:
      analysis_only_entries: Signatures, ComponentScripts and ActionRefs run a full analysis with no options, so a template using either feature could not be read at all by the entry points a framework calls before generating
      messages_fixed_by_moving_the_check: the symbol table is a generation input, so unknown-id and argument checking moved out of analysis into generation, matching how an unresolved ServerActions name is an emission error rather than an analysis one
      bindings_could_not_move: a binding's name has to be known during analysis or it is an unknown identifier, so the three entries gained a variadic AnalysisOption and routetree threads it through AnalyzeWith, PageComponent and LayoutComponent
      reading: an option that only generation needs can be checked late; an option that changes what a name means cannot, and the two have to be told apart before a seam is added
  a_library_component_carries_its_own_scope:
    mechanism: requirement:message-scope-declaration is per file, so a component shipped under decision:library-component-seams resolves against the catalog of the package that defined it
    consequence: the requirement:message-symbol-resolution generation order runs per package rather than once, and an application composes catalogs it did not write, which the reporter's table already lists as its own concern
    decision_needed: whether an external component declaration says anything about messages; the answer should be no, and saying so prevents a scope from being threaded through an import
non_goals:
  - any message storage, lookup, or formatting in this module
  - a locale type, a locale parameter, or a locale-aware runtime path
  - reading the catalog at generation time for any purpose, including diagnostics
```
