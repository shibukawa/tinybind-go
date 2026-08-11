---
id: requirement:component-script-block
type: requirement
title: Component Script Block
---
Accept a script block at the top of a component, beside its head block, read its content as authored JavaScript rather than as markup, and extract it to a file the way a head script is already extracted.

```yaml
priority: should
source:
  - decision:lifecycle-from-declaration-block
  - user direction 2026-08-11
  - requirement:static-asset-extraction
review_gate: proposed
status: shipped 2026-08-11; parser, compiler, extraction, the declaration marker, and five diagnostics, with no runtime code at all
as_built:
  parser: templates/htmlbind/html.go reads a marked block verbatim through the same readRawUntilClose path a head contribution already used, gated by isComponentScriptBlock
  compiler: collectScriptBlock takes the block out of the body before any other pass sees it, so nothing downstream treats it as markup; rejectNestedScriptBlocks reports a marked block anywhere else
  extraction: templates/htmlbind/assets.go emits it through the existing newAsset path and appends its reference tag after the head contributions
  owner: the template-time Asset gained Owner, and templates/htmlbind/emit.go writes Scope onto the generated htmlbind.Asset only when it is set
  tests: templates/htmlbind/scriptblock_test.go, over extraction, the owner, verbatim content, the module reference, import rules, the marker on an ordinary component call, and the five diagnostics
syntax:
  shape: |
    export component Counter(label: string): html {
    <head>
      <link rel="stylesheet" href="/shared.css" />
    </head>
    <script component>
      export function setup(el) { ... }
    </script>
    <div class="counter">{label}</div>
    }
  marker: a bare component attribute, required
  why_a_marker: decision:lifecycle-from-declaration-block position_alone_is_ambiguous; a top-level script is equally the shape of markup carrying a RawJavaScript or JsonForScript insertion, which is a shipped feature, so position cannot say which of the two it is
  position: a sibling of the head block, before the component's markup
  siblings: head, script, then markup; the single-file component layout with the module's own head block in the template slot
  optional: a component declaring none is unchanged in every respect
  one_only: two script blocks in one component is a generation error naming the component, the same check the head block already carries
  order_between_blocks: head and script may be written in either order and neither reorders the other's output; requiring a fixed order would be a rule with nothing behind it
parser:
  change: the raw-text gate that today marks parsing as inside a head contribution generalizes to inside a declaration block, so a script block's content is read to its closing tag as authored text
  why: a brace in JavaScript is otherwise read as template interpolation, which is what makes a body script unusable today
  covers: only the declaration block; markup inside the component keeps parsing exactly as now
  attributes: parsed as ordinary attributes and available to the rules below, so requirement:authored-language-transform can read a language marker off the tag
  reuse: the same treatment head already applies to its style and script children, applied one level out, rather than a second raw-text mechanism
extraction:
  same_pipeline: the block becomes a content-hashed file under PublicDir with a reference under PublicURLBase, exactly as a head script does
  identity: unchanged; two components whose blocks have identical bytes still emit one file
  reference_tag: emitted into the merged head like today's script reference, so nothing is written into the rendered body
  why_the_head_still_carries_the_tag: the tag's position decides when the file loads and nothing more; the lifetime is the block's position in the source, per decision:lifecycle-from-declaration-block, and the two are independent
  ownership: the asset records the declaring component, per requirement:scoped-script-declaration
load_mode:
  implied: module; a component script block is a module and states nothing
  reason: decision:script-load-mode-authoring requires an explicit mode on a head script because both are legitimate there, and only one is legitimate here, since a lifecycle method is an export and a classic script has none
  explicit_module_accepted: writing the module marker is accepted and redundant
  global_rejected: a global marker on a component script block is a generation error, naming the block and the reason
  why_it_is_an_error_and_not_a_downgrade: a classic script's only per-visit behavior is re-execution, which runs the body twice, throws NotSupportedError on a repeated customElements.define, and re-adds every listener; that is the bug concept:scope-lifecycle exists to remove
imports:
  allowed: the block is a module, so it may import; decision:share-by-module-url makes that the way two blocks share code rather than a bundler
  untouched: an import specifier reaches the emitted file exactly as authored
  relative_rejected: a relative specifier is a generation error naming the block and the URL it would have resolved to, because an extracted block is served from PublicURLBase rather than from the template's directory
markup_script_is_untouched:
  corrected: 2026-08-11, while implementing; an earlier draft of this requirement made a script in a component's markup a generation error and claimed nothing accepted it today
  what_was_wrong: testdata/templates/htmlbind/contexts compiles '<script>{RawJavaScript(javascript)}</script>' and '<script>window.payload = {JsonForScript(payload)};</script>', so a markup script with an insertion is a shipped, tested feature and rejecting it would have been a breaking change
  rule: an unmarked script element keeps its current meaning everywhere, including its existing raw-text diagnostic when a brace was meant literally
  rejected_instead: a marked block found anywhere but the component's top level, which is a declaration written where it cannot mean what it says
  reading: the feature is strictly additive; a project that writes no marker sees no change at all
compatibility:
  bytes: a project declaring no script block emits byte-identical output, per requirement:html-rendering-compatibility
  reinterpretation: none, because the marker is what selects the new reading and no existing template writes it; an unmarked script anywhere keeps its current meaning
  scope_emission: Scope is written onto a generated asset only when a block set it, so a project declaring none regenerates byte for byte there too
  tinygo: generation-time only, so requirement:tinygo-wasm targets are untouched
acceptance:
  - a component declaring a script block emits one content-hashed file and one head reference, matching what a head script emits today
  - a brace, a less-than, and a template literal inside the block reach the file verbatim rather than being read as interpolation or markup
  - the emitted reference is a module
  - the block is not rendered; nothing of its body reaches the component's output
  - two script blocks in one component fail generation naming the component
  - a global marker on the block fails generation naming the component
  - a marked block inside markup or inside a control block fails generation
  - a relative import specifier in the block fails generation naming the specifier
  - an unmarked script carrying a template insertion still compiles and is still emitted in place
  - a component declaring no block regenerates byte-identically
open_questions:
  - whether a style block should move beside the script block for symmetry, which decision:lifecycle-from-declaration-block also raises and which would change where authors write CSS
  - whether the block may be empty, which is harmless and pointless, and whether the diagnostic is worth the rule
  - whether a component may declare a script block while declaring no markup, which is a module with a component's name and no reason to be one
```
