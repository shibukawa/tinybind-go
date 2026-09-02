---
id: requirement:tinygo-wasm
type: requirement
title: TinyGo and WASM Support
---
TinyGo is a first-class target; reflection-free generated code must work on TinyGo, WebAssembly, and embedded environments.

```yaml
priority: first-class
baseline:
  tinygo: 0.42.0
  go: 1.27.x
  moved_2026_09_02: from TinyGo 0.41.1 + Go 1.26.x, with the module's go.mod line raised to 1.27.0 alongside it
  what_the_move_broke: 'TinyGo 0.42.0 cannot compile Go 1.27 go/types -- it reaches internal/runtime/maps, whose abi.MapType and abi.MapGroupSlots are absent from TinyGo''s internal/abi overlay'
  why_that_reached_a_runtime_suite: 'a TinyGo test binary compiles every test file in the package, so selecting runtime tests with -run does not keep a host-only import out of the build; internal/mappingfixture''s generator tests now carry a !tinygo tag, which is what the -run filter was being read as doing'
  generalization: any host-toolchain import in a package this project runs under TinyGo needs a build tag, not a -run filter
verified:
  host_tinygo: runtime and generated mapping checks pass
  js_wasm_json: JSON-only generated fixture builds with tinygo build -target wasm
  js_wasm_http: root HTTP runtime builds on the baseline; the roundtrip_js.go compile failure seen on TinyGo 0.41.1 + Go 1.26.x is gone, verified 2026-09-02 by a tinygo build -target wasm of a program calling http.Get with the root runtime imported
v0_1_3_boundary:
  solved: usage-directed output omits net/http for DecodeJSON / EncodeJSON-only generation
  remaining: importing the root runtime still compiles registry.go and other net/http files
resolution:
  module: github.com/shibukawa/tinybind-go
  JSON-only runtime: jsonbind
  HTTP runtime: httpbind root package
  SQL runtime: sqlbind
  dependency_check: jsonbind dependency graph excludes net/http and database/sql
acceptance:
  - importing JSON-only runtime code does not compile net/http or database/sql
  - JSON-only generated code imports only the JSON runtime boundary
  - host Go behavior and deterministic generation remain unchanged
targets:
  - TinyGo
  - WebAssembly
  - embedded environments
depends_on:
  - decision:reflection-free
  - decision:runtime-package-boundaries
related:
  - vision:tinybind
  - system:tinybind
```
