---
id: decision:partial-transfer-seams
type: decision
title: Partial Transfer Requests From The Downstream Framework
---
Accept the seventh downstream round, and record that its small item is the defect and its large item is the one this catalog designed as a client strategy and never named a server output for.

```yaml
source:
  - downstream framework partial transfer report 2026-08-08, against v0.4.2
  - decision:framework-integration-seams
  - decision:update-composition-seams
review_gate: proposed
round:
  when: 2026-08-08, the seventh round from this reporter
  previous: generation seams 2026-07-30, live integration 2026-07-31, component and asset seams 2026-07-31, runtime ownership 2026-08-01, composition seams 2026-08-02, caller-owned runtime 2026-08-04
  reporter_position: not blocked; one large ask expected behind a v0.5, one small gap with no workaround, and one scheduling request against a requirement already designed here
  shipped_before_asking: the reporter implemented keyed digest suppression per delivered boundary first, so an unchanged region is no longer transferred on a rollover or a reconnect; the ask is what suppression cannot reach
  reading: a round that arrives after the reporter absorbed the half it could is a different document from one that arrives instead of trying
verification:
  method: every claim read against v0.4.2, and the two that could be measured were rendered rather than reasoned about
  split_is_consumed_inside: confirmed; Op is one Exec method, staticOp is a string, textOp holds a function, both are unexported, and execOps concatenates before any caller sees output
  identity_ask: confirmed and narrower than the reporter assumed; see structured_output_is_larger_than_its_sketch
  redraw_takes_no_options: confirmed, and it is not only the cache; see the_small_item_is_the_defect
  json_escaping_multiplier: confirmed; appendJSONString writes '<', '>', and '&' as six-byte escapes, so the reporter's arithmetic on markup cost inside a delivery record is right
  cache_stores_assembled_bytes: confirmed; execCached renders into a buffer and stores the bytes, which the round first read as a gap and then settled as the right shape, since requirement:component-output-cache opaque_unit makes a cached output one unit with nothing inside it to report
corrections_to_the_reporter_s_reading:
  the_action_path_has_the_same_gap:
    reported: every other path we serve — document, navigation delta, action — reaches htmlbind through an entry that takes options
    actual: WriteUpdate and WriteUpdateStatus call htmlbind.Render with no options, exactly as the redraw path does
    so: the ask is two entries rather than one, and the action path carries the worse instance of it
  the_cited_example_is_not_in_this_repository:
    reported: the shape in your own examples/live_render
    actual: examples holds demo, and the live template shape is testdata/templates/htmlbind/live
    consequence: none for the argument, which the reporter's own message-row shape carries; recorded so the next reader is not sent to a path that does not exist
  the_sketch_is_a_target_and_not_a_reading_of_the_plan: the statics the sketch shows cannot be read off a plan; see below
reply_2026_08_08:
  what: the reporter read the proposal back against v0.4.2, rendered what could be rendered, and answered the three questions
  every_proposal_claim_holds: the two measured ones reproduce character for character, and the reporter measured two this round had only stated — an absent optional attribute leaves nothing behind, and a Text closure returns the value unescaped
  two_reporter_claims_withdrawn:
    action_path_takes_options: withdrawn; the reporter's own wrapper passes none either, and reproducing the failure against it is what moved the item
    example_attribution: examples/live_render is the reporter's own, not this repository's
  one_defect_found_in_this_round_s_own_quotation:
    what: the op-list excerpt in the proposal drops BoundaryAttr, which sits between the first Static and the first BoolAttr
    consequence: it is a live instance of the module_owned kind rule:dynamic-slot-kinds adds, interleaved exactly where a skeleton has to describe it, so the omission removed the evidence for one of the proposal's own additions
  identity_wording_corrected: the reporter asked for 'one rule, several addresses' rather than 'one identity', because the stronger reading is implementable against and false; requirement:structured-render-output carries the correction
  measurements_supplied: roughly threefold on a text-heavy list panel and flat in row count, with a byte diff saving 1 to 3 percent on the ordinary event; requirement:structured-render-output measurements_2026_08_08 holds them
  sequencing_accepted: the reporter agrees value was the wrong axis for the first two items, having found what the first one costs
  reading: the round closed with claims withdrawn on both sides, which is what a specification round is for and what neither side's own implementation could have produced alone
structured_output_is_larger_than_its_sketch:
  agreed: the separation exists at generation time, is destroyed at execution time, and cannot be recovered downstream — the reporter considered diffing two renders and rejected it for yielding no slot kinds, which is right
  but_the_statics_are_not_there_to_expose:
    measured: a generated plan emits Static(" <article"), BoolAttr, Attr, Static("> <a"), so a static run ends mid-tag and each attribute op holds its own ' name="' and closing quote
    and: an Attr value closure has already concatenated author literals with escaped expressions, so a raw attribute value does not exist at runtime
    so: this is emitter work producing a shape, not runtime work publishing one; the reporter says it touches the emitter and the measurement says how far
  what_is_already_the_right_shape: a For body is a separate typed op list under its own scope struct, and an If branch is its own op list, so the shared-skeleton-plus-value-sets form is what generation already emits
  the_identity_ask_half_holds:
    asked: name the units with an identity that already exists, so a skeleton cache, an output cache, and a boundary validator invalidate together
    holds: the rule, which decision:cache-key-derivation states and which is right for a skeleton cache for the same reason it was right for a cache key
    does_not_hold: the assumption that such an identity is already emitted; CachePolicy.ID exists only on a cached component, Boundary.ComponentID only on a chain member, and Plan carries no identity field, so an ordinary component — the reporter's own message row — has none
    consequence: one rule, new emission
accepted:
  - what: requirement:structured-render-output
    value: highest; it is the only item that changes what a delivery costs for a region that actually changed, and it is the one the caller cannot build
    cost: highest in the catalog's current open set; it touches the emitter, every attribute op, and the binder, and it needs a unit identity emitted where none is today
    already_ours: decision:dom-application-strategy chose this as its stage 3 and named no server output for it, so the round asks for a half this catalog left unnamed rather than for something new
    carries: rule:dynamic-slot-kinds, which is the part that decides whether the split is safe
  - what: requirement:fragment-render-options
    value: high, and higher than the reporter placed it; the missing options are not a lost optimization but a component class that cannot render
    cost: low; a variadic on two signatures, source-compatible, with today's behaviour as the default
    raised: from the reporter's small to a must, on the measurement rather than on the report
  - what: requirement:partial-update-boundaries explicit activation
    verdict: already designed here; the ask is priority rather than shape, which the reporter states plainly
    why_it_moves: it is the one of the three that is designed and unimplemented, and it decides what is sent while the structured output decides what each one costs
    unchanged: the syntax stays this project's question, which the reporter declines to propose
the_small_item_is_the_defect:
  finding: the two option-free entries do not merely miss the cache; they miss every render option, and two of the absences are wrong rather than merely default
  csrf: a component containing an unsafe form fails to render at all on both paths, which for WriteUpdateStatus is its own documented headline case
  url_policy: a caller's configured scheme allowlist does not reach either path, so a component renders one way in its page and another in the response replacing it
  reading: the reporter ranked this last and called it small because the cache was what it hit; the entries were never given options at all, and what that costs was never enumerated
severity:
  incident: none; nothing here puts wrong content on screen or leaks anything, and the URL divergence is stricter rather than looser
  defect: the option-free entries, on two counts
  missing_capability: the structured output
  scheduling: the update flag
sequencing:
  chosen: the render options, then the update flag, then the structured output
  why_options_first: it is a defect, it is a variadic on two signatures, it needs no design round, and one of its two failures makes a documented use case unusable
  why_the_flag_second: it is designed, it is opt-in by construction, and it is what makes a delta's granularity a choice at all; without it a five-hundred-row table transfers as one page boundary whatever the encoding is
  why_the_structured_output_last_in_order_and_first_in_value: it is the ranking item and the agreement with the reporter is on that point, but it wants the identity question settled, it belongs with the generated plan work, and it is a v0.5 by the reporter's own expectation
  not_a_disagreement: the reporter sequenced by value and put the structured output first; this agrees on value and puts a defect and an opt-in flag ahead of it because neither blocks on a design round
marker_round_2026_08_08:
  what: the design round that followed the reply, settling which components are units and what marks them
  settled:
    unit_set: '@cache, reloadable, a for-body root, and a chain member; the first two declared and the last two derived over the call graph'
    no_new_marker: reloadable becomes the explicit partial-update activation, so requirement:partial-update-boundaries needs no flag of its own and the annotation list stays at @cache
    modifier_versus_annotation: decision:template-annotation-syntax gains the line it was missing — a modifier says what a declaration is, an annotation configures how rendering behaves
    cache_entry: api:cache-store keeps its byte slice; a typed entry was proposed and withdrawn once requirement:component-output-cache opaque_unit settled that a cached output is never decomposed internally
  disagreement_recorded: the reporter asked that reloadable not imply the boundary, and the owner decided against it on the ground that a reloadable component is deliberately registered, so the boundary population is author-chosen; requirement:partial-update-boundaries downstream_dissent carries it
  what_it_does_to_ask_3: it stops being a flag to design and becomes a capability to add to a shipped declaration, which is materially cheaper than the round asked for
scoping_2026_08_08:
  decided: this round ships requirement:boundary-decomposed-render, and requirement:structured-render-output is deferred with its analysis intact
  what_ships: a fragment per boundary plus the tree composing them, with a placeholder at every nested boundary and a statically-known boundary never transferred again under one build
  what_defers: the split inside a dynamic region, which is where every hard part of the round lived — mixed attribute values, optional attributes, slot kinds, content-addressed skeletons, and client-side assembly
  answers_the_reporter_partly:
    transfer_half: answered at boundary granularity
    application_half: not answered; the reporter ranked it first and named it the one a caller cannot approximate, and every fragment here is still installed by parsing
    measurement_does_not_carry: the threefold figure was measured on a message row with no static subtree of any size, so it is not evidence for what this ships
    must_be_said_plainly: the reply has to state that this is the transfer half and that the application half is deferred, rather than presenting a smaller deliverable as the whole answer
  why_it_is_a_defensible_scope: the decomposition is designed already as data:component-delta-response retain_holes, it costs almost nothing on the wire, and a measured decomposition is what should decide whether the inside-a-region split is worth its emitter cost
  second_disagreement_in_this_round: after reloadable implying the boundary, this is the second place the two catalogs will differ, and both belong in the reply
principle:
  applies: the decision:framework-integration-seams rule, widen a seam whose default output stays identical and whose contract stays the caller's
  fits: the options variadic exactly, and the structured output as an entry beside the byte-writing ones
  new_reading: an item a reporter calls small deserves the same verification as one it calls large, because the report ranks by what the reporter hit and the code ranks by what is broken
contributions_offered:
  consumer: the reporter has both wire formats, a client it owns outright, and a live screen of the shape this is for, and will implement against a prerelease
  measurements: transfer sizes per delivery on real pages before and after, which is the number that should decide whether the emitter work is worth it
  taken: both; the measurement offer is worth more than an estimate, and requirement:update-wire-contract already showed what a second implementation finds
not_asked_for:
  - a wire format, a record shape, or a protocol version, per decision:caller-owned-wire-versioning
  - a browser runtime, settled in the v0.3.5 round
  - boundary identity, the validators, the manifest codec, or the delta operation kinds
  - requirement:live-mode-plan-slice, which the reporter still wants and explicitly does not make this depend on
carried_forward:
  requirement:live-mode-plan-slice: unchanged and named again, now for the third round; it shares a home with the structured output, since both execute or emit a generated plan in part rather than in whole
  requirement:live-boundary-liveness-signal: not raised this round
related:
  - requirement:structured-render-output
  - rule:dynamic-slot-kinds
  - requirement:fragment-render-options
  - requirement:partial-update-boundaries
  - decision:dom-application-strategy
  - decision:framework-integration-seams
  - decision:update-composition-seams
  - requirement:live-mode-plan-slice
```
