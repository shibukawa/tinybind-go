---
id: requirement:configbind-tinygo
type: requirement
title: configbind TinyGo Support
---
configbind runtime and generated code must build and run under TinyGo as a first-class target.

```yaml
priority: first-class
targets:
  - TinyGo
  - host Go
constraints:
  - avoid runtime reflection for field binding
  - decision:configbind-codegen-no-reflect is mandatory
  - keep dependency surface small enough for TinyGo
  - no file-upload or filesystem config value types in v1
acceptance:
  - TinyGo build of package plus generated fixtures succeeds
  - load of defaults, TOML, env, CLI works on TinyGo host target
  - generated apply and CLI paths compile without reflect
  - system:configdir imports build under TinyGo host (smoke verified)
related:
  - system:configbind
  - system:configdir
  - requirement:configbind-product-goals
  - requirement:config-file-discovery
  - decision:configbind-supported-types
  - decision:configbind-codegen-no-reflect
  - decision:configbind-runtime-architecture
  - decision:reflection-free
  - requirement:tinygo-wasm
```
