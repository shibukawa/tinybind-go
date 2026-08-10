---
id: decision:fasthttpbind-adapter-boundary
type: decision
title: net/http Handlers Behind An Adapter Boundary
---
Mount handlers that own the net/http transport behind a fasthttp-to-net/http adapter at declared routes, and report the split, because an adapted route is slower than plain net/http.

```yaml
status: not implemented, 2026-08-08; decision:backend-build-tag-mode removes the fallback this describes
why_it_cannot_coexist_with_the_tag: a wrapped handler is wrapped so it can keep calling Bind and Write unchanged, and those symbols are tagged out of a fasthttp build; there is no arrangement in which both hold
what_replaces_it: a refusal, per rule:transform-eligibility, reported by requirement:transform-diagnostics and fixed by the author rather than absorbed by the runtime
kept_because: the cost analysis below is what the tag decision was weighed against, and it is the record to reread if the adapter is ever reconsidered alongside a package split, which does permit both
confirmed_by_the_proposer_2026_08_10:
  who: the downstream framework that proposed an adapter in the first place, in its 2026-08-10 survey
  position: the refusal was right, because a buffering adapter preserves neither streaming nor a raw connection, so its guarantee was already holed exactly where they need it
  added: a refusal that names the occurrence is worth more than a silent slow path, which is the reporting_required argument below arrived at from the caller's side
  significance: the party that would have absorbed the cost of the refusal agrees with it, so nothing in this record is waiting on a second opinion
mechanism: a RequestHandler that materializes http.Request and http.ResponseWriter over the RequestCtx and calls the existing handler
cost:
  measured_2026_08_08: constructing the request object costs about 5.8 KB and 14 allocs on the tinybind benchmark path
  arithmetic: an adapted route pays fasthttp parsing plus that construction, so it is slower than the same handler on net/http
  therefore: the adapter is a migration boundary, not a destination
subtree_contagion:
  fact: one middleware requiring *http.Request pulls every route beneath it to the adapted side
  effect: the boundary is decided by the least portable thing in the chain, which is usually a context.WithValue auth middleware
  consequence: the split is rarely as small as it looks before measuring
reporting_required:
  emit: which routes run native and which run adapted, at generation time
  why: without it a migration that got slower is indistinguishable from one that got faster, and the adapter makes silence the likely outcome
  acceptance: the report names the specific declaration that forced each adapted route
entry_condition:
  test: the handler fails the occurrence whitelist of decision:transport-source-transform, meaning some use of w or r is one the rewriter does not recognize
  character: a refusal, not an inference; the transform admits what it can rewrite and sends everything else here, so an unanalyzable handler lands on the safe side by default
  runtime_available: decision:backend-build-tag-mode leaves the net/http surface unconditional, so a wrapped handler's Bind and Write calls compile unchanged inside a fasthttp build; this was the open question of 2026-08-08 and renaming the fasthttp surface answered it
not_translatable_by_any_adapter:
  - a Hijacker assertion, because fasthttp hijacks after the handler returns
  - a Flusher assertion inside a handler, because progressive delivery moves into SetBodyStreamWriter
  - anything retaining r, w, or a derived context past return, per rule:fasthttpbind-requestctx-lifetime
  handling: reject at generation with the declaration named, rather than adapt into a runtime defect
middleware_note:
  framework_owned: shipped in both shapes, with composition order verified identical across the pair
  third_party: exists in net/http shape only, so tracing, metrics, session and CSRF libraries are adapter-bound until ported; this is the practical limit on how much of an application reaches the native side
related:
  - decision:transport-neutral-handler
  - concept:net-http-handler
  - concept:stdlib-wrapper-unwrap
  - rule:fasthttpbind-requestctx-lifetime
  - requirement:fasthttpbind-product-goals
```
