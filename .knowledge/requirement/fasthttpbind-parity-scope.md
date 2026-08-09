---
id: requirement:fasthttpbind-parity-scope
type: requirement
title: fasthttpbind Parity Scope
---
Define which parts of the httpbind surface fasthttpbind must reproduce, which need reimplementation rather than adaptation, and which are out of scope for v1.

```yaml
status: proposed 2026-08-08
v1_required:
  - api:fasthttpbind-bind covering query, payload, path, header, cookie, and File fields
  - api:fasthttpbind-write for Write, WriteStatus, and WriteError
  - policy:problem-details bodies byte-identical to httpbind
  - requirement:bind-check-validation, unchanged; it reads bound values, not the transport
  - OpenAPI output identical, since it derives from the field plan and not from the transport
already_transport_neutral:
  htmlbind_render: the runtime writes to io.Writer and its Flush duck-types Flush and Flush error, so a *bufio.Writer from SetBodyStreamWriter already satisfies it
  reading: the HTML rendering half needs no port, only a caller
  scope_effect: filesystem page rendering is closer to portable than the REST half is
needs_reimplementation_not_adaptation:
  httpbind_stream: asserts http.Flusher; the fasthttp shape is api:fasthttpbind-stream over SetBodyStreamWriter
  htmlupdate_record_writer: holds an http.Flusher field, so the live boundary delivery path is transport-bound in the runtime, not only in generated code
  reason: both depend on flushing inside a handler that has not returned, which is precisely what fasthttp inverts
v1_deferred:
  - live boundary streaming and the update endpoint, pending the htmlupdate port above
  - the discovered router registry for fasthttp, pending decision:fasthttpbind-generator-backend-selection
server_actions_2026_08_08:
  was: deferred to the adapter side, since they are http.HandlerFunc by definition
  now: decision:backend-build-tag-mode leaves no adapter, so they go through rule:transform-eligibility like any other handler and a refused one fails the build
  consequence: they move from deferred into v1, because there is nowhere else for them to run
out_of_scope:
  - HTTP/2, which fasthttp implements on no toolchain
  - making any deferred item work through decision:fasthttpbind-adapter-boundary and calling it parity
tls:
  host_go: available; the fork is upstream behaviour for behaviour there, so ServeTLS works
  tinygo: impossible, and out of scope by decision:fasthttpbind-tinygo-not-first-class rather than by omission
verification:
  shared_suite: one conformance suite runs the same request set against both backends and compares status, headers, and body bytes
  divergence_policy: a difference is a defect unless it is recorded here or in rule:fasthttpbind-body-limit-mapping
related:
  - requirement:fasthttpbind-product-goals
  - api:fasthttpbind-stream
  - requirement:chain-render-pipeline
  - requirement:component-redraw-endpoint
  - decision:fasthttpbind-runtime-package
```
