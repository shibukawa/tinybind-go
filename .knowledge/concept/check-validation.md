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
    - OpenAPI required/minimum/maximum/minLength/maxLength/pattern/format
excludes:
  default: standalone default tag per decision:default-tag-form
  enum: standalone enum tag per decision:enum-tag-form; still validation, still generated
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
      Age   int    `check:"min=0,max=150" default:"0"`
      ID    string `path:"id" check:"required,uuid"`
      Sort  string `query:"sort" enum:"asc,desc" default:"asc"`
  }
related:
  - decision:default-tag-form
  - decision:enum-tag-form
  - rule:default-tag-semantics
  - rule:enum-tag-semantics
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
