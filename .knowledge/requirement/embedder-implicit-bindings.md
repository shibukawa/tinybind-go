---
id: requirement:embedder-implicit-bindings
type: requirement
title: Embedder Declared Implicit Bindings
---
Let the embedder declare names that are in scope in every template, one kind of which collapses a preceding slash in a URL attribute when it renders empty.

```yaml
source: concept:template-message-surface, request item E
review_gate: approved 2026-08-16 by the owner
as_built:
  status: implemented in full 2026-08-16, including the path-segment kind
  gate_cleared: the requirement:url-attribute-scheme-safety amendment was approved 2026-08-16 by the owner, on the encoding evidence below rather than on the design alone
  path_segment:
    declaration: ImplicitBinding.PathSegment, and a path-segment binding's provider must return a string, refused at registration otherwise
    gate_exception: isPathSegmentRead, deliberately narrow — only a bare identifier reading a declared path-segment binding qualifies, so no expression built out of one smuggles a plain string into a URL attribute
    still_refused: an ordinary binding, and a plain string, are refused exactly as before; both are tests rather than claims
    collapse:
      helper: htmlbind.URLPathSegment(prefix, value, collapse), which owns the separator so an empty value can remove it
      emitter: the static part before a segment gives up its trailing slash to the helper, or an empty segment would leave a doubled one
      collapse_flag: true only where something follows the segment, because "/{seg}" with an empty seg is the root rather than the empty string; that is the third case the requirement lists and the one a naive rule gets wrong
      scoped_to_url_contexts: a segment written in prose never reaches the helper, which is a test
    encoding:
      rule: percent-encode everything outside the RFC 3986 unreserved set
      dot_segments: "." and ".." are encoded as %2E forms, because the unreserved set contains the dot and a dot segment is an instruction to the resolver rather than a name
      why_stricter_than_a_general_escaper: a segment stands for one name the application chose, so sub-delimiters buy nothing and each is a way to mean something else
      verified: a table of hostile values — path traversal, a leading double slash, a query, a fragment, a scheme, an encoded dot segment — none of which composes a path the template did not describe
    integration_test: the helper inside the URLAttr op, because the emitter puts it there and a value surviving one but not the other would be a hole nothing else catches
    harder_positions_covered: a segment mid-path, two segments in one path, and a segment with no separator before it; the index arithmetic deciding which static part gives up its separator is where a collapse rule goes wrong quietly, so it is rendered end to end rather than asserted on emitted text alone
    protocol_relative: asserted directly, since that failure is what made this mandatory rather than optional
  option: GenerateOptions.ImplicitBindings, a list of ImplicitBinding{Name, Provider, VaryAxis}
  lowering:
    chosen: a binding lowers to a call on an embedder-named Go function taking the render context, the shape requirement:render-value-provider already uses
    what_it_avoided: a new op form per position, and a runtime lookup by string; decision:reflection-free is untouched and the existing Ctx instruction selection does the work
    why_this_is_not_the_withdrawn_external: the reporter dropped `external Lang(): string` because an external call cannot express the collapse, and that objection is about the path-segment half; a binding stays a compile-time identity the emitter recognizes, so the collapse can still be applied at the URLAttr op when that half lands
  instruction_selection: reading a binding makes the instruction a Ctx variant and pulls the context import, by the same paths a context-taking external already uses
  shadowing:
    covered: parameter, val binding, loop variable and loop index
    wider_than_the_request: the request named the parameter list only; scope wins over the binding table by construction, so every binder that could take the name had to refuse it or the binding would quietly stop being what the author reads
  cache: keyed, per decision:implicit-binding-cache-identity as_built; the interim refusal it replaced lasted one increment
  vary: a declared axis folds into Plan.Vary through the path a builtin element already uses
  unused_is_free: a project declaring bindings no template reads generates byte-identical Go, asserted by a test comparing the two outputs
  tests: templates/htmlbind/binding_implicit_test.go, plus an end-to-end compile of generated output against a real provider package
priority: the reporter states must, having withdrawn the external-declaration substitute; see why_not_an_external
problem:
  what: the active language is needed in URLs and attributes in every template and every layout
  cost_without_it: a parameter threaded through every component and every layout chain member
  examples: '<html lang="{langtag}">, <a href="/{lang}/about">, <img src="/assets/{langtag}/hero.png">'
shape:
  declared_by: the embedder, never this module
  names: supplied downstream; this module hardcodes no lang, no langtag, and nothing i18n-shaped
  values: supplied downstream per render
  shadowing: a parameter taking a declared name is an error naming the collision
  kinds: ordinary, and path segment
  reached_set: a plan records which bindings its ops read, folded in foldSlots beside head, assets, and vary, and readable on Fragment and Wrapper
path_segment_collapse:
  rule: a path-segment binding rendering as the empty string collapses one immediately preceding separator slash, in URL attribute contexts only
  cases:
    '"/{lang}/about"': ja gives /ja/about, empty gives /about
    '"/{lang}/"': ja gives /ja/, empty gives /
    '"/{lang}"': ja gives /ja, empty gives /
    prose: '/{lang}/ written in a paragraph is not a URL context and is never collapsed'
  scoped_to_the_kind_not_to_emptiness: an ordinary empty interpolation keeps rendering nothing, so the rule cannot be stated as a property of the empty string
  scoped_to_url_contexts: reuses the classification rule:url-bearing-attributes already carries for the scheme allowlist, rather than adding one
  literal_patterns_only: a fully dynamic `href={u}` is untouched, because this module does not rewrite a URL it did not compose
why_not_an_external:
  withdrawn: the reporter's earlier draft offered `external Lang() : string` as a substitute and has retracted it
  reason: an external call cannot express the collapse, and without the collapse an application whose default language carries no prefix emits `//about`
  severity: a browser reads `//about` as protocol-relative, so the failure is a different host rather than a wrong path
