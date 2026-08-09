---
id: decision:framework-tag-boundary
type: decision
title: Where A Framework Puts Its Tag Boundary
---
A framework on top of tinybind keeps one import path and tags a thin layer of type definitions rather than splitting into two packages, because most of its surface has nothing to do with the transport and would otherwise need an alias apiece.

```yaml
status: proposed 2026-08-08
raised_by: downstream framework, 2026-08-08, asking whether every non-HTTP declaration needs an alias
applies_to: a package built on tinybind, not to tinybind itself
why_the_answer_differs_from_tinybind:
  tinybind: decision:backend-build-tag-mode splits httpbind from fasthttpbind, because almost the whole surface is transport-shaped and the split costs nothing
  a_framework: most of its surface is configuration, logging, dependency wiring and rendering, none of which names a transport
  deciding_ratio: the fraction of the package that is transport-shaped; a split makes the transport-free majority pay, and that majority is the package
  conclusion: same mechanism, opposite arrangement, for the same reason
shape:
  one_import_path: yes; the package name never varies, so a caller writes one import either way
  tagged: the type definitions naming a transport, and the few functions that destructure a request or a response
  untagged: everything else, in one copy, with no alias
  aliases_needed: none, which is the point
the_move_that_makes_it_small:
  observation: a function whose signature only names a framework type keeps the same signature text under both tags, because the type name is stable even when its definition is not
  therefore: tag the type, not its users
  example:
    tagged_definition: "type Handler func(w http.ResponseWriter, r *http.Request) against type Handler func(ctx *fasthttp.RequestCtx)"
    untagged_user: "func (a *App) GET(path string, h Handler)"
  effect: routing tables, option structs, and registration APIs stay single-copy; only code that reaches into the request needs two
what_still_has_to_be_tagged:
  rule: a declaration whose body destructures the transport, or whose signature spells a transport type directly rather than through a stable framework name
  propagation: an untagged function calling a tagged one is fine only while that callee's signature is identical under both tags; the moment it is not, the caller is tagged too
  same_boundary: this is the closure rule:transform-eligibility already describes, applied to hand-written framework code instead of to generated code
keeping_the_untagged_side_large:
  method: give the transport-touching layer a transport-free signature wherever it can have one, so the untagged majority calls into it without inheriting a tag
  relation: decision:transport-neutral-handler at framework scale; there it keeps application handlers portable, here it keeps a framework's own interior single-copy
  test: if tagging spreads past the request-handling layer, a signature that should have been transport-free is not
tooling_consequence:
  cost: a tagged file is invisible to an untagged go vet, go test and gopls session, so each tag configuration needs its own run
  mitigation: keep the tagged layer thin enough that a human can read both copies side by side
related:
  - decision:backend-build-tag-mode
  - decision:transport-neutral-handler
  - rule:transform-eligibility
  - decision:framework-integration-seams
  - system:tinybind
```
