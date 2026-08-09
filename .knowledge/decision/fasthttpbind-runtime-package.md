---
id: decision:fasthttpbind-runtime-package
type: decision
title: fasthttpbind Runtime Package And Dependency Direction
---
Add the fasthttp binding runtime as its own sibling package that excludes net/http, rather than adding a build-tagged transport inside httpbind.

```yaml
status: superseded in part, 2026-08-08, by decision:backend-build-tag-mode
extends: decision:runtime-package-boundaries
what_was_superseded:
  api_spelling: the fasthttp declarations reuse the net/http names rather than taking a fasthttpbind prefix, because decision:backend-build-tag-mode imports the package under the httpbind alias; api:fasthttpbind-bind, api:fasthttpbind-write and api:fasthttpbind-stream describe those same-named declarations
  shared_leaf_added: the transport-free declarations move out of the root package so neither surface owns them, which the sibling model below did not anticipate
what_still_holds:
  - the dependency direction and the forbidden edges below
  - fork_over_upstream, which the toolchain requirement decides and packaging does not touch
  - excluding database/sql, and never linking both transports into one binary, which the tag now enforces instead of the package split
  - duplication_accepted, which is the same trade under either arrangement
package:
  name: fasthttpbind, matching jsonbind, sqlbind, htmlbind, dynamobind and firestorebind
  path: github.com/shibukawa/tinybind-go/fasthttpbind
owns:
  - api:fasthttpbind-bind
  - api:fasthttpbind-write
  - api:fasthttpbind-stream
  - the RequestCtx-shaped binder and writer registries
  - concept:fasthttp-handler content negotiation, reusing the httpbind rules
imports:
  - github.com/shibukawa/tinygodriver/fasthttp
  - github.com/shibukawa/tinybind-go/jsonbind
fork_over_upstream:
  chosen: system:tinygodriver-fasthttp, not github.com/valyala/fasthttp
  reason: decision:fasthttpbind-tinygo-not-first-class still requires the package to compile under TinyGo, which upstream cannot do
  consequence: fasthttpbind types are not upstream's types, so third-party fasthttp middleware and routers cannot wrap its handlers
excludes:
  - net/http
  - database/sql
dependency_direction:
  - user package -> fasthttpbind -> system:tinygodriver-fasthttp
  - user package -> system:tinygodriver-fasthttp, because handler signatures name RequestCtx
  - fasthttpbind -> jsonbind, the same edge httpbind already has
forbidden:
  - httpbind -> fasthttpbind, or the reverse; neither transport may pull the other in
  - a shared package holding the binder registry, which would link both transports into every binary
  - tinygodriver importing tinybind-go
rejected_alternative:
  shape: build tags inside the httpbind root package selecting the transport
  why_rejected:
    - the root package is the published import path of api:bind; a tag flipping its signatures breaks every caller by build configuration rather than by declaration
    - requirement:tinygo-wasm exists because import graphs decide what gets compiled, and a tag does not remove net/http from the graph of an untagged build
    - decision:runtime-package-boundaries already answers this question by package, and a second answer by tag would contradict it
duplication_accepted:
  what: field-plan emission runs twice, once per transport
  why: the shared alternative is an interface, refused by decision:fasthttpbind-no-transport-interface
  bounded_by: both emitters read one field plan, so the wire contract has a single source
error_bodies: policy:problem-details, byte-identical to httpbind
related:
  - requirement:fasthttpbind-product-goals
  - system:tinygodriver-fasthttp
  - decision:generated-runtime-in-module
  - requirement:tinygo-wasm
  - system:tinybind
```
