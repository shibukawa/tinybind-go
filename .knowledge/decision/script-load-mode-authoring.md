---
id: decision:script-load-mode-authoring
type: decision
title: Script Load Mode Authoring
---
Author a script load mode as a bare `module` or `global` attribute, and lower it to valid HTML, because `type="global"` would never execute.

```yaml
source:
  - decision:framework-script-delivery
  - user syntax request 2026-07-27
review_gate: proposed; requires user approval
html_constraint:
  rule: a script type attribute is either absent, a JavaScript MIME type, or module
  unknown_type: any other value makes the element a data block; the browser neither fetches src nor executes content
  consequence: 'type="global"' is not a spelling of the global load mode, it is a silently dead tag
  reference: https://html.spec.whatwg.org/multipage/scripting.html#the-script-element
accepted_authoring:
  bare_module: '<script module src="/a.mjs"></script>'
  bare_global: '<script global src="/b.js"></script>'
  standard_module: '<script type="module" src="/a.mjs"></script>'
  reason: the template is parsed and lowered, not emitted verbatim, so an authoring attribute is a legitimate source-only marker
  symmetry: the bare form spells both modes the same way, which the type attribute cannot do
rejected_authoring:
  typed_global: '<script type="global" src="/b.js"></script>'
  diagnostic: generation error naming the element and suggesting the bare global attribute
  reason: accepting it would teach a spelling that breaks the moment it is copied into a real HTML file
  alternative_considered: lowering it silently to the correct tag, rejected because the template would look like HTML it is not
emission:
  module: 'type="module" on the emitted tag; modules are deferred by definition'
  global: 'defer and no type attribute, the ordinary classic script'
  marker_removed: the bare module or global attribute never reaches output
  ordering: both emitted forms execute in document order, so decision:framework-script-delivery head ordering holds for either
requiredness:
  rule: an authored script element states exactly one mode
  both: stating module and global is a generation error
  neither: a bare script element with no mode is a generation error, matching the registration rule that no load mode is defaulted
  reason: the failure mode is silent in the browser, so it must be loud at generation
applies_to:
  component_script: a script block or reference in a component file, extracted by requirement:static-asset-extraction
  registration: requirement:framework-script-contribution Load carries the same two values from generator options
  passthrough: an external URL keeps its mode marker and is emitted with the corresponding form
  bootstrap: requirement:html-runtime-bootstrap emits its own module tag and is unaffected
constraints:
  - async stays forbidden on a script whose contribution declares a Namespace, per decision:framework-script-delivery
  - a global script installing a name is the author's contract; the generator does not analyze the JavaScript
  - rule:template-context-safety script-content rules are unchanged
acceptance:
  - a bare module attribute emits type module and no leftover attribute
  - a bare global attribute emits a deferred classic tag with no type attribute
  - type module is accepted as authored and passes through unchanged
  - type global fails generation with a message naming the bare form
  - a script element with no mode, or with both, fails generation
open_questions:
  - whether an inline script block may state global, given requirement:static-asset-extraction extracts it to a file either way
  - whether a JavaScript MIME type spelled explicitly is accepted as a global synonym or rejected as redundant
```
