---
id: rule:transport-dead-code-elimination
type: rule
title: An Unused Transport Surface Costs Nothing If Nothing Reaches It
---
Both linkers already remove an imported but uncalled transport surface, so generation must not make the unused one reachable, and a registering init is the way that happens.

```yaml
status: measured 2026-08-08
question_answered: whether leaving the net/http files untagged, per decision:backend-build-tag-mode, inflates a fasthttp binary
measurement:
  method: a program serving on the fork, built twice; the second also imports a package whose exported declarations take http.Request and http.ResponseWriter and are never called
  host_go:
    fasthttp_only: 7762082
    plus_unused_net_http_surface: 7762082
    delta: zero; the binaries are byte-identical
  tinygo:
    tags: fasthttp_nozstd
    fasthttp_only: 2852880
    plus_unused_net_http_surface: 2852832
    delta: 48 bytes, which is noise rather than linked code
  environment: darwin/arm64, Go 1.26.x, TinyGo 0.41.1
  also_measured:
    both_transports_actually_serving: 8700530 on host Go, against 7762082 for the fork alone
    reading: even genuinely running both costs under 1 MB, because they share net, crypto/tls, bufio, mime and the compression packages
conclusion_superseded_2026_08_08:
  measured_claim_stands: an unused transport surface costs nothing, and both linkers prove it
  what_changed: decision:backend-build-tag-mode is taken for structural enforcement rather than for size, so this measurement no longer argues against it
  standing_use: it is the evidence that the arrangement is not paying twice, and the acceptance test below is what keeps that true
correction_2026_08_08_the_graph_cannot_be_kept_clean:
  found: system:tinygodriver-fasthttp itself imports net/http, in fs.go, for http.FS, http.DetectContentType and http.TimeFormat
  consequence: no packaging arrangement keeps net/http out of a fasthttp build's import graph, because the transport puts it there; the structural-safety claim of decision:backend-build-tag-mode is therefore weaker than stated
  but_the_binary_is_still_clean:
    probe: count net/http server symbols in each binary with go tool nm
    fasthttp_only: 41 net/http symbols total, 0 of them from ListenAndServe, ServeMux, Server or conn
    net_http_only: 43 server-path symbols
    reading: the linker drops the net/http server machinery from a fasthttp binary; what survives is the shallow set fasthttp itself references
  restated_goal: the property worth defending is that no net/http server path is reachable, not that the package is absent from the graph
  consistency: this is why an unused net/http surface measured at zero bytes above; the graph already contained net/http and the linker was already pruning it
what_preserves_this:
  condition: nothing reachable from main refers to the unused surface
  threat: a generated init that registers a net/http binder, which is reachable by construction and drags the whole transport in behind it
  evidence: generated files emit 'func init() { httpbind.RegisterBind[T](bindT) }' where bindT takes *http.Request, so the registration alone makes the binder live
  required: in a fasthttp build, generation registers the fasthttp binder and writer for a type whose handlers were all transformed, and emits the net/http registration only for a type still reached through decision:fasthttpbind-adapter-boundary
  second_threat: any reachable code placing a net/http-typed value into an interface, which keeps its method set
verification:
  add: a size assertion comparing a transformed application against the same application with no net/http handler left
  why: this property is a consequence of what generation emits, not of the tag, so nothing enforces it unless something measures it
related:
  - decision:backend-build-tag-mode
  - decision:fasthttpbind-adapter-boundary
  - decision:transport-source-transform
  - requirement:fasthttpbind-tinygo
  - rule:usage-directed-generation
```
