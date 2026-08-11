---
id: requirement:authored-language-transform
type: requirement
title: Authored Language Transform
---
Let a component script block declare the language it is written in, hand its extracted content to the caller-supplied transform the module already specifies, and write what comes back, so TypeScript compiles at generation without a compiler entering this module.

```yaml
priority: should
source:
  - user direction 2026-08-11, asking for TypeScript and bundling through esbuild
  - requirement:component-script-block
  - concept:build-time-asset-transforms
  - decision:transform-seam-ownership
review_gate: proposed
status: shipped 2026-08-11; the marker, the seam, and its diagnostics are in place, and no transform ships with them
as_built:
  seam: templates/htmlbind/content_hook.go holds ContentHook, ContentRequest, ContentResult, and ValidateContentHooks, the content-side counterpart of the ReferenceHook already there
  claimed_by: the hook's Lang against the block's lang attribute; the marker is read as literal text and an expression there is a diagnostic
  request_carries: the authored content, the component name, and the template's own directory, which is what a bundling transform resolves an import against
  result_carries: the produced content, an extension override, and the files the transform read
  read_set: joined into Result.ReadSet beside the reference transforms', so one path regenerates for either kind
  registration: GenerateOptions.ContentHooks and generator Options.ContentHooks, validated once per generate command
  marker_removed: the lang attribute never reaches the emitted tag, which is checked
  tests: templates/htmlbind/content_hook_test.go, over the compile, the request contents, the extension, the read set, the unregistered marker, a failing transform, and registration validation
  not_done:
    esbuild_transform: none ships, which is the point; a project builds one into its own generate command
    style_blocks: a lang marker on a style block, which is the same seam one file over and is left open below
diverged_from_the_draft:
  hook_shape: an earlier draft left open whether this is a second field beside the element hook or a second call shape on the same one; it is a separate type, because the two claim different things and share no field but a name
already_decided_do_not_reopen:
  dependency: concept:build-time-asset-transforms dependency_ownership states that no codec and no compiler enters this module's go.mod, which today holds golang.org/x/tools and a few small packages
  converter_owner: decision:transform-seam-ownership division puts the converter itself, the switch, the settings, and the output naming with the caller
  typescript_is_the_worked_case: that concept already names TypeScript as a driving case and data:element-hook-definition already carries a TypeScript transform entry
  therefore: esbuild is a Go library and imports cleanly, and it still belongs in the generator command a project builds rather than in this module; the ask is served by reaching the existing seam, not by adding a dependency
the_gap_this_closed:
  what_the_reference_seam_reaches: requirement:element-reference-hook matches an element attribute naming an authored file, the '<script src="/public/app.ts">' case, and rewrites the reference
  what_it_did_not_reach: a script block has content and no reference, so extraction produced bytes that no transform was offered
  closed_by: a second hook type claiming a lang marker on the block, per as_built
  why_not_the_same_type: a reference hook is keyed by element and attribute and returns a rewritten value; a content hook is keyed by language and returns bytes, so they share a name and no field
  what_carried_over_unchanged: the purity rule, the once-per-distinct-value call, the caller-owned output name, and the read set that keeps the incremental skip honest
language_marker:
  spelling: a lang attribute on the block, 'lang="ts"'
  default: absent means JavaScript, passed through untransformed, which is every block today
  rejected: 'type="ts"'
  why_rejected: decision:script-load-mode-authoring already refused 'type="global"' because an unknown script type makes the element a data block that neither fetches nor executes, so the spelling teaches a habit that breaks the moment it is copied into real HTML; ts is the same trap with a different word
  why_lang_is_safe: lang is not a script attribute in HTML at all, so it cannot be mistaken for a spelling that works, and it is what the single-file component ecosystems already use
  convention: policy:frontend-convention-alignment prefers React, which offers no counterpart because it has no template-block surface; the divergence clause decision:lifecycle-from-declaration-block already invokes covers this too
  lowered_away: the marker never reaches the emitted tag, matching decision:script-load-mode-authoring marker_removed
  open_set: the module validates that a marker was registered and never that it names a language it knows; an unregistered marker is a generation error naming the block and the registered set
transform_contract:
  input: the block's extracted content, the language marker, and the directory of the template that declared it
  why_the_directory: a bundling transform resolves 'import "./util.ts"' against something, and only the template's own location is meaningful; esbuild's stdin form takes exactly this as its resolve directory
  already_noted: requirement:derived-asset-generation records that no URL-to-directory option exists and that a transform resolves its own paths today, which is the same gap seen from the referenced-file side
  output: the produced bytes and the set of files the transform read
  read_set_is_required: decision:transform-seam-ownership puts hashing every file the transform read with the module, because incremental correctness is the module's; a transform that under-reports leaves a stale bundle after an edit to an imported file
  read_set_is_available: esbuild's metafile reports its inputs, so the contract is satisfiable by the transform the user has in mind rather than being a demand nothing can meet
  naming: the produced extension comes from the transform, per decision:transform-seam-ownership caller owns the output name; a ts block becoming js is the caller's rule and not a mapping written here
  purity: unchanged from that decision, one call per distinct content and the result treated as pure
compile_but_do_not_bundle:
  decided: decision:share-by-module-url, 2026-08-11
  shape: the transform compiles one block's content and leaves its import specifiers alone; esbuild's transform form does exactly this
  fits_the_seam_exactly: the call stays once per distinct content and pure, so decision:transform-seam-ownership needs no batch form and no exception
  no_duplication: a module shared by two blocks is one URL in the document's module map and is fetched and evaluated once
  why_this_is_not_a_compromise: an earlier draft here recommended per-block bundling and accepted duplicated shared code as a size cost; bundling is the step that creates the duplication, so removing it removes the problem rather than trading against it
  specifier_rules: decision:share-by-module-url, which rejects a relative specifier because an extracted block resolves it against its generated URL rather than against the template's directory
  what_still_needs_more: a bare specifier, which resolves only through an import map the caller owns
what_this_does_not_add:
  no_dependency: nothing enters go.mod, and a project wanting no transform pays nothing
  no_language_knowledge: the module does not know what ts means, does not parse JavaScript, and does not validate the output
  no_minification_or_target_policy: settings, environments, and switches are the caller's, per decision:transform-seam-ownership
  no_source_map_rule: whether a transform emits one and where it is served is the caller's, and the module writes what it is handed
  no_runtime_change: this is generation-time only; nothing about rendering, the wire, or the client half moves
compatibility:
  bytes: a project declaring no lang marker and registering no transform regenerates byte-identically
  tinygo: generation-time only, so requirement:tinygo-wasm targets are untouched
  determinism: a pure transform over hashed content keeps the requirement:static-asset-extraction guarantee that identical input produces identical names and bytes
acceptance:
  - a block marked lang ts is handed to the registered transform and its output is written under the extension the transform chose
  - a block with no marker is written unchanged and calls no transform
  - a marker with no registered transform fails generation naming the block and the registered set
  - editing a file the transform reported reading regenerates the block's output, and editing an unrelated file does not
  - a transform is called once for two components whose blocks have identical bytes
  - a project registering no transform regenerates byte-identically
reopens:
  export_convention: decision:lifecycle-from-declaration-block rejected reading the export set because the module does not parse JavaScript; a transform that already parses could report it, which would let a block claiming a lifecycle be checked rather than trusted
  caveat: that would move a check behind a caller-supplied func, so a project with no transform would lose it; noted as a possibility rather than a plan
open_questions:
  - whether a lang marker is also accepted on a style block, which would reach Sass through the same seam and needs only the same call one file over
  - whether a component name on the request invites a transform to depend on it, which would break the purity the seam requires; it is passed for diagnostics and nothing enforces the distinction
  - whether a block importing another component's block is expressible at all, or shared code must be an authored file outside the templates
  - whether a caller compiling several modules needs its own memo over the transform, as the generator already keeps for reference hooks, since once-per-distinct-content holds within one module only
```
