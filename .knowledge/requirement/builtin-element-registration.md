---
id: requirement:builtin-element-registration
type: requirement
title: Builtin Element Registration
---
The generate command receives one whitelist of hyphenated element names, and every template in the generation unit is evaluated against it.

```yaml
priority: should
status: delivered 2026-08-03; see as_built
as_built:
  surface: GenerateOptions.BuiltinElements and .PassthroughElements, normalized into an immutable snapshot before analysis, so a registration mistake is reported against the generate command rather than against the first template that uses the element
  open_question_answered:
    registry: options fields rather than the CallRegistry; the whitelist is data a command declares, and the registry exists to resolve Go symbols, which this deliberately does not do
  closed_space: an undeclared hyphenated element is a generation error naming file, line, and column, with a nearest-name suggestion; a project registering none regenerates byte for byte
  foreign_content: SVG and MathML subtrees are outside the whitelist, tracked by depth in both the analysis walk and the emitter
  patterns: passthrough entries accept an exact name or a prefix glob ending at a hyphen; a builtin always wins over a glob covering it
  registration_time_checks: duplicate name, name declared as both kinds, malformed name or glob, hole with no matching parameter or provider, provider filling no hole, provider without its Result type named, hole in a position that cannot be escaped as a value, opaque shape
  analysis_time_checks: undeclared element, unknown attribute, missing required attribute, attribute type mismatch, children supplied to a definition declaring none, head-only element written in the body
  vary_axis: declared per entry, rolled up over the call graph, and emitted as Plan.Vary; readable as Fragment.Vary, Wrapper.Vary, and MergeVary
  assets: a definition's Assets join the required set of every component writing the element, folding into the same transitiveAssets walk as head declarations
  not_built:
    provider_symbol_resolution: a signature is checked by the Go compiler rather than here, as with ContextExternals; resolving Go symbols would mean loading the target package, which this package does not depend on
    head_region: a builtin element inside a head declaration is still refused by the head validator, so PlaceHead means "refuse in the body" and not yet "accept in the head"
    scripts_field: requirement:framework-script-contribution is not built, so no script contribution field ships
    cli_spelling: the passthrough list is a Go option only
source:
  - concept:framework-template-extensions
  - user design discussion 2026-07-27
  - user whitelist decision 2026-07-27
  - downstream framework component seam report 2026-07-31
review_gate: proposed
downstream_confirmation:
  when: 2026-07-31, per decision:library-component-seams
  reached_independently: the reporter asked for components a library implements and callable by name, naming csrf-token first, without having read this requirement; no part of it is implemented, so it could not have been found in the source
  their_reasons: the value never enters template scope, placement is checkable, and an author can find it, move it, and delete it
  not_sugar_over_an_external: agreed on both sides; requirement:render-context-externals is the other half and neither replaces the other
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
declared_per_entry:
  placement:
    values: head, body, or either
    why: a head-only contribution used in the body becomes a generation error rather than a page that half works
    relation: narrower than the data:builtin-element-definition insertion context, which says where the markup parses and not which region owns it
  vary_axis:
    what: the request property the element's output depends on, such as a cookie or a header
    why: an element reading a cookie makes the whole response vary on it, and nothing in the template says so; the caller cannot build a Vary header for what it cannot see, and an output cache cannot refuse to store what it cannot key
    surface: reported with the rest of the composition's capabilities, so a caller reads it from the value it holds
    added: 2026-07-31, per decision:library-component-seams
  assets: requirement:component-asset-requirements
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
  - a head-only element written in the body fails generation, naming the element and the position
  - an element whose output varies on a cookie reports that axis to the caller, so a shared cache can key or refuse the response
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
