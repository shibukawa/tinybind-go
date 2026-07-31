---
id: requirement:builtin-element-registration
type: requirement
title: Builtin Element Registration
---
The generate command receives one whitelist of hyphenated element names, and every template in the generation unit is evaluated against it.

```yaml
priority: should
source:
  - concept:framework-template-extensions
  - user design discussion 2026-07-27
  - user whitelist decision 2026-07-27
review_gate: proposed
surface:
  timing: passed to the generate command; nothing is registered at runtime
  model: data:builtin-element-definition for a builtin entry, a name or pattern for a passthrough entry
  owner: data:generator-options, alongside the existing Calls and template pattern options
  registration: an api:generator-call-registration style registry, so a framework command declares markup extensions the same way it declares call patterns
  snapshot: registration returns an immutable options value safe for concurrent package analysis
  no_process_global: no package init and no runtime registry, matching requirement:framework-wrapper-discovery
contributors:
  framework: builtin entries, carrying markup, provider symbol, and capabilities
  application: passthrough entries for the Web Components it uses, per decision:builtin-element-syntax
  merge: both sets combine into one whitelist before analysis; a name declared twice with different kinds fails
application_form:
  go_options: an application building its own generator command adds entries as values
  cli: passthrough entries carry no Go symbol, so a flag or config-file list is sufficient for them
  builtin_via_cli: a builtin entry naming a provider symbol needs Go-level construction, so it is not expressible as a bare flag
resolution:
  scope: every template file in the generation unit; ambient by design, unlike requirement:template-file-scope external declarations
  lookup: the element name, with passthrough patterns matched after exact names
  disjoint: a whitelist entry never shadows a real HTML element, because standard HTML element names carry no hyphen
  ordering: resolution happens during HTML parsing, before requirement:builtin-element-lowering
validation:
  registration_time:
    - duplicate element name, or one name declared as both kinds
    - malformed element name or passthrough pattern
    - markup shape declaring a hole with no matching provider field or parameter
    - provider named for an opaque shape, or absent for a markup shape carrying provider holes
    - script contribution name with no matching requirement:framework-script-contribution registration
  analysis_time:
    - undeclared hyphenated element in a template file
    - provider symbol unresolvable in the target module
    - provider signature incompatible with requirement:render-value-provider
    - markup template that does not parse as HTML in the declared insertion context
zero_registration:
  behavior: no builtin element is available and every hyphenated element is undeclared
  compatibility: a project using no hyphenated element is unaffected and imports no framework package
  note: this is the one behavior change for existing templates, so the diagnostic must name every element to declare
acceptance:
  - a framework whitelists csrf-token once and every page in the unit may write it
  - an application file needs no external declaration and no import statement for a builtin element
  - a misspelled builtin element fails generation, naming the file, line, and column
  - an application-declared Web Component is emitted verbatim and produces no plan step
  - a component library declared by pattern needs no per-element entry
  - two contributors declaring the same element name fail registration rather than one silently winning
related:
  - requirement:custom-framework-generation-profile
  - requirement:configurable-generator-discovery
  - requirement:extensible-generator-command
  - rule:go-types-symbol-identity
open_questions:
  - whether builtin element registration joins the existing CallRegistry or gets its own registry type
  - whether the passthrough list is spelled in generator options only, or also in a project config file the generate command reads
  - whether the CLI reports the resolved whitelist for editor tooling and documentation
```