the_blocker_the_request_does_not_know_about:
  verified_2026_08_16:
    where: templates/htmlbind/compiler.go analyzeAttribute, the gate at the isURLAttribute test, and templates/htmlbind/emit.go where a URL-bearing attribute is routed to the URLAttr op
    finding: the url type gate runs per interpolated part, not per whole attribute value; any part of a URL attribute whose type is not url is refused
    therefore: '<a href="/{lang}/about"> with a string binding does not compile today, and fails with "attribute href requires url, got string"'
  the_request_is_wrong_about_current_behavior:
    claimed: '"/search/{q}" with an empty q currently yields /search/, and a collapse rule stated over emptiness would change it'
    actual: that template does not compile at all, for any q, because q is a string in a URL attribute
    net_effect_on_the_design: the compatibility objection the request uses to justify scoping the rule to the binding kind is void
    but_the_conclusion_survives: scoping to the kind is still right, because a future url-typed or otherwise permitted interpolation should not acquire collapsing behavior it never asked for
  consequence_for_sizing: this is not additive. It is an amendment to requirement:url-attribute-scheme-safety, which states that a raw string cannot reach these attributes and treats that gate as the thing standing between an untrusted value and a URL position
  what_the_amendment_must_say:
    permitted: a declared path-segment binding may appear in a URL attribute despite not being url-typed
    justified_by: the value is embedder-supplied rather than request-supplied, so it is not the untrusted input the gate exists for
    not_justified_for: an ordinary implicit binding, which should stay under the gate unless the embedder declares it url-typed
    still_required: the rendered segment must be constrained — an allowlisted character set or percent-encoding at emission — because a binding whose value reaches a path unescaped is a path-traversal and open-redirect surface, and the reporter resolves locales from request input
    the_case_that_makes_it_concrete: a locale taken from an Accept-Language header or a path prefix is attacker-influenced by definition, so embedder-supplied does not mean trusted content
  where_the_collapse_lives: the URLAttr op assembles the value and applies the render's policy, so the collapse belongs there beside the scheme check rather than in the value closure, matching the reasoning decision:url-context-escaper already recorded for why escaping cannot stay in the closure
settled:
  declaration_site: per compilation through data:generator-options, plus the reached set recorded in the plan and folded up the chain; decision:implicit-binding-declaration-site, 2026-08-16
  collision_report: at the parameter position, naming the declared binding and where it was declared, matching how decision:cache-scope-declaration reports a declaration the caller cannot satisfy
  vary_axis: one optional axis per binding declaration, named by the embedder
  values_at_render: an existing render option, as in concept:framework-template-extensions render_time
not_a_parameter_has_consequences_beyond_scope:
  cache: settled by decision:implicit-binding-cache-identity, which keys a cached component on the bindings it reaches and folds a declared axis into the response vary
  redraw: requirement:component-redraw-endpoint contracts that the browser passes every input, so a binding is the first input it must not pass; it is derived from the request on that path, per rule:redraw-input-trust
  declaration_shape_it_adds: a binding declaration carries its kind, ordinary or path segment, and optionally one vary axis; the axis is the embedder's to name because only it knows whether the value is recoverable from the URL
  reading: all three follow from the same property this requirement exists to provide, so they belong here rather than downstream
relation_to_render_value_provider:
  overlap: requirement:render-value-provider already carries a per-request framework value into markup
  difference: that value never enters template scope and is placement-checked behind a registered element; this one is an ordinary name any expression may use
  why_the_difference_matters_here: putting a framework value in scope is exactly what makes the URL gate question unavoidable, since scope means every position, including the ones the gate protects
  not_a_replacement: a builtin element cannot appear inside an attribute value, so requirement:builtin-element-lowering cannot express '"/{lang}/about"'
acceptance:
  - a declared binding is usable in every template with no parameter and no import
  - a parameter shadowing a declared binding fails generation at the parameter
  - a path-segment binding rendering empty collapses one preceding slash in a URL attribute
  - the same binding rendering empty in text collapses nothing
  - an ordinary binding rendering empty collapses nothing anywhere
  - a project declaring no binding generates byte-identical Go
  - a fragment reports the bindings it reaches, and a composition reaching one the caller supplies no value for is detectable before it renders
  - a path-segment value carrying a slash, a scheme, or a percent sequence cannot compose a URL the template did not describe
```
