---
id: requirement:element-reference-hook
type: requirement
title: Element Reference Hook
---
Match a registered element and attribute during HTML lowering, hand the static value to a generation-time transform, and replace the value or the element with what it returns.

```yaml
priority: should
source:
  - concept:build-time-asset-transforms
  - user design discussion 2026-08-01
review_gate: implemented 2026-08-02
conversion_timing:
  round_one:
    when: 2026-08-02
    built: a transform returning converted bytes inline
    problem: every encode ran inside a sequential template compile, and a build reconverted everything it had converted before
  round_two:
    raised_by: user, asking whether conversion could be a separate phase run later across goroutines
    built: a resolver naming outputs, with the declared work batched to the caller after every template compiled
    gained: concurrency and a caller-owned cache
    lost: a resolver cannot see converted bytes, so the size-regression rule is not expressible
  round_three:
    when: 2026-08-02, same day
    decided_by: user, who wants the size comparison, so inline conversion is correct and what it needs is a cache of its inputs and outputs
    built: inline again, plus an optional CacheKey the hook declares and a store the caller configures
    why_it_settles: the rewrite depends on the conversion's outcome, so only the converting call can decide it; the expense of converting is a caching problem, not a phasing one
  what_the_cache_holds: the whole outcome, including a decision to decline, so an encode that once lost to its source is never rerun to rediscover that it loses
  remaining_cost: a cold cache converts serially, since the compile that needs each value is sequential; an incremental build converts nothing
  cache_honesty: a key is only as good as the Params string a hook writes, so an encoder upgrade nobody mentions serves stale bytes; that is the caller's to state and is the same class as an under-reported read set
as_built:
  where: templates/htmlbind/hook.go, GenerateOptions.ReferenceHooks, Result.Produced and its siblings; generator/hooks.go for the run memo and the conversion cache
  results_shipped: value and skip, plus head entries from 2026-08-04 per requirement:hook-head-contribution
  transform_concurrency: a transform is called from one goroutine unless the caller sets generator Options.ConversionWorkers, which is opt-in precisely because being pure is necessary and not sufficient for being safe under requirement:parallel-conversion
  element_result_deferred:
    what: the markup-replacing result is designed and not built
    why: no shipped policy uses it, decision:transform-seam-ownership records the image case rejecting the use it was added for, and it needs fragment parsing plus an insertion-context check that the value result does not
    consequence: a caller wanting a picture wrapper writes the picture in the template and registers a second hook on source srcset, which is the shape that decision already prefers
  registration_time_only: two hooks sharing an element and attribute pair pass registration; the collision is a use-time diagnostic, as designed
  strict_option: GenerateOptions.StrictReferenceHooks promotes the dynamic-value report to a compile error
  two_memo_layers: within a run, one transform call per hook and distinct value, widened from the module to the run by the generator; across runs, the on-disk store keyed by what CacheKey named
  cache_optional: a hook declaring no CacheKey, or a run configuring no directory, converts every build, which is correct and slow
  sources_recorded_regardless: the files a CacheKey names are hashed as build inputs whether or not caching is on, because leaving them out would let an edit go unnoticed
  escaping_as_built: the quote and both angle brackets are escaped in the replacement and the ampersand is not, which is exactly what the authored attribute path already does, so a rewritten attribute stays byte-comparable with a hand-written one
  found_while_building:
    options_became_unhashable: a hook holds func values and rule:generation-input-hash marshals the options value, so registering one silently turned the incremental skip off; closed by a ReferenceHook JSON form carrying the registration and not the behavior, which the generator executable hash already covers
surface:
  timing: passed to the generate command in data:generator-options, like the requirement:builtin-element-registration whitelist
  model: data:element-hook-definition
  scope: every template file in the generation unit, ambient like a builtin element and unlike requirement:template-file-scope declarations
  contributors: the framework building the generate command, or the application building its own
  no_process_global: no package init and no runtime registry
transform_identity:
  form: a Go func value in the options snapshot, called inside the generator process
  not_a_symbol: unlike requirement:render-value-provider, which names a rule:go-types-symbol-identity symbol in the target module for generated code to call at render time
  reason: this transform runs during generation and never appears in generated output, so go/types resolution buys nothing
  cli_consequence: a hook is not expressible as a bare flag; a stock hook ships inside a generate command and a flag selects it
matching:
  element: exact lowercase element name, such as img
  attribute: exact attribute name, such as src
  value: a prefix, a glob, or a predicate func over the static value
  static_only: an attribute whose value is a template expression never matches, because the value does not exist yet
  foreign_content: an element inside an SVG or MathML subtree is out of scope, following decision:builtin-element-syntax
  no_ancestor_constraint: unchanged from decision:builtin-element-syntax; the ancestor may live in a caller, a layout, or a slot fill
reach:
  body_markup: every template file in the generation unit
  head_declaration: a script or link element in a component head declaration, which requirement:static-asset-extraction passes through unchanged when it names an external URL
  why: an entry point referenced from a head declaration is the ordinary place to write one, and a seam that skipped it would miss the case decision:transform-seam-ownership records as case_typescript
  document_shell: decision:html-document-shell markup, which is outside any component and is where a scaffolded page names its own scripts
  order: the rewrite happens before requirement:static-asset-extraction decides the element is a passthrough, so extraction sees the rewritten URL
overlap:
  rule: two hooks matching one attribute occurrence fail generation, naming both and the source position
  why_not_first_wins: registration order would decide output, and requirement:component-asset-requirements already requires order-independent bytes for `--check`
  detection: at use time, not at registration time, because two prefixes cannot be compared without a value
results:
  value:
    returns: a replacement attribute value
    lowering: folds into the surrounding static run exactly as the original value did
    cost: none at render time
    escaping: the replacement takes the escaping of its own attribute position under rule:template-context-safety, so a transform cannot inject markup through a value
  element:
    returns: replacement markup for the whole element
    lowering: parsed at generation time in the insertion context of the element it replaces, like the markup shape of requirement:builtin-element-lowering
    checks: the replacement must parse in that context, and each of its holes takes its own escaping
    structural_cost: a result changing the element tree breaks CSS combinators, flex and grid item identity, and structural JavaScript written against the authored markup, and the generator cannot detect any of it because global stylesheets and scripts are outside its view
    consequence: decision:transform-seam-ownership records the image case rejecting the picture-wrapping use this result was added for
    remaining_use: a hook whose replacement preserves the tree, such as one adding attributes to the same element, and a project that has verified its own CSS
    guidance: prefer the value result; an element result changing the tree is a project-level decision, not a default
  skip:
    returns: nothing
    meaning: the predicate matched but the transform declined, such as an SVG source or a derived file larger than its original
    effect: the element is emitted unchanged and the reason enters the build report
memoization:
  key: element, attribute, and value
  effect: twenty img elements naming one file call the transform once, so requirement:derived-asset-generation encodes once
  scope: the generation unit
determinism:
  contract: a transform is a pure function of its declared inputs, and the generator treats it as one
  measured: identical inputs produce identical rewritten bytes and identical derived files
  violation: nondeterminism surfaces as a `--check` failure, not as a silent difference
diagnostics:
  - a matched value the transform reports as unresolvable, naming the file, line, and column of the attribute
  - two hooks matching one occurrence
  - element-result markup that does not parse in the insertion context
  - a hook registered for an element and attribute pair already registered
dynamic_value_report:
  problem: a hook registered for img src cannot see '<img src={item.Image}>', so a page is half optimized and nothing says so
  rule: an expression-valued attribute at a registered element and attribute pair is reported once per occurrence
  level: a report line by default, promotable to an error by option, because a project may legitimately mix authored and user-supplied images
  not_solved_here: rewriting a runtime value needs a lookup table and a runtime call, which concept:build-time-asset-transforms excludes
ordering_in_the_pipeline:
  after: HTML parsing, attribute typing, and requirement:builtin-element-registration resolution
  before: static byte coalescing and requirement:static-whitespace-normalization
  reason: a rewritten value must be foldable, and a replaced element must be parsed before the surrounding run closes
interaction:
  cache: a rewritten value is static, so requirement:component-output-cache and requirement:layout-reuse-boundaries are unaffected, unlike a per-request builtin element
  capabilities: a hook declares none, adds no data:component-render-capabilities entry, and never forces requirement:html-runtime-bootstrap
  builtin_element: a hook may match a builtin element's own rendered attribute only through the definition, not through this seam; the two do not compose in the first shape
constraints:
  - a project registering no hook regenerates byte-identical Go and writes no extra file
  - the transform runs at generation time only; nothing is looked up per request
  - source positions survive the rewrite, so a diagnostic points at the template and not at generated markup
  - a hook never emits document tags and never replaces the decision:html-document-shell
acceptance:
  - a hook registered for img src with prefix '/public' rewrites every static match in the unit and leaves every other img untouched
  - the same page regenerates identical bytes, and regenerating with a different registration order does the same
  - one file referenced twenty times calls the transform once
  - a transform returning hostile bytes cannot break out of the attribute value
  - an img whose src is an expression is reported and emitted unchanged
  - two hooks matching one attribute fail generation naming both
  - an element result parses in the insertion context of the element it replaces, and the surrounding static run stays folded
  - a unit using only value results produces markup whose element tree is identical to the authored one
related:
  - requirement:derived-asset-generation
  - requirement:builtin-element-lowering
  - requirement:configurable-generator-discovery
  - decision:framework-integration-seams
answered_2026_08_04:
  reviewed_by: the first downstream project to build against this seam, reading the shipped API rather than this catalog
  head_entry:
    was: whether a hook may contribute a head entry, which a preload link would need
    now: yes, and it is the only thing that project needs and cannot have; taken as requirement:hook-head-contribution
    driving_case_is_stronger_than_the_one_asked_about: a preload is a latency loss and a CSS module's companion stylesheet is a correctness one
  hyphenated_element:
    was: whether a hook may match an attribute on a hyphenated passthrough element
    now: no, closed rather than deferred; that project keeps its reference sites to img src and script src, and nothing else has asked
    reopen_when: a caller names an attribute on a component-supplied element as a reference site
  srcset:
    was: whether srcset is matched as one value or parsed into candidates by the seam
    now: as one value, which is what already happens; Value replaces the whole attribute string, so a transform parses the descriptor list and reassembles it, and the escaper touches only the quote and the angle brackets, which a descriptor list never contains
    no_change_needed: the seam does nothing here, and whether to do it is the caller's question rather than a capability question
  element_result:
    was: not an open question but a built-and-unbuilt half, recorded above under element_result_deferred
    now: stays unbuilt; the first caller refuses tree-changing rewrites as its own policy, so the use it was designed for has no user
    kept_as_designed: the design is not deleted, because a project that has verified its own CSS may still want it
open_questions:
  - whether an application config file may enable a stock hook, given the transform itself needs Go
```
