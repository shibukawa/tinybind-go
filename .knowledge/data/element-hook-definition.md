---
id: data:element-hook-definition
type: data
title: Element Hook Definition
---
Generation-time record describing one reference hook: which element and attribute it matches, which values it accepts, and the transform that rewrites them.

```yaml
source:
  - requirement:element-reference-hook
  - concept:build-time-asset-transforms
status: proposed
fields:
  Name: hook identity, used in diagnostics and in the requirement:derived-asset-generation report
  Element: exact lowercase element name; a hyphenated name is out of scope in the first shape
  Attribute: exact attribute name
  Match: prefix, glob, or predicate over the static attribute value; exactly one form per entry
  Transform: Go func called at generation time, returning a value result, an element result, or a skip with a reason
  Mounts: URL prefix to directory pairs this hook resolves through, defaulting to the run's table
  Outputs: whether the transform may write derived files, so a read-only hook is declarable
  Report: whether each rewrite enters the build report individually or only in aggregate
transform_contract:
  input: the matched value, the resolved filesystem path when a mount covers it, a reader for the mounted tree, and the source position for diagnostics
  output: one of value, element, or skip, per requirement:element-reference-hook
  artifacts: zero or more produced files, each carrying bytes, a media type, and a target path and URL; a source map is one such file and no rewrite names it
  read_set: every file the transform read, reported back so requirement:derived-asset-generation can hash it; a converter following imports reads more than the file the reference named
  errors: a returned diagnostic, never a panic and never a write to the response, which does not exist yet
  purity: a pure function of its read set plus its own settings; the generator memoizes on that assumption
no_capabilities:
  rule: a hook declares nothing about the request
  contrast: data:builtin-element-definition carries Vary, per_request, needs_context, and needs_bootstrap, because its provider runs while rendering
  consequence: a hook result is static, so it enters a requirement:component-output-cache body and a requirement:layout-reuse-boundaries frame without restriction
identity_difference:
  builtin_provider: a rule:go-types-symbol-identity SymbolPattern resolved in the target module and called by generated code
  hook_transform: a func value in the generator process, absent from generated output
  effect: a hook needs no import in generated source and no rule:usage-directed-generation import decision
validation:
  registration_time:
    - duplicate Name
    - two entries sharing an Element and Attribute pair
    - malformed element name, attribute name, or match pattern
    - a transform declaring outputs with no configured output root
  analysis_time:
    - two hooks matching one attribute occurrence
    - a matched value under no mount
    - a transform diagnostic, reported at the attribute position
example_image:
  Name: image-format
  Element: img
  Attribute: src
  Match: prefix '/public'
  Transform: caller-supplied; the decision:transform-seam-ownership case_image
  Outputs: true
  note: a value result, so the element tree is unchanged; the caller's naming appends, making 'a.png.webp'
example_image_source:
  Name: image-format-source
  Element: source
  Attribute: srcset
  Match: prefix '/public'
  Outputs: true
  note: the aggressive-format half, firing only where the author wrote a picture; two entries on two elements express it and neither needs an ancestor test
example_typescript:
  Name: script-compile
  Element: script
  Attribute: src
  Match: glob '/public/*.ts'
  Transform: caller-supplied; the decision:transform-seam-ownership case_typescript
  Outputs: true
  note: the caller's naming replaces rather than appends, making 'app.js', and its read set covers imported modules the reference never named
example_fingerprint:
  Name: asset-fingerprint
  Element: link
  Attribute: href
  Match: prefix '/public'
  Outputs: false
  note: a read-only hook appending a content digest query to an authored stylesheet, writing nothing and needing no output root
related:
  - data:generator-options
  - requirement:derived-asset-generation
  - requirement:builtin-element-registration
```
