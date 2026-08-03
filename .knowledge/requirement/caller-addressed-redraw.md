---
id: requirement:caller-addressed-redraw
type: requirement
title: Caller Addressed Redraw
---
Let the caller choose the URL a redraw is addressed to, by recognizing a redraw request mode and carrying kind and instance in headers rather than in the path.

```yaml
priority: must
source:
  - downstream framework caller-owned runtime report 2026-08-04, against v0.3.3
  - requirement:component-redraw-endpoint open_questions
review_gate: proposed protocol surface requires user approval
shipped_today:
  client_builds_the_url: htmlupdate/runtime.js redraw composes the path prefix, the kind, and the instance itself
  no_mode_on_the_request: Negotiate parses navigation and live only; redraw is a response echo and nothing reads it on a request
  handler_reads_the_path: Options.RedrawHandler splits kind and instance out of r.URL.Path under a fixed prefix
  reading: the caller mounts the route, owns the prefix, owns the registry, and renders every refusal, while the module's client decides the address
not_a_deviation_exit:
  what_the_catalog_says: requirement:render-mode-negotiation modes.redraw states the redraw is addressed by URL rather than by the mode header, and selection.endpoint says boundary mode targets the generated update endpoint
  so: this changes a recorded design choice rather than retiring a temporary shape
  contrast: requirement:update-wire-contract and requirement:runtime-default-retirement are scheduling; this one is not, and saying so is what keeps the change honest
  half_already_open: requirement:component-redraw-endpoint asks whether the instance id belongs in the path, for the weaker reason of its cost as a cache key
why_the_caller_needs_the_address:
  authorization: path protection is configured by path pattern, so a redraw on a reserved path needs a second pattern maintained in parallel with the one protecting the page, and nothing forces the two to agree
  at_the_page_url: the redraw inherits the page's protection automatically, and with the branch in the page handler it inherits that handler's own checks rather than only the middleware's
  reading: this is the caller's own endpoint, and its address is the caller's decision
headers_not_query:
  chosen: headers, on the same prefix family as the render, manifest, build, and live headers
  why_not_query: the generated typed decoder treats an unknown parameter name as an error, so query-carried kind and instance reserve names an author may then not declare
  fits: requirement:update-protocol-naming-ownership, since the names compose from the one configured prefix rather than being literals
cache_hazard_this_creates:
  what: requirement:redraw-cache-policy ships a private revalidatable response with a keyed ETag, varying on the build and render headers
  depended_on: the URL identifying which component the bytes belong to
  breaks_when: two reloadable components on one page redraw at the same page URL, where a cache may answer one with the other's fragment
  fix: the response varies on the kind and instance headers too
  already_covered: requirement:render-mode-negotiation never_share bypasses a shared cache that cannot vary
  severity: the one correctness item in the change, and it is created by the change rather than found by it
build_mismatch_diverges:
  path_mounted: RedrawHandler answers 409 FailureStalePage, because no page exists at that address to fall back to
  request_driven: resolve to not-a-redraw and let the caller render its page normally, which is what a page URL makes available
  cheaper: a stale path-addressed redraw costs a 404 and then a reload; a stale page-addressed one returns the page itself
  stated_because: the same condition otherwise carries two answers with nothing saying which applies where
ask:
  mode: Negotiate recognizes a redraw request mode on the same header and token grammar as the others
  addressing: kind and instance travel in headers, so a redraw is addressable at any URL
  entry: a redraw entry answering from a request rather than from a mounted route, so a caller invokes it inside its own handler
  precedent: the action path already has this shape, where the caller issues the request and the module applies what came back
path_form:
  removed: 2026-08-04, rather than kept as a compatibility shape
  why: an endpoint whose address the caller cannot choose is the defect this concept names, and keeping it alive would publish two addressings in one contract for a module with no released consumer of the old one
  nothing_is_lost: a deployment wanting a dedicated route writes five lines that set the three headers from path values and call Redraw, which is the same code at any URL it picks
  who_pays: whoever writes those five lines takes the parallel-path-pattern problem back deliberately, which is the right place for that cost
constraints:
  - a redraw stays a GET and stays side-effect free
  - rule:redraw-input-trust is unchanged, because moving the address changes nothing about the arguments being attacker controlled
  - a caller using the request entry and mounting nothing publishes no reserved path at all
acceptance:
  - a redraw addressed at a page URL renders the same fragment as the same redraw at the mounted route
  - two components redrawing at one page URL are never answered from each other's cached response
  - a caller invokes a redraw inside its own handler, after its own authorization check
  - a stale page redrawing at its own URL receives that page rather than a 404
as_built:
  shipped: 2026-08-04
  mode: ModeRedraw, recognized by Negotiate on the same header and grammar as navigation and live
  headers: '<prefix>-Kind and <prefix>-Instance, composed from the one configured prefix'
  entry: Options.Redraw(w, r, reg) bool, which answers a redraw and reports whether it did, so a caller branches on it inside its own handler after its own authorization
  vary: Redraw declares render, build, kind, and instance whichever response it serves, because the page and the redraw share a URL; the mounted route keeps build alone, since its URL already names the component
  build_mismatch: Negotiate answers a stale redraw with ModeDocument, so Redraw returns false and the caller renders its page; there is no stale-page status left, because there is no longer an address with no page behind it
  deleted: RedrawHandler, RedrawPath, splitRedrawPath, the Mount redraw registration, and FailureStalePage, which had become reachable only from the mounted route
  mount_signature: Mount(router) no longer takes a registry, because the runtime asset is the only endpoint this package still owns
  failure_rename: FailureMalformedPath became FailureMalformedRequest, since the remaining case is a redraw naming no component rather than a path that did not split
  client: the reference runtime addresses the page's own URL by default and takes an explicit url for a deployment still routing to the mounted path
  found_while_building:
    kind_attribute_was_a_literal: runtime.js read 'data-tb-kind' while the generator emits 'data-<prefix>-kind', so a deployment that renamed the prefix rendered one name and looked for another and redraw silently stopped working
    how_it_surfaced: writing requirement:update-wire-contract, which is the argument for the contract rather than an aside; the defect was invisible while the only client was the one that shared the mistake
    fixed: KIND_ATTR derives from the configured prefix and is exposed as kindAttribute, with a harness case asserting it follows a renamed prefix
related:
  - requirement:component-redraw-endpoint
  - requirement:render-mode-negotiation
  - requirement:redraw-cache-policy
  - requirement:update-protocol-naming-ownership
  - requirement:update-wire-contract
resolved:
  mounted_handler: deleted outright on 2026-08-04 rather than deprecated on a schedule; pre-1.0 with one known consumer, and the deprecation window would have cost a second published addressing in the contract for its whole length
```
