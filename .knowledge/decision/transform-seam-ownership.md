---
id: decision:transform-seam-ownership
type: decision
title: Transform Seam Ownership
---
Ship the rewrite seam and no transform policy, and keep the image and TypeScript cases that motivated it as the worked evidence that fixes the seam's shape.

```yaml
source:
  - user design discussion 2026-08-01
  - concept:build-time-asset-transforms
review_gate: proposed
scope_correction:
  when: 2026-08-01, after the format and fallback rounds below
  stated: the framework converts the file and owns the switch, the format choice, and the measurement; this module is asked for the tag rewriting that makes it possible
  effect: an earlier draft read as this catalog's image policy, which was the wrong owner; the format findings survive as evidence and stop being rules here
division:
  module:
    - matching an element and attribute in a parsed template and rewriting the value
    - a diagnostic carrying the template file, line, and column
    - calling a transform once per distinct value and treating the result as pure
    - declaring every written file as a data:generation-artifact of the run
    - resolving a reference URL to an authored file and hashing every file the transform read
  caller:
    - what a source becomes, under what settings, and how it is classified
    - the output name and where it is served
    - the switch, per environment or per build
    - measuring sizes and deciding what a measurement means
    - the converter itself
test: a thing belongs here when a transform running as a callback cannot do it without breaking a build contract
why_not_all_in_the_callback:
  observation: the transform is the framework's own Go func, so it could read, convert, and write files itself and ask this module for nothing but the rewritten string
  what_breaks:
    stamp: a file written behind the generator is absent from the data:generation-stamp outputs, so rule:generation-input-hash cannot verify it and a later run skips while the file is missing
    check: a `--check` run compares the bytes a run declares; an undeclared write is invisible to it
    input_hash: an authored source the callback read is not a hashed input, so editing it changes nothing and the run skips, per the requirement:derived-asset-generation gap
    cleanup: a file no run declares is a file no run can remove
  conclusion: byte production belongs to the caller and bookkeeping belongs here, which is why requirement:derived-asset-generation exists at all
measurement:
  finding: the framework needs no reporting API from this module, because it owns the transform and can measure inside it
  caveat: requirement:element-reference-hook memoizes per distinct value, so a call count counts distinct sources and not elements
  provided: occurrence counts and the written artifact list, which the run knows and the callback cannot see
switch:
  form: registering the hook or not, since data:generator-options is a value per invocation
  regeneration: the options value is hashed by rule:generation-input-hash, so flipping the switch regenerates rather than leaving stale output
  dev_mode: a registered transform returning skip does no work and writes nothing, which is the cheap way to keep one options value across environments
case_image:
  what: convert a referenced image to a modern format at build time
  structure_invariance:
    found: wrapping an img in a picture breaks CSS combinators, flex and grid item identity, and structural JavaScript, and the generator sees neither global CSS nor scripts, so it cannot warn
    raised_by: user, 2026-08-01
    fixed_in_the_seam: requirement:element-reference-hook leads with the value result and marks a tree-changing element result as a project decision rather than a default
    kept_available: the element result still exists, because a caller that has verified its own CSS may want it
  fallback_is_not_expressible:
    found: format negotiation on a bare img is impossible, since srcset carries density and width descriptors and no type
    consequence: a caller wanting a format without universal support writes the picture itself, and the seam serves that by matching source srcset as an ordinary second entry
    seam_property: two entries on two elements need no ancestor test, so decision:builtin-element-syntax keeps its no-ancestor-constraint rule
  classification_is_the_callers:
    found: lossy and lossless are separate axes, so jpeg, webp lossless, and avif do not order on one; within each axis the ranking holds, and avif's lossless mode is often no better than webp lossless on small flat images
    consequence: the useful decision for a png source is whether to leave the lossless axis at all, which is a judgment about content no build rule can make
    seam_property: the transform receives the bytes and decides; this module classifies nothing
  skip_must_be_first_class:
    found: a derived file can be larger than its source, and an svg or an already-converted file needs no work
    seam_property: skip is a result kind with a reason, not an error and not a silent no-op
case_typescript:
  what: '<script src="/public/app.ts"> becomes /public/app.js, compiled at build time'
  raised_by: user, 2026-08-01
  value: it is the second case, and everything it needs that the image case did not is a property the seam would otherwise have gotten wrong
  naming_is_the_callers:
    found: the image case appends, so 'a.png' becomes 'a.png.webp'; this case replaces, so 'app.ts' becomes 'app.js'
    consequence: a naming rule in this module would have been wrong for one of the two cases; requirement:derived-asset-generation states it as a transform policy for exactly this reason
  read_set_is_wider_than_the_reference:
    found: a TypeScript entry imports other files, so the digest of the named file is not the input
    consequence: the transform reports every file it read, and those digests join the hashed inputs; the image case never needed this because an image reads only itself
    failure_avoided: editing an imported module changes no hashed input, so the run skips and ships stale JavaScript
  outputs_outnumber_references:
    found: a source map is written and referenced by no attribute in any template
    consequence: an artifact list is not a rewrite list; the run declares files the markup never names
  head_declaration_reach:
    found: requirement:static-asset-extraction passes a script or link element naming an external URL through unchanged and produces no file
    consequence: the hook must reach those tags too, or a project referencing its entry point from a component head declaration is the one place the seam does not apply
  sibling_case: a stylesheet link naming a preprocessor source is the same shape with a different extension
non_goals:
  - a codec, a compiler, a format table, or a quality setting in this module
  - dependency graph knowledge; the transform reports what it read and this module hashes it
  - bundling or minification as a module concern, unchanged from requirement:static-asset-extraction
  - a runtime negotiating on Accept, which concept:build-time-asset-transforms excludes
  - this module choosing URLs, serving files, or setting cache headers, unchanged from requirement:component-asset-requirements
acceptance:
  - a framework registers one hook and gets rewriting, diagnostics, artifact bookkeeping, and incremental skip without this module naming a format
  - two hooks with opposite naming rules both work, because this module has no naming rule
  - editing a file the transform read, and never named in a template, regenerates
  - this module's go.mod gains no codec and no compiler
  - a project registering no hook regenerates byte-identical output
related:
  - requirement:element-reference-hook
  - requirement:derived-asset-generation
  - data:element-hook-definition
  - decision:framework-integration-seams
```
