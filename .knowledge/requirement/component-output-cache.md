---
id: requirement:component-output-cache
type: requirement
title: Component Output Cache
---
Reuse explicitly cache-enabled component output for equivalent typed inputs until its TTL expires.

```yaml
source: concept:html-render-runtime-extensions
declaration: decision:cache-component-declaration
identity: decision:cache-key-derivation
store: api:cache-store
value: validated rendered HTML bytes for one component invocation
behavior:
  hit: write cached bytes through the caller's current response stream
  miss: render into an isolated buffer, publish only after the whole subtree renders, then reuse until expiry
  expiry: expired entries behave as misses
  no_store: without a supplied store the component renders normally, so caching is a deployment choice rather than a template rewrite
streaming:
  restriction: a cached component owns no decision:async-boundary-syntax boundary, so its output is one contiguous byte range
  buffering: only the cached subtree is buffered; the rest of the document keeps streaming
partial_update:
  boundary: independent from requirement:partial-update-boundaries; cache and client update flags may be enabled separately
  reuse: cached content validator may satisfy requirement:component-delta-rendering without rerendering
safety:
  - never cache errors or partial output
  - preserve rule:template-context-safety at insertion; cached bytes were context-checked when produced
  - cache only output whose complete dependency set is represented by the key
  - per-user variation is covered by the decision:cache-scope-declaration prefix; locale and any other axis the scope value does not cover must be an explicit parameter or the component must not be cached
  - a request property a builtin element declares makes the component ineligible rather than mis-keyed
concurrency:
  misses: concurrent misses each render; the runtime does not coalesce, so a store that wants single-flight implements it
  reason: coalescing inside the runtime would tie one request's cancellation to another request's render
compatibility: requirement:html-rendering-compatibility
acceptance:
  - identical inputs within TTL can avoid component logic and rendering
  - changed input, changed generated plan, or expired TTL cannot reuse the old value
  - a render with no store behaves exactly like the same template without the annotation
structured_output:
  layering: this cache reuses execution keyed on inputs; requirement:structured-render-output reuses transfer keyed on the template, so the two are complementary rather than alternatives
  collision: the stored value is assembled bytes, so a cached subtree on a structured path either loses the split or the store holds the structured form instead
  raised: downstream framework partial transfer report 2026-08-08, which prefers the structured form and says it is cheap to design for and awkward to retrofit
  forced_by_the_unit_set: requirement:structured-render-output makes an @cache component a unit, and a hit does not execute, so a store holding bytes alone loses the split for exactly the components declared most reusable
opaque_unit:
  decided: 2026-08-08 by the owner; a cached component's output is one opaque unit and is never decomposed internally
  general_argument: the annotation already declares this output reusable as a unit, so separating its statics from its variables contradicts its own declaration
  the_case_that_makes_it_obvious: a rendered markdown body arrives from an external function as one trusted string, so it has no template structure to separate, and recording spans inside it would find exactly one
  application_too: a client installs the unit whole, with no slot mapping to carry
  where_it_still_participates: it is a requirement:boundary-decomposed-render hole in its parent, so the parent carries a placeholder and the parent's own render computes the decomposition around it
  api_consequence:
    decided: api:cache-store keeps its byte slice; no typed entry, no span offsets, no static sequence address
    withdraws: an interface change proposed earlier in the same round on the assumption that a cached component would decompose internally
    reasoning_that_failed: a store holding bytes alone was said to lose the decomposition, and there is no decomposition inside to lose
  head_excluded: head contributions live on Plan.Head and merge before the first body byte; decision:cache-component-declaration forbids html parameters, so a cached component's head is fully static and nothing about it needs storing
  omitting_an_unchanged_cached_fragment:
    mechanism: the existing validator path; the parent's boundary list names the hole and the client returns the validator it holds, so an unchanged cached fragment is a hole with no fragment
    no_new_identity: requirement:component-delta-rendering already owns this comparison, so nothing about the cache has to reach the client
    never_send_the_cache_key: decision:cache-key-derivation frames every declared parameter in plaintext and hashes nothing in the runtime, so a cache key carries parameter values and must never reach a browser
    recorded_because: sending the key looks like a free way to let a client hold a cached fragment, and it would publish whatever the parameters are
open_questions:
  - stale-while-revalidate and explicit invalidation
  - caching a fully settled boundary set, which requirement:suspense-html-streaming currently excludes
  - what a rich entry costs the document path in practice, which is the one number that should be measured before the interface changes
downstream_experience_2026_08_13:
  status: input, not a request; offered against the two open questions by a reporter who built both at its own data layer, per decision:cache-key-generator-seams
  why_it_transfers_only_partly: a duplicate fetch costs an upstream call where a duplicate render costs local CPU, so the same mechanism carries a different cost-benefit
  stale_window_changes_the_entry_not_the_policy:
    finding: one TTL cannot express it; the entry needs a fresh deadline and a last-answerable deadline, and a read between them returns the held value and starts one revalidation
    consequence_for_this_module: it is an api:cache-store interface change, not a policy setting; the reporter did not retrofit it onto bytes plus one duration and built a superset instead
    weighs_against: the unchanged_2026_08_08 decision above, which kept the byte slice for a different reason and would have to be reopened on this one
  tag_invalidation_needs_a_reverse_index:
    prefix_axis_is_free: prepending the scope makes scope deletion a range rather than a scan, and the reporter reports it paid off as decision:cache-key-derivation scope_prefix predicted
    other_axis_is_not: with the scope first, every entry of one key type across all readers is not a prefix
    consequence: a tag index answers it and does not fit Get/Set
  coalescing_objection_answered_not_overruled:
    ours: coalescing inside the runtime would tie one request's cancellation to another request's work
    their_answer: the shared fetch runs on a context with cancellation removed and values kept, so a waiter that goes away stops waiting and stops nothing else
    cost_it_creates: detaching removes the only bound on the fetch, which is why their store also carries a fetch timeout
    their_own_verdict: the mechanism transfers, the cost-benefit does not, and the no-coalescing default is still right for a render
  fingerprint_has_no_hand_written_equivalent: a Go function compiles to nothing to digest, which is why requirement:cache-key-generation is the blocking ask of that round rather than a convenience
self_loading_component_2026_08_14:
  wanted: a component declaring a primary key as its parameter, loading its own data, and carrying one annotation over the load and the render together
  today: this cache saves the rendering and the reporter's own data cache saves the fetching, so one slow page is configured in two places for what an author thinks of as one thing
  already_covered_here: the load would sit inside the cached subtree and decision:cache-key-derivation keys on declared parameters, so a hit skips the loader with no change to this requirement
  blocked_on: proposed requirement:template-value-binding, because a component cannot name a fetched value today and calls its loader once per field it renders
  still_open: a staleness policy over fetched data, which is the stale-while-revalidate question above rather than a new one
  not_requested_yet: the reporter asks for the binding alone and says the key routing for the combined form is unresolved on their side
```
