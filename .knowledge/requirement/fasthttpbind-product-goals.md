---
id: requirement:fasthttpbind-product-goals
type: requirement
title: fasthttpbind Product Goals
---
Serve the same generated binding and writing contract over fasthttp so a throughput-bound application keeps its models, tags, OpenAPI, and generator workflow.

```yaml
status: proposed 2026-08-08
priority: additive; never a replacement for httpbind
target_toolchain: host Go; TinyGo is compile-only per decision:fasthttpbind-tinygo-not-first-class
transport: system:tinygodriver-fasthttp
motivation:
  measured_2026_08_08:
    generated_bind_and_write: 527 ns, 728 B, 11 allocs
    same_path_including_http_Request_construction: 1371 ns, 6590 B, 25 allocs
    inference: about 5.8 KB and 14 allocs per request are the transport object, not the binding
  claim: the binding layer is already cheap, so the remaining win is the request object and the connection layer, which is exactly what fasthttp pools away
  caveat: the delta above is measured through httptest.NewRequest; a real net/http server reuses read buffers, so treat it as an upper bound
non_goals:
  - a runtime abstraction shared by both transports, refused by decision:fasthttpbind-no-transport-interface
  - rewriting or reinterpreting handler bodies written against net/http
  - HTTP/2, which fasthttp implements on no toolchain
  - a TinyGo size or throughput story, dropped by decision:fasthttpbind-tinygo-not-first-class
  - making fasthttp the default backend of cmd/tinybind-gen
workload_fit:
  strong: high request rate, small payloads, no database round trip
  weak: template plus database pages, where htmlbind renders in about 900 ns and one query costs hundreds of microseconds
  requirement: measure a representative application shape before adopting, because the transport can be noise
unchanged_by_this_work:
  - request and response model declarations, including every source tag
  - policy:problem-details error bodies
  - OpenAPI output
  - generator invocation and its unchanged-input skip
acceptance:
  - one model set generates working binders for either backend with no model edit
  - a fasthttp application never links net/http through fasthttpbind
  - httpbind output is byte-identical to a run predating this feature
  - the seam gain is reported: generation names which routes run native and which run adapted
related:
  - system:tinybind
  - decision:fasthttpbind-runtime-package
  - requirement:fasthttpbind-parity-scope
  - requirement:fasthttpbind-tinygo
  - decision:runtime-package-boundaries
```
