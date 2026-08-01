---
id: requirement:static-asset-extraction
type: requirement
title: Static Asset Extraction
---
Extract component style and script blocks into files under a configured public folder and reference them from the merged head.

```yaml
source:
  - decision:component-style-delivery
  - user asset decision 2026-07-25
extracted:
  style: a component style block, already rewritten by requirement:scoped-component-style
  script: a component script block carrying inline content
  passthrough: a script or link element that already names an external URL contributes its tag unchanged and produces no file
options:
  owner: data:generator-options
  PublicDir: filesystem directory receiving generated asset files
  PublicURLBase: URL path prefix under which those files are served
  defaults:
    PublicDir: public/generated
    PublicURLBase: /public/generated
    reason: extraction always happens, so a zero-configuration project still gets working asset URLs
  pairing: setting either option explicitly requires setting the other
  independence: neither option is derived from the other; the generator never strips, adds, or infers a path segment
naming:
  base: generation unit name plus asset kind
  hash: content hash in the file name, so the URL is immutably cacheable
resolution:
  file_path: PublicDir joined with the generated file name
  reference_url: PublicURLBase joined with the generated file name
  base_forms: PublicURLBase is either an absolute URL path or a full URL, and is used verbatim either way
  cdn: a full URL base emits that host in the reference, with no other behavior change
  no_magic: the two joins are the whole rule; nothing about public or the project layout is implied
emission:
  artifact: data:generation-artifact with a public asset destination
  writer: the generator writes the file, exactly as it writes Go artifacts
  determinism: identical input produces identical file names and bytes
head_reference:
  produced: a link or script tag stored on the decision:generated-render-plan value as a pending head contribution
  href: the resolution reference_url, written into the link href or script src attribute
  merged: requirement:head-merging places it in the root head
  static: the tag needs no per-request collection because the composition does not change it
  attributes: author attributes such as defer, async, and type are preserved on the emitted tag
script_rules:
  scoping: none; unlike CSS there is no name to rewrite, so a component script shares the global scope
  ordering: deterministic emission order; scripts are not reordered relative to each other
  dedup: one contribution per component regardless of instance count
  bootstrap: requirement:html-runtime-bootstrap remains a distinct runtime script and is never merged into a component bundle
  csp: external files let a policy forbid inline script
constraints:
  - extraction and hashing happen at generation time; nothing is assembled per request
  - a component script is authored content, so requirement:explicit-output-control trusted values stay unnecessary
  - a written asset never escapes PublicDir
  - removing a component removes its asset content on the next generation
acceptance:
  - a component shipping a script and a style produces two files and two head tags
  - an unchanged project regenerates identical file names, so caches stay valid
  - a project configuring neither option writes public/generated files and emits /public/generated URLs
  - a full URL base emits absolute references without changing where files are written
  - configuring only a URL base or only a directory fails with an actionable message
  - a component referencing an external CDN script emits no file
library_case:
  covered_here: a component declared in a template file of the generation unit being compiled
  not_covered: a component a library registers or a template file in another package, which owns no route, no scaffold, and no shell, and therefore cannot reference its own file
  extension: requirement:component-asset-requirements, which adds the declaration, the embedded table, the statically known required set, and the caller-supplied URL function
  reported: 2026-07-31 against v0.2.8, where component script had no counterpart to the style path at all; that half closed the same day
open_questions:
  - whether script blocks bundle per generation unit like styles or emit one file per component
  - default script type and whether module is assumed
  - asset cleanup of files left by removed components
  - single-binary mode that embeds assets instead of writing them, for requirement:tinygo-wasm targets; requirement:component-asset-requirements now carries this one
```
