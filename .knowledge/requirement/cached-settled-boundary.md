---
id: requirement:cached-settled-boundary
type: requirement
title: Caching A Settled Await Boundary
---
Let a storing @cache cover a component that reaches an await boundary, storing the settled subtree, while a live boundary stays refused.

```yaml
priority: should
source:
  - downstream framework change request 2026-08-14, against v0.5.9 and v0.5.10
  - requirement:component-output-cache open question "caching a fully settled boundary set"
  - decision:cache-component-declaration await_rationale future
review_gate: proposed
motivating_case:
  what: a component takes a primary key, loads its own record through an await boundary, and one annotation covers the load and the render
  first_request: the fallback, so the page commits immediately and the record arrives when it arrives
  later_requests: the stored settled markup, with no wait at all
  owner_words: the effect on user experience is the reason to want it, not the saved CPU
  reached_by: requirement:template-value-binding made a self-loading component writable, and this is the limit the first one written runs into
criterion:
  rule: an await boundary over requirement:async-external-functions settles exactly once, so a settled form exists and there is something to store; an external live boundary keeps delivering after the document ends, so it never settles and no stored byte range could stand for it
  not_a_compromise: the distinction is the whole eligibility test rather than a line drawn to make the change smaller
  flags_already_express_it: HasLiveBlock is a subset of HasAwaitBlock, so the check narrows rather than gaining an analysis
  as_built_check: templates/htmlbind reachesAwait and reachesLive both exist and are already written; the refusal at validateCachedComponents swaps one for the other
what_already_works:
  finding: the runtime half of a hit exists and is reached today, verified against the working tree 2026-08-14
  await_op: awaitOp.Exec branches on a nil coordinator into execBlocking, which resolves the bindings, writes no fallback, and renders the primary subtree in place and contiguous
  cached_subtree: execCached builds its buffer renderer with no coordinator, so a cached subtree already takes that branch
  the_comment_says_so: the site records that generation rejects an await boundary inside a cached component, so dropping the coordinator "only makes the runtime's behavior match that rule instead of storing a placeholder"
  consequence: the settled contiguous form the request asks to store is what this path already produces, and the reporter's reading that the refusal is purely a generation check is correct
what_does_not_follow:
  the_gap: narrowing the refusal alone gives the hit and breaks the miss
  why: with the coordinator dropped, a miss blocks on the fetch and emits the settled subtree in place, so the first request waits and no fallback is ever written
  why_that_is_not_an_option: decision:cached-boundary-delivery; delivery is a property of the template and storage is a deployment question, so a miss that blocks would make the same template stream where no store is configured and wait where one is
  what_is_required_instead: the miss renders exactly as it does uncached, and the settled content is captured alongside for storage
  where_decided: decision:cached-boundary-delivery
semantics:
  invariant: decision:cached-boundary-delivery; a component without an await boundary renders inline and one with an await boundary emits its fallback, cached or not
  miss: delivery is exactly what the same component does uncached; the settled subtree is stored once every boundary has settled, which is after this request's response is already streaming
  hit: the await becomes synchronous — the settled markup is written in place, contiguously, with no placeholder, no fallback, and no completion frame
  what_a_hit_is_shaped_like: what the synchronous render entries already produce, since awaitOp settles in place whenever no coordinator is present
  asymmetry_is_the_point: the cached form is better than the boundary rather than equivalent to it, because a hit skips the wait, the client apply, and the render together
  failure: a boundary that fails stores nothing, which extends the existing rule that a failed render publishes nothing
  recover_output_is_not_a_hit: a settled recover subtree is a rendered failure rather than a rendered answer, so it is not the value the key stands for
unchanged_refusals:
  live: a component reaching a live boundary keeps the current generation error
  nested_reloadable: decision:cache-component-declaration no_nested_boundary is a separate rule with its own reason, requirement:component-output-cache opaque_unit, and narrowing the await rule does not narrow it
  html_parameters_shell_per_request: each keeps its own reason and is untouched
consistent_with_opaque_unit:
  question: whether a stored settled boundary set contradicts requirement:component-output-cache opaque_unit
  answer: no; a settled boundary set stored as one contiguous range is exactly one opaque unit, and nothing about it is decomposed internally
  why_it_matters: opaque_unit is what kept api:cache-store holding a byte slice, and this change gives it no reason to carry structure
flag_consequence:
  what: HasAwaitBlock is a plan constant read before rendering by requirement:render-mode-negotiation
  effect: a component that may or may not open a boundary makes the flag conservative rather than exact — the chain reports true, the streaming path is selected, and on a hit no boundary opens
  benign: a streamed response whose boundaries all resolve immediately is a complete document written through the streaming path, which is what a fast page already produces
  but: the flag stops promising that a boundary will open, so anything reading it for more than path selection has to be checked
  exact_on_a_miss: the boundary really opens there, so the flag is only ever wrong in the direction of promising more than happens
downstream_owned:
  stated_by: the reporter, before either side starts, so the division is on the record
  items:
    - render-mode selection, which reads HasAwaitBlock and must tolerate a hit opening no boundary
    - the render boundary trace span, which a hit makes disappear
    - the cache hit and miss counters, which stop distinguishing a markup hit from one that skipped a fetch
    - html.cache.max_entries, an entry count over entries that now vary hugely in size
    - where a newly possible synchronous external failure is rendered
  the_one_that_costs: the entry cap; a count chosen when an entry was a markup fragment does not describe a store whose entries hold fetched records
  why_it_reaches_us: it may change what the entry should look like, which is this module's; it connects to requirement:component-output-cache open questions on what a rich entry costs
why_not_downstream:
  refusal: applied where the annotation is compiled, and no framework option reaches it
  lookup: inside plan execution; the store arrives as an option and the plan decides when to consult it, so there is no seam at which a caller could substitute bytes for a boundary
  bytes: the settled subtree exists only inside the boundary machinery; what reaches a response writer is a shell holding a placeholder plus completion frames addressed by boundary id
  verified: all three hold against the working tree, so the reporter's claim is not taken on assertion
acceptance:
  - a storing @cache on a component reaching an await boundary generates, and on a live boundary still fails with the current diagnostic
  - a miss delivers the placeholder, the streamed fallback, and the completion frame exactly as the same component does uncached
  - the same template delivers the same way whether or not a store is supplied, so configuring one changes what is stored and never what is sent
  - a hit writes one contiguous range with no boundary opened, no fallback, and no completion frame
  - a boundary that fails stores nothing, and the next request is a miss
  - a component reaching a nested reloadable component still fails generation
  - a cached component with no await boundary generates exactly the bytes it generates today
open_questions:
  - what a stored entry should be measured in once it holds fetched records rather than markup; safe to answer after building, because the entry stays a byte slice and a store can already bound itself by len, so only htmlbind MemoryCache capping by entry count would want an additive byte-bounded sibling
  - whether a hit should be distinguishable from a markup-only hit at the api:cache-store seam; worth asking the reporter what they actually need first, since decision:cache-key-derivation frames the component identity into the key in plaintext and a wrapping store may already be able to tell
closed_questions:
  suspense_html_streaming: nothing there needs changing; requirement:suspense-html-streaming never mentioned a settled boundary set, and the deferral lived in requirement:component-output-cache and decision:cache-component-declaration, both now updated
related:
  - requirement:component-output-cache
  - decision:cache-component-declaration
  - requirement:async-external-functions
  - decision:async-boundary-syntax
```
