---
id: requirement:derived-asset-generation
type: requirement
title: Derived Asset Generation
---
Resolve a matched reference to an authored file, produce a derived file from it at build time, and guarantee the rewritten reference names a file the build actually wrote.

```yaml
priority: should
source:
  - concept:build-time-asset-transforms
  - requirement:element-reference-hook
  - user design discussion 2026-08-01
review_gate: partially implemented 2026-08-02
as_built:
  shipped:
    inline_with_a_cache: a transform converts and returns the bytes, so the rewrite can depend on the outcome; Options.ConversionCacheDir stores that outcome under the hook's declared key, per the requirement:element-reference-hook conversion_timing
    produced_files: a conversion returns named files; the generator writes them under Options.DerivedAssetDir and declares every one in GenerateResult.Paths, so `--check` and any future cleanup can see them
    artifact_path: the same files also reach the artifact API, as ArtifactDerivedAsset with a public asset destination, for a caller taking files rather than writes
    containment: an absolute name, a backslash, or one reaching above the root fails generation
    collision: two different files claiming one name fails generation
    naming_is_the_callers: no naming rule shipped, which is what lets the append and replace cases both work
    read_set: the sources every cache key named, plus whatever each transform reported reading beyond them, deduplicated and sorted across the run and surfaced on GenerateResult
    input_hash_gap_closed:
      how: the run records each read file and its digest in tinybind_deps_gen.json beside the generated Go, and the next run verifies that record before trusting its own skip
      why_not_the_fingerprint: the fingerprint decides whether the run happens and the read set is known only once it has, so a recorded dependency is the only ordering that works
      no_hook_no_file: a project registering none writes no record and behaves exactly as before
      unverifiable_is_regenerate: a missing, malformed, or older record regenerates rather than being trusted
    missing_source: a transform returning an error fails generation with the template file, line, and column
  not_shipped:
    mount_table: no URL-to-directory option exists; a transform resolves its own paths today, which works and leaves the resolution rule unstated
    output_placement: the three placements are still a design question; DerivedAssetDir is one directory and the transform chooses the URL, so a mirror overlay is expressible and unproven
    parallel_conversion: a cold cache converts serially, because the compile that needs each value is sequential; an incremental build converts nothing, which is what made this affordable to defer
    transform_identity: an in-process converter is covered by the executable hash, and an external binary's path and version are not hashed
    report: GenerateResult.Rewrites carries the data and nothing formats it
    cleanup: unchanged; a produced file whose source disappeared is still stale
review_gate_note: the shipped half needs user approval as built; the not_shipped half is still proposed
source_resolution:
  need: a hook sees '/public/img/a.png', a URL, and the transform needs bytes
  mount_table: an explicit list of URL prefix to filesystem directory pairs in data:generator-options
  no_magic: the pair is used verbatim in both directions, following the requirement:static-asset-extraction rule that nothing about a project layout is inferred
  distinct_pair: this is the authored asset tree; PublicDir and PublicURLBase name where generated assets are written and served, and conflating the two is the mistake this states to prevent
  unmounted: a matched value under no mount is a generation error, because a hook that cannot read cannot rewrite honestly
  escape: a resolved path leaving its mounted directory is a generation error, matching the requirement:static-asset-extraction containment rule
safety_property:
  claim: the rewritten reference never dangles
  how: the rewrite and the file write are one step; a transform that cannot produce bytes returns skip and the original reference stands
  contrast: this is what the naming form buys over serving the original bytes under a different content type, per decision:transform-seam-ownership
output_placement:
  options:
    mirror_root:
      form: a derived tree the application serves at the same URL prefix as the source, overlaid ahead of it
      url: identical to the source URL plus the added suffix, which is the shape the design started from
      cost: the application serves two directories at one prefix
      recommended: true
    in_place:
      form: written beside the source in the authored tree
      url: same as mirror_root, with no serving change
      cost: generated files in the author's tree, needing a gitignore rule
    generated_root:
      form: written under PublicDir like every other generated asset
      url: no longer a suffix of the source URL, so the rewrite is a full replacement
      cost: none in wiring, but the derived name loses its visible relation to the source
  owner: the transform, through the URL and path policy it declares; requirement:element-reference-hook has no opinion
naming:
  owner: the transform, which returns the output name; this module has no naming rule
  proved_by: the two decision:transform-seam-ownership cases disagree, since an image appends to make 'a.png.webp' and a TypeScript entry replaces to make 'app.js'
  append_form:
    what: 'a.png' becomes 'a.png.webp'
    merits: never collides with an author's own 'a.webp', keeps provenance readable in a listing and in view-source, and needs no table of replacements
  replace_form:
    what: 'app.ts' becomes 'app.js'
    merits: the output is the name the ecosystem expects, and a served '.ts' would be the surprising artifact
  immutability: an output name derived from the source name is exactly as stable as it, so it inherits whatever cache policy the application already serves that prefix with
  hashed_variant: a content-hashed name is a transform's choice for an immutable cache header, and is nobody's default because it hides the source name
  collision: two sources producing one output name is a generation error, and this module detects it because it owns the artifact list
emission:
  artifact: data:generation-artifact, like every other written output
  writer: the generator writes it, so a dry run writes nothing
  determinism: identical inputs produce identical bytes and identical names
read_set:
  what: every authored file a transform read, reported back with its result
  wider_than_the_reference: a converter following imports reads files no template names, so the digest of the referenced file is not the input
  reported_not_derived: this module understands no dependency graph and asks for none; the transform knows what it opened and says so
  effect: the read set is both the memoization key and the hashed input set
incremental:
  why_it_matters_more_here: extracting a style block is microseconds and converting a file is seconds, so the skip path is the difference between a usable build and an unusable one
  key: the hook name, the claimed value, the digests of the sources the hook named, and a caller-written params string carrying format, quality, and encoder version
  cache: the whole outcome is stored under that key and reused across runs, independent of the rule:generation-input-hash stamp that decides whether the run happens at all
  decision_is_cached: a skip is stored like a conversion, so an encode that lost to its source runs once and never again
  reuse_across_units: two generation units referencing one source convert it once
input_hash_gap:
  found: rule:generation-input-hash hashes Go sources, template files, go.mod, options, and the generator executable, and hashes no authored asset
  consequence: today an edited image or an edited module changes no hashed input, so a run that would rewrite its reference skips entirely
  fix: the read set digests join the hashed input set, recorded per run
  scanning_refused: hashing every file under a mount would make an unrelated upload force regeneration
  worst_case: a transform under-reporting its read set produces a stale output no diagnostic catches, which is the one correctness property this module cannot verify for the caller
transform_identity:
  in_process: a converter compiled into the generator command is already covered, because rule:generation-input-hash hashes the executable content
  external_binary: an invoked tool is not covered, so its resolved path and reported version are hashed explicitly and a missing binary is a generation error naming it
  reason: a silent tool upgrade changing bytes is a `--check` failure with no stated cause
report:
  content: per produced file, the source, the output name, both sizes, and whether it was converted or reused from cache
  skips: every skip with its reason, so a page that transformed nothing says why
  why: an author cannot see a build-time rewrite in the template, so the build is the only place it is visible
  not_an_interpretation: the run reports what it did; whether a size is acceptable is the caller's judgment, per decision:transform-seam-ownership
cleanup:
  same_question: requirement:static-asset-extraction already carries asset cleanup for files left by removed components, and a produced tree has the same shape
  stated: a produced file whose source disappeared is stale, and no rule removes it yet
constraints:
  - nothing reads a produced file at runtime, unchanged from requirement:component-asset-requirements
  - no codec and no compiler dependency enters this module, per concept:build-time-asset-transforms
  - a project registering no hook writes no produced file and hashes no asset
  - a produced file never escapes its configured output root
acceptance:
  - a matched reference to a missing file fails generation naming the template position and the resolved path
  - an unchanged project regenerates identical bytes and performs no conversion
  - editing a referenced source regenerates it and only it
  - editing a file the transform read but no template names regenerates it
  - editing an unrelated file under the same mount regenerates nothing
  - an external tool upgrade regenerates every produced file rather than producing a silent mismatch
  - two templates referencing one source produce one output
  - a transform producing a file no rewrite names, such as a source map, still has it declared and verified
  - the build report names every rewrite and every skip
related:
  - data:element-hook-definition
  - decision:transform-seam-ownership
  - data:generation-artifact
  - requirement:incremental-generation
open_questions:
  - whether the derived cache lives in the module cache directory, the build directory, or a configured path
  - whether a mount may be declared read-through to a CDN origin rather than a local directory
  - whether the mirror root should be the default given it needs application wiring, or in-place should be, given it needs none
```
