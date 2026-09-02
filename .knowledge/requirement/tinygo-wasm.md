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
nesting_depth_2026_09_02:
  found: the JSON nesting bound of 10000 protected nothing on TinyGo; SkipValue overflows a native goroutine stack between 500 and 1000 brackets, and the wasip1 default 64 KiB stack at about 100, where the overflow is reported only at exit and the request gets a wrong answer
  measured: native main stack survives 10000; -stack-size=1MB carries wasip1 to 1000 and the engine traps around 2000 whatever the size
  changed: DefaultMaxNestingDepth is 90 and SetMaxNestingDepth raises it process-wide, mirroring SetMaxJSONBodyBytes; jsonbind joined scripts/tinygo-check.sh natively and on wasip1
  why_a_default_rather_than_a_tag: the bound is a property of the smallest stack the package runs on, and a host that wants encoding/json's ten thousand sets it once at startup
tinygo_0_42_cleanup_2026_09_02:
  sqlbind: database/sql compiles under TinyGo 0.42 and tinygo test ./sqlbind passes, so the !tinygo tags on sql.go, registry.go and context_test.go are gone and the package joined scripts/tinygo-check.sh; whether a SQL driver runs there is tinygodriver's question, not this module's
  errors_as: the hand-rolled unwrap loops in bindcore and the root runtime existed for a TinyGo 0.40 gap, reflect.AssignableTo on interface targets, which 0.42 closed; AsHTTPError, IsMessageTooLarge and isRequestTooLarge use errors.As and errors.Is again, verified with json.RawMessage linked on native and wasip1; the AsError walkers in jsonbind, dynamobind and firestorebind stay, since their reason is decision:reflection-free rather than that gap
  local_type_names: the TinyGo name-mangling collision between same-named function-local generic parameter types no longer reproduces on 0.42, so the awaitScope and reporterScope spellings in htmlbind's tests went back to scope
  still_true: recover does not run on wasm targets, SkipNow and FailNow have no Goexit there, AllocsPerRun counts nothing, and Go 1.27's go/types does not compile under TinyGo; every guard for those stays
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
