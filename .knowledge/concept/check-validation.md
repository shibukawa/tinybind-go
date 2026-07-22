---
id: concept:check-validation
type: concept
title: Check Tag Validation
---
Struct-tag validation rules on request models generate runtime checks and OpenAPI constraints from one source.

```yaml
status: designed
tag_name: check
intent: replace handwritten Validation/Field checks with generated validate functions
bind_requirement: requirement:bind-check-validation
ssot:
  input: Go struct check tags
  outputs:
    - generated validateXxx after bind then defaults
    - OpenAPI required/minimum/maximum/minLength/maxLength/enum/pattern/format/default
pipeline: rule:check-codegen-pipeline
pipeline_order:
  - bind
  - validate
  - apply defaults

syntax: rule:check-tag-syntax
rules: rule:check-v1-rule-set
required_semantics: rule:check-required-semantics
formats: rule:check-format-validators
openapi: rule:openapi-validation-metadata
decision: decision:check-tag-validation
example: |
  type CreateUserRequest struct {
      Name  string `check:"required,minlen=1,maxlen=64"`
      Email string `check:"required,email,maxlen=254"`
      Age   int    `check:"min=0,max=150"`
      ID    string `path:"id" check:"required,uuid"`
  }
related:
  - concept:request-binding
  - concept:code-generation
  - concept:openapi-generation
  - concept:error-helpers
  - api:bind
  - requirement:bind-check-validation
  - decision:reflection-free
  - decision:single-source-of-truth
  - requirement:tinygo-wasm
  - vision:tinybind
```
