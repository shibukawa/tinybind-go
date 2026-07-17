---
id: rule:generator-feature-disable
type: rule
title: Generator Feature Disable
---
Configured feature disablement removes discovery, emitted code, registration, imports, and CLI artifacts for that feature.

```yaml
status: required
features:
  - route discovery
  - OpenAPI generation
  - Bind
  - Write
  - WriteStatus
  - DecodeJSON
  - EncodeJSON
  - streaming
  - ScanRows
  - multipart File
behavior:
  - disabled operation ignores RuntimePackages and operation-specific Set
  - disabled operation contributes no nested helper closure
  - disabled operation contributes no generated import or registry entry
  - disabled OpenAPI makes the openapi flag unavailable or permanently false
  - generate-all never overrides disablement
validation:
  - reject contradictory options that require a disabled dependency
  - report disabled call sites only in explicit check mode
related:
  - api:generator-main
  - data:generator-options
  - rule:usage-directed-generation
  - requirement:configurable-generator-discovery
```
