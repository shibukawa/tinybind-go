---
id: requirement:redraw-cache-policy
type: requirement
title: Redraw Cache Policy
---
Let the caller supply the cache policy for a redraw response, because the fixed no-store the handler writes forbids the conditional request this catalog already asked for.

```yaml
priority: should
source:
  - downstream framework runtime ownership report 2026-08-01, against v0.3.0
  - requirement:component-redraw-endpoint response
review_gate: proposed
shipped_today:
  redraw.go: sets the HTML content type, 'Cache-Control: no-store', and the echoed render mode header on every redraw response
  reason_in_code: the URL alone identifies the response and the content is usually per-user, so a shared cache must never hold it
contradiction:
  requirement:component-redraw-endpoint says:
    cache: private by default, because a redraw usually renders per-user content
    conditional: an ETag over the rendered bytes lets an unchanged redraw answer 304, reusing HTTP instead of a bespoke manifest
    acceptance: an unchanged redraw can answer 304
  handler_does: no-store, which forbids the conditional request entirely
  reading: the code took the strictest reading of privacy and dropped the revalidation half; private and no-store are not the same answer
  found_by: the downstream framework, reading the requirement against the handler
not_contested:
  - the content type on every update path
  - the echoed mode header, which requirement:render-mode-negotiation needs so a proxy-substituted body is detectable
  - 'no-store on a delta and immutable on the runtime asset, both of which the reporter calls correct'
ask: the caller supplies the cache policy for the redraw response
constraints:
  - the default stays private rather than shared, per requirement:component-redraw-endpoint and requirement:render-mode-negotiation delta_privacy
  - a redraw response still varies on the render header, so a document request can never be answered from it
  - a caller relaxing the policy takes responsibility for what rule:redraw-input-trust makes attacker-controlled input
acceptance:
  - an unchanged redraw answers 304 to a conditional request
  - a per-user redraw defaults to a policy no shared cache may hold
  - a caller supplying nothing gets a private, revalidatable response rather than today's no-store
as_built:
  shipped: 2026-08-01
  default: 'private, no-cache, plus an ETag over the rendered bytes'
  why_no_cache_not_no_store: no-store would forbid the conditional request the ETag exists for, because a browser that may not keep the bytes can never ask whether they changed
  conditional: If-None-Match is honoured, including the list form and the weak prefix, and an unchanged redraw answers 304 with no body
  vary: the response now varies on the build header, which the fixed no-store had made unnecessary and a cacheable response makes required
  vary_becomes_load_bearing: this policy assumes the URL identifies which component the bytes belong to; requirement:caller-addressed-redraw removes that assumption, so the kind and instance headers join the vary set or a cache may answer one component with another's fragment
  override: Options.RedrawCacheControl
  open_question_resolved: the ETag is computed by the module over the rendered bytes, which is free because the render is already buffered
  keyed: the tag is an HMAC under Options.Key, because a 304 confirms a guess and a redraw usually renders low-entropy per-user content; this is rule:update-validator-computation reasoning arriving at a second surface
  found_while_building: an unkeyed content digest would have made the endpoint a guess-confirmation oracle, which the module already refuses for frame validators and had no reason to accept here
etag_after_the_json_body_2026_08_08:
  covers_the_head: the digest is over the whole body, so a changed head contribution invalidates as a changed region does; decided by the owner, and it is what a head carried in the body means
  what_a_304_stops_carrying: the head header was set before the conditional check, so a 304 delivered it; in the body a 304 delivers nothing at all
  works_because: private, no-cache leaves the conditional request to the browser's own cache, which replays the stored 200 body
  breaks_if: a client manages validators itself in storage or a service worker, where a 304 would confirm markup it no longer holds the head for
  same_shape_for_every_body_field: the live marker and the manifest entry are equally invisible on a 304, so the reading has to be that a 304 means every part of the previous response is still current, not only its markup
  settled: 2026-08-08, the browser's own cache owns the conditional request
  consequence_that_dissolves_the_concern: the browser revalidates, receives the 304, and reconstructs the full 200 from its store, so a client fetch never observes a bodiless response and head, live, and manifest always arrive
  the_one_obligation: a client must not set If-None-Match itself; doing so returns a bare 304 and loses every body field, which is the failure this section was written about
  why_this_way: private, no-cache already puts the store in the browser, and a client managing validators would have to cache the head beside them to stay correct — a second cache to keep in step with the first
  unchanged_never_means_correct: a revalidated response says the bytes are current, never that the screen is; see requirement:component-delta-rendering for the case that makes the difference visible
the_render_side_is_a_separate_gap:
  what: this concept gave the caller the response cache policy; the render itself still takes no htmlbind option, so a cached component redrawn alone renders uncached whatever this policy says
  wider: requirement:fragment-render-options, which found that the redraw and action entries pass no render option at all, and that two of the absences fail rather than default
  reading: the HTTP-level and the render-level caches were adjacent enough that closing one read as closing both
superseded_by_decision_caller_writes_the_response_2026_08_09:
  what_this_concept_asked_for: the caller supplies the cache policy, with a private revalidatable default when it supplies nothing
  what_shipped_instead: no default at all, because the module no longer writes a header field of any kind
  removed: Options.RedrawCacheControl and Options.SequenceCacheControl, along with the DefaultRedrawCacheControl and DefaultSequenceCacheControl constants they overrode
  kept: the ETag, which only this module can compute, and Response.NotModified, which does the If-None-Match comparison a caller would otherwise reimplement
  the_304_is_now_the_callers: answering one is a cache decision, and a module writing no cache policy has no business making it
  acceptance_still_met: an unchanged redraw still answers 304 to a conditional request, by a caller writing four lines the guide gives verbatim
  acceptance_no_longer_met_by_default: 'a caller supplying nothing now gets no Cache-Control rather than private, no-cache; the suggestion survives in docs/httpbind_update_surface.md as prose'
  the_open_question_below_is_answered: yes, the same seam covers the delta and stream paths, and it covers Vary and the content type too
related:
  - requirement:component-redraw-endpoint
  - requirement:render-mode-negotiation
  - requirement:component-output-cache
  - requirement:fragment-render-options
  - decision:caller-writes-the-response
open_questions:
  - whether the same seam covers the delta and stream paths, whose no-store the reporter accepts
```
