---
id: decision:cache-scope-seams
type: decision
title: Cache Scope Requests From The Downstream Framework
---
Accept the eighth downstream round, and record that its three asks hold, its companion was already closed under another name, and its own spelling could not express the example it was argued from.

```yaml
source:
  - downstream framework cache scope report 2026-08-09, against v0.4.8
  - decision:framework-integration-seams
  - decision:partial-transfer-seams
review_gate: proposed
round:
  when: 2026-08-09, the eighth round from this reporter
  previous: generation seams 2026-07-30, live integration 2026-07-31, component and asset seams 2026-07-31, runtime ownership 2026-08-01, composition seams 2026-08-02, caller-owned runtime 2026-08-04, partial transfer 2026-08-08
  reporter_position: three asks that only make sense together, one companion asking that an unspecified interaction be decided either way, and a migration preference with a stated floor
  outcome: all three accepted, in a form larger than the ask on the default and different from the ask on where the declaration may sit
verification:
  method: every claim read against v0.4.9, which is one tag ahead of the version the round names
  cache_key_is_id_plus_params: confirmed; cacheKey is KeyString(c.ID) + c.Key(params) and nothing else reaches it
  foldslots_unions_five_properties: confirmed at the cited position, unioning hasAwait, hasLive, head with headSources, assets, and vary
  chain_helpers_exist: confirmed; HasAwaitBlock and MergeVary take wrappers and a leaf, and ChainHead shows the same shape is already read before the first byte
  caller_holds_the_chain: confirmed; RenderChain takes wrappers and leaf from the caller, so a new chain accessor is readable at the same point the head already is
  call_graph_walk_is_affordable: confirmed; validateCachedComponents already calls two such walks, reachesAwait and reachesPerRequest, and a third reading another bit is the same shape
  head_sources_precedent: confirmed; Plan.HeadSources exists so a caller that cannot deliver a contribution can name the component to change, which is the situation the round cites
corrections_to_the_reporter_s_reading:
  the_ask_cannot_express_its_own_example:
    reported: a single declaration on an authenticated layout covers every page beneath it, which is the shape a login-gated application actually has
    actual: scope is proposed as an option on `@cache`, and decision:cache-component-declaration no_html_parameters refuses `@cache` on any component declaring an html parameter; a layout declares one by definition
    consequence: under the round's own spelling no wrapper can ever declare private, so IsPrivate over a chain would read false on every wrapper and the asked-for payoff is unreachable
    resolved: decision:cache-scope-declaration modes, which lets the annotation sit on a slot owner as a declaration that stores nothing
    reading: the round argued the chain union from an example its own syntax forbids, which is the kind of gap only reading the eligibility rules against the ask finds
  the_vary_companion_is_already_closed:
    reported: a component reaching a builtin element that reads a cookie is cacheable today, and the cookie does not enter its key
    actual: BuiltinElement perRequest is Provider != nil, an element reading a cookie needs a provider to read it, and validateCachedComponents refuses `@cache` on any component whose call graph reaches one, at the declaration position and naming the element
    tested: both the direct case and the call-graph case have tests
    so: the interaction is decided in the stronger of the two forms the round offered, a component depending on a request property being ineligible for `@cache`, keyed on Provider rather than on Vary
    what_actually_remains: an element declaring Vary with no Provider, whose markup is fully static and whose output therefore cannot vary; nothing validates that pair, and it is a registration mistake rather than a cache defect
    catalog_effect: decision:cache-component-declaration options.future stops calling vary unspecified
    reading: the round reasoned from the annotation surface rather than from what the generator refuses, and the refusal it wanted was already there under a name it was not looking for
what_the_owner_changed_relative_to_the_ask:
  default_moved_up_a_level:
    asked: private as the default of the scope option, so an unannotated component declares nothing
    decided: private as what an undeclared chain reports, so a login-gated application writes nothing at all
    why: the round's own failure-asymmetry argument does not stop at the annotation boundary, and an application that never writes `@cache` is exactly the one whose pages are least examined
  the_declaration_may_sit_on_a_layout:
    decided: `@cache` on a slot owner or shell declares scope and stores nothing, with ttl a generation error there and required on a component
    why_it_is_not_an_exception: every eligibility condition exists because bytes are stored, so removing the storage removes each premise
    what_it_buys: the one-annotation-per-application shape the round asked for, and the document shell as the place to write it
accepted_unchanged:
  - the scope value is opaque, supplied per render beside the store, and never interpreted here
  - a private component rendered with no scope value stores nothing rather than storing under an empty scope
  - the refusal of public over declared private is a generation error at the declaration position, not a warning and not a silent downgrade
  - undeclared does not block a public assertion, or nothing could ever be public
  - no header, no store change, no configuration switch, and no relaxation of the eligibility rules
migration:
  what_the_reporter_asked: require scope explicitly for one minor version, then default to private; or default to private immediately behind a major bump
  floor_stated: no silent default flip in a patch release, and no permanent required argument
  vehicle_correction: this module is at v0.4.9, so a minor bump is the breaking-change vehicle a major bump would be after 1.0; the reporter's second option maps to 0.5.0 rather than to 1.0.0
  decided: 2026-08-09; the second option, at v0.5.0, per decision:cache-scope-declaration migration
  reporter_preference_declined: the first option buys a deliberate per-call-site decision, which a module that is not yet a public framework does not need for a consumer set a release note reaches
  larger_under_the_owner_s_default: the flip reaches components carrying no annotation at all, so the release note has to name the header change and not only the key change
  cliff_is_not_compile_caught: every existing `@cache` becomes scoped, and a caller that passes no scope value stores nothing, which is the performance cliff the reporter preferred to trade for a compile error; the trade is being declined with the cliff known
  measurement_now_follows_the_flip: the hit-rate number the reporter offered was meant to decide between the two options, so it becomes a post-release observation rather than an input
what_the_reporter_can_contribute:
  consumer: the response path that needs IsPrivate, the identity feeding WithCacheScope, and a login-gated example application
  vocabulary: their own layered-cache policy already names private scope and states the rule this implements, so alignment is available rather than invention
  measurements: what the private default costs in hit rate on real pages, which decides the migration option
  their_own_gap_disclosed: MergeVary was never wired into their response path, so a declared vary axis reaches no Vary header on their side; disclosed unprompted, and it means the accessor has a caller now
```
