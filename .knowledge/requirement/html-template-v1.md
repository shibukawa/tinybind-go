---
id: requirement:html-template-v1
type: requirement
title: HTML Template V1
---
Generate streaming HTML from statically known markup and typed component composition.

```yaml
source: concept:typed-template-language
output: html
declaration: lowercase component keyword with PascalCase name from decision:template-declaration-kinds
parser: decision:template-parser-delegation
ast:
  type_ids: [html:component, html:element, html:attribute, html:text]
  embedded_nodes: [template:expression, template:if, template:for]
  contexts: [html:child, html:attribute, html:url, html:script, html:style]
structure:
  elements: lowercase names
  component_calls: uppercase names with named arguments
  children: elements, calls, text, expressions, if, and for
  nested_content: reserved children parameter of type html
  slots: requirement:html-slot-syntax reserved slot element
  reserved_elements: slot is reserved as an element name and never emitted; template is a fill block only with a slot attribute directly under a component call; the slot attribute name stays ordinary
  raw_text: script and style content use distinct insertion contexts
attributes:
  names: static
  values: expressions allowed; block if and for forbidden
  url: requires url type where URL policy applies
  boolean: emit name only when true; omit when false
  optional: omit whole attribute when absent
escaping:
  text: HTML text-context escaping
  attribute: HTML attribute-context escaping
  control: requirement:explicit-output-control
  raw_html: only trusted_html in child-node position
  script_data: only script_json or trusted_javascript in script content
  style_data: only trusted_css in style content
forbidden:
  - dynamic tag or attribute names
  - arbitrary attribute spreads
  - conditional attribute groups
  - complete intermediate DOM
acceptance:
  - HTML parser discovers embedded boundaries and supplies the active html context to shared nodes
  - inserted strings cannot inject markup
  - text, ordinary attribute, URL, boolean, script, and style contexts are distinguished
  - static HTML writes directly to an output stream
```
