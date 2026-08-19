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
  - DynamoDB item codec, proposed by requirement:dynamobind-generated-item-codec
  - DynamoDB table definition emission, per decision:dynamobind-table-definition
  - CBOR wire codec and CBOR world codec, each direction on its own, proposed by requirement:declared-cbor-codec
behavior:
  - disabled operation ignores RuntimePackages and operation-specific Set
  - disabled operation contributes no nested helper closure
  - disabled operation contributes no generated import or registry entry
  - disabled OpenAPI makes the openapi flag unavailable or permanently false
  - generate-all never overrides disablement
not_a_feature:
  - the determinism check of rule:cbor-deterministic-types, for a type reached by a CBOR codec; it is a build gate rather than a lint, and leaving it off is how a float reaches production as a desync
  - open: whether the CBOR mode as a whole may be disabled, which is a different question from disabling the check inside it
validation:
  - reject contradictory options that require a disabled dependency
  - report disabled call sites only in explicit check mode
related:
  - api:generator-main
  - data:generator-options
  - rule:usage-directed-generation
  - requirement:configurable-generator-discovery
```
