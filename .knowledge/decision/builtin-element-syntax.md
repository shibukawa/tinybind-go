---
id: decision:builtin-element-syntax
type: decision
title: Builtin Element Syntax
---
Treat the hyphenated element space as a closed whitelist supplied to the generate command, so a builtin keeps its bare name and an undeclared hyphenated element is a generation error.

```yaml
source:
  - concept:framework-template-extensions
  - user syntax request 2026-07-27
  - user whitelist decision 2026-07-27
review_gate: proposed; requires user approval
problem:
  casing: rule:template-name-casing assigns kebab-case to real HTML elements, so a bare csrf-token sits in the custom-element space
  web_components: a hyphen is the HTML custom-element marker, so a project may legitimately write its own hyphenated element
  typo: an unrecognized hyphenated element emitted unchanged renders nothing and reports nothing
decision:
  form: bare name, as requested; '<csrf-token/>' with no prefix
  whitelist: the generate command receives the complete set of hyphenated element names it may see
  closed_space: a hyphenated element absent from the whitelist is a generation error, not silent passthrough
  effect: the typo case disappears without a prefix, because the generator knows the whole legal set
entry_kinds:
  builtin:
    meaning: rewritten by requirement:builtin-element-lowering into plan steps
    carries: a full data:builtin-element-definition
    source: the framework generator command
  passthrough:
    meaning: emitted verbatim as an ordinary custom element
    carries: a name or a name pattern only
    source: the application, listing the Web Components it uses
    reason: without this kind a closed space would ban Web Components outright
patterns:
  exact: a single element name
  glob: a library prefix pattern such as 'sl-*', so a component library is declared once
  scope: patterns apply to passthrough entries only; a builtin is always an exact name
  overlap: a builtin name matching a passthrough pattern resolves as the builtin; the reverse never happens
foreign_content:
  rule: hyphenated names inside SVG and MathML subtrees are outside the whitelist and stay verbatim
  reason: they are standard foreign-namespace element names, not custom elements
rejected:
  registered_prefix:
    shape: '<pw-csrf-token/>' with the framework owning a prefix
    reason: the whitelist already closes the space, so the prefix buys no diagnostic and costs verbosity
    note: a framework may still choose prefixed names as a convention; nothing forbids it
  open_passthrough:
    shape: unknown hyphenated names emitted unchanged
    reason: this is exactly the undiagnosable typo case the whitelist removes
  pascal_component:
    shape: '<CsrfToken/>' auto-imported into every file
    reason: does not match the requested tag look, and an ambient import contradicts requirement:template-file-scope
form:
  attributes: static names, expression values, following requirement:html-template-v1 attribute rules
  void: self-closing when the definition declares no children parameter
  children: a definition may declare a reserved children parameter, filled like requirement:html-slot-syntax nested content
  no_dynamic_name: a builtin element name is static, matching the requirement:html-template-v1 dynamic-name ban
diagnostics:
  - undeclared hyphenated element, naming the file position and the nearest whitelisted name
  - unknown attribute, or an attribute whose expression type does not match the declared parameter
  - a required parameter left unset
  - children supplied to a definition declaring none
  - a builtin element written in a position its declared insertion context forbids
migration:
  existing_project: a project already using Web Components adds passthrough entries once; the diagnostic names every element it must declare
no_ancestor_constraint:
  decision: a definition never constrains its allowed ancestor element
  reason: the enclosing form may live in a caller, a layout, or a slot fill in another file, so the ancestor is unknowable at the element position
  consequence: a hidden-input builtin written outside a form is the author's error and is not diagnosed
  still_checked: the rule:template-context-safety insertion context, which is local to the element position and always known
open_questions:
  - whether two frameworks in one build may contribute builtin entries to one whitelist
  - whether the nearest-name suggestion uses edit distance or plain listing
```
