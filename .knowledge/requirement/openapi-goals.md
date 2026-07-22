---
id: requirement:openapi-goals
type: requirement
title: OpenAPI Generator Goals
---
OpenAPI generation must be automatic, deterministic, runtime-synchronized, reflection-free, and fully derived from Go source.

```yaml
goals:
  - generated automatically
  - deterministic
  - synchronized with runtime behavior
  - reflection-free
  - TinyGo compatible
  - independent of handwritten YAML
  - based entirely on Go source code
  - composable across independently generated framework and application packages
developer_rule: never manually maintain the OpenAPI document as source of truth
related:
  - requirement:modular-package-generation
  - requirement:openapi-fragment-aggregation
  - concept:openapi-generation
  - decision:single-source-of-truth
  - decision:reflection-free
  - requirement:tinygo-wasm
  - decision:openapi-31
```
