---
id: concept:build-time-asset-transforms
type: concept
title: Build Time Asset Transforms
---
Let a registered hook match an element in a template at generation time, hand the file its attribute references to a caller-supplied transform, and rewrite the reference to what the transform produced.

```yaml
evidence:
  source: user design discussion
  received: 2026-08-01
review_gate: proposed requirements require user approval
what_this_module_ships:
  seam: matching, rewriting, diagnostics, artifact bookkeeping, and incremental correctness
  not_shipped: any transform, any format, any naming rule, and any switch
  owner_of_those: the framework building the generate command, per decision:transform-seam-ownership
driving_cases:
  image: convert a referenced image to a modern format at build time, the case that produced the design
  typescript: '<script src="/public/app.ts"> becomes /public/app.js, the case that proved naming and the read set belong to the caller'
  shared_shape: an authored file, referenced by URL from markup, converted by the build, with the reference following it
motivation:
  observation: the template already names the file, and the build already writes files, so the conversion needs neither a request nor a runtime
  today: a referenced source file is outside every seam; requirement:static-asset-extraction takes content out of a template, and nothing takes a file the template points at
new_axis:
  what: a hook intercepts a standard HTML element, where decision:builtin-element-syntax deliberately closed only the hyphenated space
  why_it_is_new: every existing generation-time extension is opt-in at the element name; this one changes markup the author wrote in plain HTML
  risk: the template says '/public/app.ts' and the page says '/public/app.js', so the source stops being literally what ships
  containment: registration is explicit per project, the produced file must exist or generation fails, and a hook rewrites only a value it can resolve at generation time
structure_invariance:
  rule: a shipped policy rewrites attribute values and leaves the element tree the author wrote
  raised_by: user, 2026-08-01
  reason: CSS and JavaScript are written against the authored structure, and the generator sees neither a global stylesheet nor a script, so a structural rewrite breaks pages with no diagnostic
  effect: it is why requirement:element-reference-hook leads with the value result
parts:
  mechanism: requirement:element-reference-hook, the match-and-rewrite seam
  bookkeeping: requirement:derived-asset-generation, the produced file and the inputs it depends on
  ownership: decision:transform-seam-ownership, the division and the evidence behind it
  registration: data:element-hook-definition
division:
  module: matching, rewriting, artifact emission, determinism, incremental skip, and diagnostics
  caller: the bytes-in-bytes-out transform, the output name, and where the file is served
  reason: the same rule requirement:component-asset-requirements states, that the module decides identity and the caller decides delivery
dependency_ownership:
  rule: no codec and no compiler enters this module's go.mod, which today holds golang.org/x/tools and two small packages
  form: the transform is an ordinary Go func value in data:generator-options, supplied by whoever builds the generator command
  consequence: a project wanting a converter pays for it in its own generator command, and a project wanting none pays nothing
distinct_from:
  builtin_element:
    theirs: a new kebab-case element the framework defines and the author writes on purpose
    ours: an element the author already wrote, whose reference the build redirects
    shared: both resolve at generation time, both fold into static bytes, and neither costs anything at render time
  static_asset_extraction:
    theirs: requirement:static-asset-extraction takes content out of a template and gives it a URL
    ours: takes a URL the template already carries and points it at a file produced from an authored one
    overlap: that requirement passes an external script or link URL through unchanged, and a hook is what would act on it
    conflation_to_avoid: PublicDir and PublicURLBase name where generated assets go; an authored source tree is a different pair, per requirement:derived-asset-generation
  component_assets:
    theirs: requirement:component-asset-requirements gives a library a way to ship a file it owns
    ours: transforms a file the application owns and already references
scope:
  - static attribute values only; an expression-valued attribute is never rewritten
  - generation time only; no runtime lookup, no manifest read, no negotiation
  - a project registering no hook regenerates byte-identical Go, per decision:framework-integration-seams
non_goals:
  - bundling, minification, or a dependency graph this module understands
  - runtime content negotiation on Accept
  - rewriting references inside CSS, JavaScript, or Markdown
  - a plugin surface loaded after generation
later_items:
  intrinsic_size: a transform holding decoded image bytes knows the pixel size, so it could add width and height to an img that omits them; same hook, separate decision
  css_url: requirement:scoped-component-style already rewrites a style block, so a url() pass has a natural home and no design; a standalone stylesheet in a project's own asset tree is outside every requirement here and stays the caller's, so the two never contend
taken_2026_08_04:
  head_contribution: the preload item, promoted to requirement:hook-head-contribution once a downstream project found the case that makes it a correctness problem rather than a latency one
  parallel_conversion: requirement:parallel-conversion, which converts before the compile rather than after it, so the cold-cache serialization goes without giving up the outcome the rewrite depends on
  neither_widens_the_non_goals: a head entry references a file the transform already produced, and concurrency changes wall clock; no codec, bundler, or format table comes in with either
```
