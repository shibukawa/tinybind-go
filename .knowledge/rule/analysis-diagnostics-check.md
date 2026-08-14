---
id: rule:analysis-diagnostics-check
type: rule
title: Analysis Diagnostics Check
---
Host check (petitweb check / tinybind check) reports registration sites that look like routes but cannot be fully analyzed.

```yaml
status: implemented
requirement: requirement:analysis-diagnostics
tooling:
  name: check command (petitweb check or tinybind check)
  host_only: true
severity:
  preferred: error on undiscoverable route-like registration in analyzed packages
  optional: warning mode for migration
silent_drop_forbidden: true
# Must not omit OpenAPI path without a diagnostic when a registration was candidate but incomplete.

trigger_examples:
  - dynamic pattern: '"GET " + path' or non-string pattern arg
  - unanalyzable_middleware: wrapper chain does not yield known leaf
  - cross_package_model: Bind/Write type arg is selector to other package
  - complex_type_arg: pointer *T, nested generic, or unsupported type expr
  - unresolved_symbol: types-check cannot resolve Handle/HandleFunc under rule:go-types-symbol-identity (not a false positive)

non_triggers:
  - intentional non-httpbind handlers with no Bind/Write if product chooses info-only
  - third-party routers outside allowlist (not candidates)

report_fields:
  - file and position
  - registration call summary
  - reason code (dynamic_pattern|opaque_middleware|cross_package_model|complex_type_arg|other); a cross_package_handler code was documented through 2026-08-14 but no pass ever emitted it
  - whether OpenAPI will omit the route

related:
  - requirement:analysis-diagnostics
  - rule:same-package-convention
  - rule:unsupported-route-patterns
  - rule:go-types-symbol-identity
  - concept:openapi-generation
  - flow:code-generation
  - flow:handler-parse
```
