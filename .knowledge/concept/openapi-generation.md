---
id: concept:openapi-generation
type: concept
title: OpenAPI Generation
---
OpenAPI package fragments are generated from each package's shared IR and assembled through public httpbind APIs; the document is never hand-edited.

```yaml
primary_source: Go source code
openapi_role: derived artifact only
never: manual OpenAPI edits as source of truth
version: decision:openapi-31
schemas_for:
  - request models
  - response models
  - validation constraints
  - error models
  - streaming event models
field_rules:
  - rule:openapi-input-fields
  - rule:openapi-payload-fields
  - rule:openapi-query-fields
  - rule:openapi-http-metadata-params
  - rule:openapi-payload-rest
  - rule:openapi-nested-schemas
responses:
  - rule:openapi-success-response
  - rule:openapi-success-status
  - rule:openapi-streaming-content
  - rule:openapi-error-statuses
diagnostics: requirement:analysis-diagnostics
validation_tags: rule:openapi-validation-metadata
errors: policy:problem-details
route_analysis:
  - concept:route-discovery
  - concept:stdlib-wrapper-unwrap
  - flow:handler-parse
artifacts:
  - data:openapi-fragment
  - concept:openapi-embed
  - api:openapi-json
  - api:openapi-yaml
  - concept:openapi-ui
goals: requirement:openapi-goals
pipeline:
  - Go source
  - intermediate representation
  - request binder
  - response writer
  - validation
  - error mapping
  - OpenAPI
related:
  - requirement:modular-package-generation
  - requirement:openapi-fragment-aggregation
  - decision:single-source-of-truth
  - flow:code-generation
  - concept:code-generation
  - concept:handler-discovery
```
