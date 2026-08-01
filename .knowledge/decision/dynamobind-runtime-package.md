---
id: decision:dynamobind-runtime-package
type: decision
title: dynamobind Runtime Package And Dependency Direction
---
Add a DynamoDB binding runtime as its own package that imports the driver, and never let the driver import tinybind-go.

```yaml
status: accepted
extends: decision:runtime-package-boundaries
package:
  name: dynamobind, matching jsonbind, sqlbind, htmlbind and configbind
  path: github.com/shibukawa/tinybind-go/dynamobind
owns:
  - api:dynamobind-operations
  - the item codec interfaces ItemEncoder, ItemDecoder, Keyer
  - decode field errors, reusing the jsonbind FieldError shape where it fits
imports:
  - github.com/shibukawa/tinygodriver/nosql/dynamodb
driver_version:
  minimum: v1.1.3, the release that introduced nosql/dynamodb
  effect: adding dynamobind makes the driver a required dependency of a runtime package, not only of an example
excludes:
  - net/http beyond what the driver itself pulls in
  - database/sql
dependency_direction:
  - user package -> dynamobind -> tinygodriver/nosql/dynamodb
  - user package -> tinygodriver/nosql/dynamodb, because generated code names driver types
forbidden:
  - tinygodriver importing tinybind-go, including its examples and tests
  - a dynamobind example living in tinygodriver; it belongs in tinybind-go
cycle_note:
  fact: Go forbids package cycles, not module requirement cycles, so a driver-side import would still build
  reason_to_avoid: it makes the two releases order-dependent
current_edge_2026_07_31:
  only_go_import: examples/demo imports tinygodriver v1.0.3
  string_only: generator/options.go and generator/options_test.go name tinygodriver/httpmux as a TypePattern PackagePath, not an import
  option: giving examples/demo its own go.mod removes the module edge entirely
generated_code_placement:
  location: the user package
  may_import: both dynamobind and the driver
  declares: only the methods of its own declared types, per decision:generated-runtime-in-module
related:
  - requirement:dynamobind-product-goals
  - system:tinygodriver-dynamodb
  - decision:dynamobind-static-dispatch
  - requirement:tinygo-wasm
  - system:tinybind
```
