---
id: rule:openapi-godoc-descriptions
type: rule
title: OpenAPI Descriptions from Go Doc Comments
---
Go doc comments discovered during analysis become OpenAPI documentation text; no separate annotation syntax exists.

```yaml
extraction: comment markers stripped, text copied verbatim
operation:
  source:
    - handler func doc comment
    - handler type doc comment
    - ServeHTTP doc comment when the handler type has none
  summary: first sentence
  description: remaining text, omitted when empty
schema:
  source: request or response struct doc comment
  target: description
property:
  source: struct field doc comment or trailing line comment
  target: description
parameter:
  source: struct field doc comment or trailing line comment
  target: parameter description, not schema description
deprecated:
  source: paragraph starting with "Deprecated:"
  target: deprecated true on operation, schema, and property
absent_doc: field omitted
inline_handler: no doc source, operation carries no summary
related:
  - concept:openapi-generation
  - rule:openapi-validation-metadata
  - decision:single-source-of-truth
  - concept:handler-discovery
```
