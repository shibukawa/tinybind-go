---
id: decision:share-by-module-url
type: decision
title: Share Component Script Code By Module URL, Not By Bundling
---
Transform a component script block and leave its imports as URLs, because the browser's module map already evaluates a shared module once per document, and bundling is what would duplicate it.

```yaml
source:
  - user direction 2026-08-11, that shared code must not be duplicated
  - requirement:authored-language-transform
  - decision:transform-seam-ownership
review_gate: proposed
supersedes: the bundling recommendation in an earlier draft of requirement:authored-language-transform, 2026-08-11, which accepted per-component duplication as a size cost and offered a batch seam as the way out; both were answers to a problem that only exists once bundling is chosen
statement: a component script block is compiled but not bundled; its import specifiers reach the emitted file unchanged, and code shared by two blocks is one module fetched and evaluated once
the_inversion:
  earlier_reading: bundling each block independently duplicates a shared utility into every output, and avoiding that needs code splitting, which needs every entry in one call, which breaks the once-per-value purity of decision:transform-seam-ownership
  what_was_missed: bundling is not required to reach the browser, and it is the step that inlines the shared module into each output
  the_fact: a module is keyed by its resolved URL in a per-document module map and is fetched and evaluated once, however many modules import it
  measured_already: the downstream survey measured exactly this when it established that a module script is not re-evaluated on re-adding, which is the same map seen from the other side; concept:scope-lifecycle already rests on it
  therefore: not bundling is not a compromise that tolerates duplication, it is the option with no duplication in it
  and_the_contract_holds: esbuild's transform form compiles one file without resolving imports, so the call stays once per distinct content and pure, exactly as decision:transform-seam-ownership specifies; no batch seam is needed
what_is_actually_shared:
  update_runtime:
    what: internal/updatecore/runtime.js, a classic script configured at load, which installs names from configuration rather than literals
    shared_how: already one asset loaded once, per requirement:browser-runtime-asset-ownership and requirement:framework-script-contribution, ordered ahead of component script by decision:framework-script-delivery
    duplication: none today and none introduced here
  framework_surface:
    what: the downstream framework's own client object, its navigation, and its signal registry
    shared_how: the same contribution mechanism, with a declared Namespace when it installs a global
    duplication: none; it is one registration and one tag
    already_stated: requirement:browser-runtime-asset-ownership says a framework merges the module's runtime bytes with its own into one asset, so the merge is the framework's and not a bundler's
  application_utilities:
    what: a formatter, a fetch wrapper, a validator; author-written code two or more blocks both need
    shared_how: an authored module under the public tree, imported by URL from each block
    duplication: none, by the module map
    this_is_the_new_case: the three above were already solved, and this is the one the question is really about
  third_party_packages:
    what: an npm dependency imported by a block
    shared_how: not by URL alone, because a bare specifier does not resolve in a browser
    needs: an import map, or a caller-built vendor bundle
    the_only_case_that_needs_more: everything above works with no new machinery, and this one does not
specifier_rules:
  absolute_or_root_relative: resolves against the document origin and is the form that works
  bare: resolves only through an import map; the module neither writes one nor validates against one
  relative_is_a_trap:
    why: an extracted block is written under PublicDir and referenced under PublicURLBase, so './util.js' resolves against the generated file's URL rather than against the template's directory
    consequence: an author writing a path relative to the template gets a silent 404 for a file that exists
    decided: reject a relative specifier in a component script block at generation, naming the block and the resolved URL it would have produced
    why_reject_rather_than_rewrite: rewriting would make the emitted file say something the author did not write, which is the structure_invariance principle concept:build-time-asset-transforms already applies to attribute values
ownership:
  module:
    - the block is a module, which is what makes native sharing possible at all
    - import specifiers reach the emitted file unchanged
    - the relative-specifier diagnostic
    - not bundling, and not owning a bundler; requirement:browser-runtime-asset-ownership already states that no bundling, minification, or JavaScript toolchain enters this module
  caller:
    - the import map, if bare specifiers are wanted
    - whether to bundle anyway for a production build, which stays possible through the transform and is nobody's business here
    - the vendor strategy for third-party code
    - the transform itself, per decision:transform-seam-ownership
  answer_to_who_owns_duplication: mostly neither, because the browser resolves it; the module's one obligation is to keep the block a module and leave its imports alone, and the caller's is the bare-specifier case
costs_accepted:
  request_count: an unbundled graph is more requests than one bundle, and a nested import is a waterfall
  mitigations: HTTP/2 and later make the request count cheap, and modulepreload removes the waterfall
  already_open: decision:framework-script-delivery carries emitting preload or modulepreload hints as an open question, and the asset set is readable before rendering starts, so the hint is derivable rather than guessed
  not_paid_here: this decision neither emits nor forbids such a hint
  no_tree_shaking: an unbundled import ships whatever the imported module contains, where a bundler would drop unused exports; a caller wanting that bundles in its own transform
constraints:
  - the module writes no import map and validates no specifier against one
  - a project registering no transform and importing nothing is unaffected in every respect
  - a block's emitted bytes differ from its authored bytes only by what the caller's transform did
related:
  - requirement:authored-language-transform
  - requirement:component-script-block
  - concept:scope-lifecycle
open_questions:
  - whether the module should emit modulepreload hints for the assets it already knows, which would need head markup and is the decision:framework-script-delivery question one file over
  - whether an import map belongs in requirement:head-merging as a contribution kind, since it is head markup a framework would otherwise write by hand
  - whether a shared authored module may itself be TypeScript, which requires it to travel requirement:element-reference-hook and to be named by a URL that survives the transform's renaming
```
