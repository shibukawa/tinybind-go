---
id: data:builtin-element-definition
type: data
title: Builtin Element Definition
---
Generation-time record describing one framework builtin element: its name, typed parameters, output shape, per-request value source, and head contributions.

```yaml
source:
  - requirement:builtin-element-registration
  - concept:framework-template-extensions
status: proposed
fields:
  Kind: builtin, the rewritten form; a decision:builtin-element-syntax passthrough entry carries only a name or pattern and needs no definition
  Name: bare kebab-case element name, unique in the whitelist
  Params: ordered parameter definitions; each carries a name, a declared template type, and a required flag
  Children: optional reserved children parameter of type html
  Context: the rule:template-context-safety insertion context this element may appear in
  Placement: head, body, or either; checked against the region the element is written in, per requirement:builtin-element-registration
  Vary: the request properties this element's output depends on, empty for one that depends on none
  Assets: static files this element requires, per requirement:component-asset-requirements
  Shape: markup or opaque, per requirement:builtin-element-lowering
  Markup: for the markup shape, the fixed output template with named holes
  Provider: optional SymbolPattern for the requirement:render-value-provider function supplying hole values
  Scripts: script contribution names this element pulls in, per requirement:framework-script-contribution
capabilities_derivation:
  rule: capabilities are derived, not declared, so a registration cannot disagree with its own shape
  provider_present: implies per_request and needs_context
  provider_absent: no capability; the element folds entirely into static bytes
  scripts_present: contributes the named requirement:framework-script-contribution entries
  vary_declared: not derived, because only the implementation knows what its provider reads; an undeclared axis is the invisible dependency decision:library-component-seams accepted this field to close
  conservative: a provider returning a process-constant value is still treated as per_request, because the safe direction is cache exclusion
parameter_types: the template types of requirement:template-language-core, so an attribute expression is checked exactly as on an ordinary element
markup_holes:
  reference: a hole names a Provider result field or a declared parameter
  context: each hole carries its own escaping context from its position in the markup template
  static_structure: the markup shape itself never varies per request; only hole values do
capabilities:
  per_request: output varies per request; excluded from reusable cached output
  needs_context: rendering requires a context value, so composition validation demands it
  needs_bootstrap: the element requires requirement:html-runtime-bootstrap in the document
identity:
  symbols: rule:go-types-symbol-identity package path plus name, so the generator never imports the framework at generation time
  import: generated template source imports the framework package only when a builtin element or provider is actually used, per rule:usage-directed-generation
construction:
  owner: the framework generator command, alongside api:generator-call-registration
  immutability: definitions normalize into an immutable snapshot before package analysis
  duplicate: two definitions sharing a Name fail registration with an actionable diagnostic
example_csrf:
  Kind: builtin
  Name: csrf-token
  Params: []
  Context: html:child
  Shape: markup
  Markup: '<input type="hidden" name="{{.FieldName}}" value="{{.Token}}">'
  Provider: framework package TokenFor function; its result fields resolve both holes by name
  derived_capabilities: [per_request, needs_context]
  note: both holes sit in attribute-value context, so both take attribute escaping
example_telemetry:
  Name: otel-tracing
  Params: [serviceName required string]
  Context: html:child
  Shape: markup
  Markup: '<meta name="otel-service" content="{{.ServiceName}}">'
  Scripts: [otel-runtime]
  derived_capabilities: []
  note: no provider, so the whole element folds into static bytes and costs nothing at render time
related:
  - data:generator-options
  - api:generator-call-registration
  - requirement:explicit-output-control
```
