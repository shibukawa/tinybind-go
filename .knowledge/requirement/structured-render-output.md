---
id: requirement:structured-render-output
type: requirement
title: Structured Render Output
---
Yield a render as each template unit's static runs and its dynamic values separately, with a slot kind per dynamic and an identity per unit, instead of the assembled bytes that are all a caller can obtain today.

```yaml
priority: should
status:
  deferred: 2026-08-08, after the analysis below and by the owner's scoping decision
  ships_instead: requirement:boundary-decomposed-render, which decomposes a render at boundaries and leaves each fragment assembled
  what_that_reaches: the transfer half at boundary granularity, and none of the application half
  why_defer_rather_than_drop: every hard part recorded here comes from dividing the inside of a dynamic region, and that cost is only worth paying once a measured decomposition shows what it does not cover
  keeps: this concept's verification, its corrections to the reporter's sketch, and every decision reached on it, so a later round starts from settled ground rather than re-deriving it
  staged_as: decision:dom-application-strategy stage 3, with the decomposition sitting between stage 1 and it
  decisions_that_moved: entry_shape and skeleton_delivery below apply to what ships and are restated in requirement:boundary-decomposed-render as entry_shape and static_sequence_delivery; they are left here because they were reasoned out here, and the shipping concept is where they are in force
  what_is_actually_left_here: the ability to apply a value without reparsing, which is the only part the decomposed design does not reach; every difficulty this concept records belongs to that part alone
source:
  - downstream framework partial transfer report 2026-08-08, against v0.4.2
  - decision:dom-application-strategy staging 3
  - decision:generated-render-plan
review_gate: proposed
names_stage_three:
  what: decision:dom-application-strategy already chose this as the end of its staging and called it a second rendering backend emitting values instead of bytes
  changed: it was an application strategy with no module-side output behind it; this is that output, asked for by the caller who would consume it
  reading: the catalog designed the client half and left the server half unnamed, which is why a reader of that concept concludes the split is undesigned
problem:
  separation_exists_and_does_not_escape: a plan's Static ops are compile-time constants and Text, Attr, and their variants hold functions, and execOps concatenates both into one byte stream before any caller sees output
  suppression_has_no_answer_for_a_changed_region: a region that ticks every second changes every second and pays for its whole subtree each time, whatever a digest suppresses
  the_multiplier: htmlbind appendJSONString escapes '<', '>', and '&' as six-byte sequences, so markup costs roughly triple inside a delivery record while content costs nothing extra; a row carrying twelve angle brackets spends about a hundred bytes on structure before a character of data
  application_half: a delivery applied by setting innerHTML reparses the subtree every time, which is why form state, focus, selection, and preserved islands have to be carried across the swap
what_the_caller_shipped_first:
  done_downstream: a keyed digest per delivered boundary, returned on reconnect, so a boundary whose digest still matches is not written at all
  removes: the retransfer of settled regions on every lifetime rollover and every dropped connection
  does_not_remove: the cost of the regions that did change, which is what this asks for
  reading: the reporter absorbed the half it could before asking, so the ask is what is left rather than what was not tried
ask:
  statics: the static runs of each template unit
  dynamics: the values in plan order, unassembled
  kinds: rule:dynamic-slot-kinds, so a client applies a value without deciding how to escape it
  identity: a stable identity per unit, so a client caches a skeleton and a server sends it once
  nesting: a Component op yields a child unit, a For yields one skeleton with one value set per item, an If yields the branch it took
verified_2026_08_08:
  method: every claim read against v0.4.2 and two of them measured by rendering
  split_is_inside: confirmed; Op is one Exec method, staticOp is a string, textOp holds a function, both unexported, and execOps writes them into one stream
  for_is_already_a_shared_skeleton: emitForOp generates a scope struct and its own Builder per loop, so a loop body is already a separate typed op list executed once per item; the skeleton-plus-value-sets shape is what generation emits rather than one it must invent
  if_is_already_a_unit: If carries then and otherwise as separate op lists
  text_is_already_available: a Text closure returns the value unescaped and textOp escapes at Exec, so the text kind is data today
what_the_sketch_does_not_survive:
  statics_are_not_the_runs_between_values:
    measured: a generated plan emits Static(" <article"), then BoolAttr, then Attr, then Static("> <a")
    consequence: a static run ends mid-tag, and each attribute op carries its own ' name="' prefix and closing quote as op state rather than as a Static
    so: the reporter's flat static array cannot be read off the plan; producing it is emitter work, which the reporter says and this measures
  an_optional_attribute_is_structure:
    what: Attr returns (string, bool) and an absent value omits the whole attribute, its name, and its quotes
    consequence: a unit holding one is not a fixed skeleton with a hole, because presence changes the static runs themselves
    not_covered: the reporter's boolean row covers BoolAttr and its attribute row assumes a value always exists
  an_attribute_value_is_already_assembled_and_escaped:
    what: the value closure concatenates author literals with escaped expressions, because only the expressions may be escaped, and returns one string
    consequence: a raw attribute value does not exist at runtime; the emitter has to stop escaping inside the closure and let the op do it
    precedent: URLAttr already made exactly this move, for the structural reason decision:url-context-escaper records, and its value closure returns unescaped assembled text
    reading: the precedent is what makes this tractable and also what sizes it, since it is one op today and every attribute op after this
  a_child_unit_is_opaque:
    what: Component binds to a Fragment carrying a render closure, and a Fragment holds no plan pointer and no identity
    consequence: nesting needs Bind to carry what it drops today, so the parent op can name the child unit at all
  module_owned_insertions:
    what: BoundaryAttr, CSRFField, and MergedHead write from render state rather than from parameters
    consequence: they are neither static across renders nor a parameter-derived dynamic, and rule:dynamic-slot-kinds gives them their own kind
identity:
  asked: one identity, not a third, so a client's skeleton cache, a server's output cache, and a boundary validator invalidate together on a regeneration
  agreed_on_the_rule: decision:cache-key-derivation version_rationale is exactly the property a skeleton cache key needs, for exactly the reason it was adopted there
  finding: neither existing identity is emitted for an ordinary component
  detail:
    cache_policy_id: present only on a component carrying a cache annotation
    boundary_component_id: present only on a component that can be a chain member
    plan: has no identity field at all
  consequence: this generates a unit identity for every component under the rule decision:cache-key-derivation already states, rather than reusing an identity already emitted; the ask for one rule holds and the ask for zero new emission does not
  reading: a message row, which is the reporter's own example, is exactly a component with neither identity today
  one_rule_several_addresses:
    corrected: 2026-08-08 by the reporter, against the first wording of this concept
    was: one identity, which read literally says the three values coincide
    is: one derivation rule producing several related addresses, since decision:cache-key-derivation is scoped per component while a skeleton address is per emitted skeleton, and a component with a conditional or an optional attribute has several
    property_that_was_actually_wanted: a template edit invalidates all of them together, and there is one rule to explain rather than three; both hold either way
    why_the_wording_matters: the stronger reading is implementable against and false, which is the kind of claim a specification exists to prevent
settled_2026_08_08:
  by: owner, on the proposal round, and accepted by the reporter in reply the same day
  unit_is_a_component:
    decided: a unit is a component call, never a loop body, a conditional branch, or an arbitrary region
    buys: the identity rule is already component-scoped, and Component becomes the only nesting op, so the opaque-child-unit problem is solved once instead of three times
    asks_of_the_author: a loop body that should be a shared skeleton is written as a component call
    accepted_downstream: the reporter takes it as an improvement rather than a tax, on the ground that an inline row hides whether it is a unit and a named one states it
    costs_here: being a unit becomes a fifth phase-dependent capability in decision:generated-render-plan inlining, so which components are units has to be decided rather than defaulted to all
  identity_is_a_content_address:
    decided: the address is a hash of the emitted skeleton, not of the component declaration
    dissolves: a conditional changes structure rather than values, so each branch is its own address; an optional attribute changes the static runs, so present and absent are two addresses
    bounded: at most one address per distinct emitted skeleton of a component, each small and each sent once per connection
    was_already_written: decision:dom-application-strategy skeleton_distribution says a content address is immutable, permanently cacheable, and invalidated by a deploy; this takes that sentence literally
  not_the_update_boundary:
    decided: a unit and a requirement:partial-update-boundaries boundary stay separate declarations; a component may be either, both, or neither
    reason: a boundary is instance-scoped state the client returns on every request, so its cost is linear in instance count, while a skeleton is template-scoped and sent once per connection, so its cost is flat
    the_case_that_decides_it: a five-hundred-row list needs the row to be a unit and needs it not to be a boundary; decision:manifest-state-ownership size_mitigations already refuses per-row boundaries for the linear cost
  unit_set:
    decided: a component is a unit when it is declared @cache, declared reloadable, called as the root of a for body, or a chain member — meaning a generated route page or layout
    declared: the first two; the author already writes them and neither is added for this
    derived: the last two, computed over the call graph exactly as HasAwaitBlock, HasLiveBlock, Assets, and Vary already are, so no annotation is added
    not_exported: visibility is not the gate; requirement:partial-update-boundaries exported_only already records what forcing an export costs, and repeating it here for a skeleton would be the same mistake in a second place
    live_needs_nothing: requirement:live-boundary-rendering makes a live boundary a partial-update boundary already, so the highest-frequency case is covered without a marker of its own
    inside_a_unit_stays_inlined: a component reached from within a unit folds into that unit's skeleton, which is cheaper than its own address because the skeleton travels once either way
    identity_coverage:
      already_emitted: '@cache carries CachePolicy.ID; reloadable and chain members carry Boundary.ComponentID'
      new: the loop-top component alone, which is also where the measured win is concentrated
      reading: three of the four kinds need no new identity emission, which is most of what the one_rule_several_addresses concern was about
    why_not_everything_repeating: a component rendered once with no repetition costs more as a unit than as assembled bytes, since skeleton plus values plus envelope exceeds the interleaved form; a loop wins on the first render already, because the skeleton is shared between rows within one render
    inline_loop_body:
      consequence: a for body written as inline markup produces no unit, so its rows share no skeleton
      diagnostic: undecided, and folded into decision:list-item-key, which asks the same question about an unkeyed loop in an updatable region
  first_build:
    question: an element description is exact to construct and slow, where one innerHTML is one parse, and the first build is what a user waits for
    decided: the skeleton is the element description alone, and the client assembles the string, inserts its own markers, and parses once
    rejected: a skeleton carrying an assembled string form plus slot addresses within it
    decisive_reason: a post-parse address is not expressible by this module, because the tree is what the browser's parser decides — an implied tbody, content foster-parented out of a table, a p closed early — so any address computed here would be a prediction of parser behaviour
    why_the_client_can: it is the party assembling, so it may insert markers it then finds, and it never needs this module to predict anything
    loop_consequence: a row is assembled and parsed once and cloned per item, which is one parse for a hundred rows
    emitter_reading: an element description transcribes the op list; an assembled form with holes is not expressible for an absent attribute and would need a marker language to be defined here and honoured there
measurements_2026_08_08:
  reported_by: the reporter, on a reproduced Room panel — a title and a for over messages emitting a strong, a text run, and a small per row
  event: one message arriving, which shifts the list by one
  result: 743 to 262 bytes at 5 rows, 4039 to 1258 at 30, 13280 to 4059 at 100; roughly threefold and flat in row count, because values grow with rows too
  shape_sensitivity: markup carrying classes, ARIA, or SVG does better; a panel that is mostly one long text value does worse
  kills_the_alternative: a downstream byte diff of two consecutive renders saves 1 to 3 percent on this event, because a prepended row shifts every subsequent byte; the argument against building it was principled and the measurement is stronger than the argument
  status: a computed projection on a reproduced shape rather than production traffic, and the projection assumes a record shape neither side has settled
escaping: rule:dynamic-slot-kinds owns what travels and what stays; the URL scheme policy stays here, per requirement:url-attribute-scheme-safety
cache_interaction:
  today: requirement:component-output-cache stores assembled bytes for one component invocation
  choice: a cached subtree either loses the split or the store holds the structured form
  preference: the reporter asks for the structured form, and it is cheap to design for and awkward to retrofit
  layering_is_agreed: the output cache reuses execution keyed on inputs and this reuses transfer keyed on the template, so the two are complementary rather than alternatives
relation_to_the_plan_slice:
  separate: requirement:live-mode-plan-slice is not a dependency in either direction and this request does not ask for it
  shared_home: both are a generated plan executed or emitted in part rather than in whole, so they belong in the same body of work
what_is_not_asked_for:
  - a wire format, a record shape, or a protocol version, which decision:caller-owned-wire-versioning puts on the caller
  - a browser runtime, settled by decision:client-runtime-ownership
  - changes to boundary identity, the validators, the manifest codec, or the delta operation kinds
compatibility:
  additive: a structured entry sits beside the byte-writing ones and Plan.Exec keeps its current path
  kinds: data on the new output; nothing existing carries them
  release: the reporter expects it behind a v0.5 rather than a patch, because it touches the emitter
acceptance:
  - one render yields per unit its static runs, its dynamic values in plan order, and a kind per dynamic
  - a loop yields one skeleton and one value set per item, not one skeleton per item
  - a conditional yields the branch it took as its own unit
  - a unit identity changes when the template changes, by the rule decision:cache-key-derivation already applies to a cache key
  - a client applying values reproduces the bytes the assembled path would have written, for every kind
  - an optional attribute that becomes absent resolves to a second skeleton address rather than to an empty value, and the client holds both after seeing both once
  - a component with an unsafe form or a URL attribute yields the same output through both paths, so the structured path is not a way around a check
entry_shape:
  decided: 2026-08-08, the structured entry yields units as an iterator sequence rather than collecting them
  matches: the RenderAsync entry of api:render-html-chain, so the structured path has the same two-entry shape the byte path has
  forced_by: a unit may contain an await boundary — only an @cache unit is guaranteed not to, by decision:cache-component-declaration eligibility — so the units of one render do not all exist at one moment
  what_collecting_would_cost: requirement:suspense-html-streaming, since nothing could be sent until the slowest boundary settled
  keeps_open: the richer answer to a boundary inside a unit, where the boundary is a slot whose completion arrives later as its own unit; collecting would have forced the poorer one
skeleton_delivery:
  unit_of_delivery: one skeleton per unit component per distinct emitted shape, not per delivery, per chunk, or per row; a list of five hundred rows has the same two skeletons a list of five has
  scale: typically five to thirty per page, a few kilobytes in total, and immutable until a template changes
  inline:
    cost: sent once per connection, which means every skeleton again on every page load, every lifetime rollover, and every reconnect
    reads_on: the reporter's own workload, where a rollover runs roughly every ten minutes per client, so once per connection is the pattern its digest suppression was built to remove
  fetched_by_address:
    cost: one batched request for the addresses a client does not hold, and zero once it holds them
    caching: the browser's own cache keeps them under immutable, across page loads and sessions
  the_argument_that_decides_it:
    what: a skeleton is the only thing on this wire that is not per user, being derived from the template rather than from a request
    consequence: it is the only response that could be public and immutable, and therefore shared or served from an edge
    inline_forfeits_it: riding inside a private response makes it unshareable, which is the one property it uniquely has
  decided: 2026-08-08, fetched by address
  three_lifetimes:
    reading: a skeleton, a request, and a subscription have different lifetimes, different cache policies, and different termination rules, which is why each is its own request
    per_template: the skeleton — public and immutable, one response, valid until a template changes
    per_request: the document or the delta — private, one chunked response, an initial frame then later fills, ended by rule:stream-termination-marker
    per_subscription: the live mode — private, chunked, open for the subscription's lifetime and resumable
    the_argument: riding a skeleton on a request response makes it inherit that response's cache policy and lifetime, both of which are wrong for it
    what_it_protects: keeping the first and third out is what lets the middle one stay one shape; requirement:streaming-delta-response already says an await completion inside a delta yields the record shape a document completion does, so one client runtime consumes both
  no_waterfall:
    concern: a response naming an unheld address would cost a round trip before anything could be applied
    why_it_does_not: the first delivery carries assembled markup, so a client applies immediately and fetches skeletons behind it; the structured form starts from the second delivery
    which_is: the two-phase shape the reporter described for itself, where the delivery after a page load lands the old way and every one after it is direct
    closing: fetching works because the assembled form covers the cold case, so the fetch is warm-up rather than a precondition
  cache_policy: public and immutable, keyed by the content address, so a deploy invalidates by producing new addresses and no explicit invalidation exists
  constraint:
    rule: a skeleton request is a render mode on a URL the caller chooses, never a path this package mounts
    precedent: requirement:caller-addressed-redraw removed the last mounted registry route four days earlier, so adding one back would restore what that round deleted
    weaker_here: authorization is not the driver, since a skeleton is public; the caller owning its own addresses is
  live_does_not_join_the_response:
    what_rides_along: a live boundary's first delivery, as an await completion, per decision:live-transport-boundary first_delivery_inline
    what_does_not: every delivery after it, which decision:live-transport-boundary rejected_endless_document_response refuses on four grounds — a document that never completes loading, no resume after a drop, a later-inserted boundary that cannot join, and a connection pinned for the session
open_questions:
  - whether decision:list-item-key participates in the encoding or only in application, which that concept already asks and this makes answerable
  - what the emitter cost is in generated file size, given every attribute op grows a static frame it did not carry
  - what an await or live boundary inside a unit yields, since a boundary writes a placeholder now and content later; decision:cache-component-declaration rejects the same shape at generation time for the same reason
related:
  - decision:dom-application-strategy
  - decision:generated-render-plan
  - rule:dynamic-slot-kinds
  - decision:cache-key-derivation
  - requirement:component-output-cache
  - requirement:live-mode-plan-slice
  - decision:list-item-key
  - decision:partial-transfer-seams
```
