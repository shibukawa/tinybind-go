---
id: requirement:boundary-decomposed-render
type: requirement
title: Boundary Decomposed Render
---
Return one render as its per-boundary HTML fragments plus the boundary tree that describes them, so a caller chooses which fragments to transfer instead of receiving one assembled subtree.

```yaml
priority: should
as_built:
  shipped: 2026-08-08, the boundary-hole tier
  capture: a boundary's captured fragment stops at its children — Collector.Write records into the innermost open boundary only, where it used to write into every enclosing one
  holes: 'the collector writes an inert placeholder into the parent where a child opens, the same <prefix>-boundary element with display:contents a progressive render already writes for an await boundary, so a client recognises one hole shape rather than two'
  boundary_list: Operation.Boundaries names the holes in a fragment, and travels as a boundaries field on the JSON body and on the stream record
  selection_inverted:
    was: send the topmost changed boundary carrying its whole subtree, and skip every descendant because the replacement contained it
    now: send every changed boundary as its own fragment, and skip every unchanged one including a child of a changed parent, whose hole the client fills from the DOM it already holds
    gain: a changed panel no longer recreates the rows inside it, so the focus, the form values, and the playing media those rows held survive
  frame_covers_the_hole:
    found_while_building: the placeholder has to be hashed into the parent's frame even though the child's bytes are not
    without_it: a parent that gained or lost a region compares equal, and the client is handed a fragment with no hole to put it in
    reading: this is the difference between a frame that excludes a child's content and one that cannot see the child at all
  collector_signature: 'Begin gained the placeholder element name, because a decomposing observer writes markup and the naming is a render option it cannot otherwise see'
  await_needs_nothing:
    why: an await boundary already writes a placeholder carrying its own id, and its completion already arrives as a later record addressing that id
    so: a boundary inside a fragment is a hole of the kind this concept already describes, and a caller overwrites by id
  verified:
    - TestFragmentCarriesHolesNotChildren, that a parent carries a hole and none of its child's bytes
    - TestOnlyTheChangedRowIsSent
    - TestChangedParentRetainsUnchangedChildren, the case the change exists for
    - TestAGainedChildChangesTheParentFrame, covering both halves of the frame rule
    - TestFirstDeltaReturnsEveryBoundary, rewritten from asserting whole-subtree containment to asserting a fragment per boundary
  slot_spans: shipped later the same day; see slot_spans_as_built
  redraw_json_body: shipped 2026-08-08; requirement:component-redraw-endpoint json_body carries it, and the validator gap it was opened for is closed
slot_spans_as_built:
  shipped: 2026-08-08
  derivation_is_from_the_plan_not_the_render:
    what: Plan.Sequence walks the instruction list and evaluates nothing, so one plan yields one tree however many times it renders
    why_that_matters: it is what makes a sequence addressable — a server rebuilds one from the plan rather than having stored what a render happened to produce
    corrects: the earlier reading that this needed generated tables; the plan is already the static data the derivation needs, so no emitter change was required after all
  tree_shape: static text, a slot, a conditional carrying both branches, a repeat carrying the body once, and a component carrying the two shapes a call takes
  values: one flat stream per fragment — a slot's output, the branch a conditional took, the count a loop ran, and whether a call opened a boundary or rendered inline
  hole_frames_are_static: a called boundary contributes only its attribute name and its id, so a hundred holes are a hundred ids rather than a hundred copies of one element
  round_trip_is_the_property: Sequence.Reassemble rebuilds the exact bytes the render wrote, and a value list that does not fit the tree is refused rather than reassembled into something plausible
  wire:
    request: a client sets the sequences header to say it can walk a tree
    response: an operation carries the address and the values in place of the markup
    fetch: Options.Sequence answers a tree by address, on a URL the caller chooses, public and immutable because a sequence derives from the template rather than from a request
    build_exempt: a sequence request skips the build comparison, since the address digests the tree; gating it would forfeit the one response here a shared cache may hold across builds
    unknown_address: answered not found, and a client falls back to asking for markup — a sequence is an optimisation over something still available
  per_fragment_choice:
    found_by_measuring: 'a hundred-row panel is 10,641 bytes of markup and 6,429 as values, only forty percent, because the address is per-operation overhead and a small fragment has almost no static text to save'
    rule: values replace markup only where they are smaller, decided per fragment
    consequence: the split is never a loss; a list row stays markup and its parent, a hundred hole frames, becomes values
  found_while_building:
    a_slot_is_a_called_component: a filled slot renders another fragment exactly as a call does, so it takes the component shape; treating it as an ordinary instruction made the parent's values carry both the hole and the subtree it stands for
    a_chain_member_opens_its_own: memberFragment already opens and closes, so a slot holding one records the shape and leaves the opening alone rather than opening twice
slot_span_readings_that_were_wrong:
  emitter_work:
    said: the sequence has to be generation-time data, so this is emitter work after all
    actual: it has to be plan-derived, and a plan is already static data the runtime holds, so the derivation walks the instruction list and no emitter change was needed
    what_survives: the reason, which is that a sequence assembled from what a render happened to produce cannot be served back from its address
  what_it_buys:
    estimated: roughly fourfold on a cold page
    measured: forty percent on a hundred-row panel taken whole, because the address is per-operation overhead and a small fragment saves almost nothing
    resolved_by: per_fragment_choice, which sends values only where they are smaller, so the estimate was optimistic and the shipped rule is never a loss
structural_operations:
  shipped: 2026-08-08, immediately after the decomposition, because measuring it showed the decomposition alone missed the case it was built for
  measured_first:
    method: a hundred-row message panel, bytes over the wire per event
    decomposition_alone: 'a full render is 15,328; editing one row is 76; changing the title is 7,240; appending one row is 7,383'
    reading: the case decomposition nails is one region changing among unchanged siblings, at two hundred times; anything that changes the parent re-sends its whole list of holes, at about two
    why_that_mattered: appending is the ordinary event on a live list and is the reporter's own measured headline case, so the round had optimised everything except what it was for
  what_shipped:
    frame_stops_at_the_hole: the placeholder is no longer hashed into the parent's frame, so a frame answers only whether the component's own markup moved
    children_validator: a second digest over the nested boundary ids in order, carried beside the frame in the manifest
    op_children: an operation naming a boundary and its new child order, carrying no markup; the client keeps what the list keeps, moves what moved, drops what it omits, and fills what arrives as its own operation
    three_way: frame changed means replace; frame equal and children changed means the order alone; both equal means nothing
  measured_after: 'appending one row to a hundred fell from 7,383 bytes to 365, twenty times, and to forty-two times against a full render'
  what_it_still_costs: changing the parent's own markup replaces it with its whole hole list, which is 7,240 bytes at a hundred rows and is what slot spans would compress to the ids alone
  removal_is_the_subtle_half:
    covered: a removal whose parent survives with an unchanged frame, since that parent reports the survivors and the client drops what the list omits
    not_covered: everything else, which keeps the outermost-replacement fallback
    why_the_frame_and_not_mere_survival: a chain member is numbered by position, so a shorter chain renumbers and an id surviving can mean a different component wearing the same number, whose operation says nothing about the region that went
    found_by: the existing disappearing-boundary test, which failed against the looser rule
  manifest_grew: 'an entry is now id:frame:children:parent rather than id:frame, adding roughly thirty bytes per instance; the parent is what lets a removal be attributed at all'
  verified: TestAnAppendedRowCostsItsOwnFragmentAndAnIDList, TestARemovedRowIsAnIDListToo, TestAReorderedListSendsOnlyTheOrder, TestAChangedParentIsStillReplaced, and the unchanged disappearing-boundary fallback
  never_reached_the_streams:
    reported: downstream framework defect report 2026-08-09, reproduced against the streamed navigation path with three rows
    cause: every stream site decided what to write by asking whether markup was present, and a children operation carries none by design
    three_faces_one_cause:
      streamed_navigation: fell into the unchanged shape, so nothing said where the appended row went and a client could only reload — the reported one, and the mildest
      live_delivery: fell into the branch that deliberately says nothing for an unchanged boundary, so the record was dropped and the row never appeared
      streamed_render: written as a replace carrying no markup, which empties the region rather than reordering it — silently destructive, and the worst
    reporter_found: the first; the other two came from reading the same branch on the other two paths
    fix: dispatch on Operation.Kind, and a DeltaStream.Children writer, which did not exist
    fourth_gap_found_with_it: an op record carried the frame validator and not the children one, so a client rebuilding its manifest from a stream returned half of what the next request compares against and every list looked reordered
    verified: TestStreamedNavigationCarriesTheChildrenOperation, TestLiveDeliveryCarriesTheChildrenOperation, TestStreamedRenderDoesNotEmptyTheList
    reading: the buffered path copied Kind because it copies the operation; the streamed paths reconstructed one from its parts, and a reconstruction is where a new kind goes missing
    then_the_same_shape_again:
      reported: the same reporter, immediately after, on the field the first fix added
      what: an op record carried the frame and the children digest and not the parent, where a manifest entry has all three
      cost: disappeared reads the known parent, so a client rebuilt from a stream could not attribute a removal and fell back to replacing the outermost boundary — expensive in exactly the case a children operation exists to make cheap
      fixed: every op record carries the whole entry, through a ManifestEntry value rather than a growing parameter list, so the next field added cannot be forgotten at one of four call sites
      verified: TestStreamRecordsCarryTheWholeManifestEntry and TestAShrinkingListStaysAChildrenOperation
      reading: three rounds of the same defect, each one field of a value that was being passed apart; the fix that holds is passing the value
source:
  - owner scoping decision 2026-08-08
  - downstream framework partial transfer report 2026-08-08
  - data:component-delta-response retain_holes
review_gate: proposed
position:
  what_the_module_owes: HTML fragments, and the metadata that says how they compose
  what_it_does_not: the wire format, the live transport, and which fragments are worth sending, all of which stay with the caller per decision:caller-owned-wire-versioning and decision:client-runtime-ownership
  reading: the module decomposes; the caller selects
shape:
  records: an ordered list mixing two kinds
  boundary_list: the reloadable boundary ids appearing inside one fragment, declared for that fragment's scope
  fragment: one boundary's HTML with its id
  holes: a fragment carries an empty placeholder element per nested boundary, carrying that boundary's id, so the parent is structurally complete without its children
  nesting: a child fragment declares its own boundary list, so the tree is expressed by ordinary nesting rather than by a separate index
  ordering: an ancestor precedes its descendants, which lets a client install a parent and fill each hole as its fragment arrives, matching requirement:streaming-delta-response structural_first
the_list_is_not_redundant:
  apparent: the ids are readable from the placeholders in the parent's HTML, so a separate list looks like duplication
  two_kinds_of_hole:
    retain: the client already holds that node and moves its live DOM in, preserving rule:preserved-client-subtree state
    install: a fragment for it arrives in this response and replaces it
  what_the_list_decides: a hole with no fragment is a retain rather than a truncation, which nothing else in the response can say
  and_it_carries_the_selection: the list declares the complete structure while the fragments sent declare the selection, which is what lets a caller omit fragments without the response becoming ambiguous
two_tiers:
  decided: 2026-08-08 by the owner
  boundary_holes:
    members: reloadable, @cache, and chain members
    buys: DOM retain and independent addressing
    needs: one root element and an id, which reloadable and chain members already guarantee and which decision:cache-component-declaration gained for this
  slot_spans:
    what: every dynamic op's output inside one fragment — a value insertion, a conditional, an attribute — reported as its own span so the fragment's static parts separate from its variable ones
    buys: transfer only; the client reassembles and reparses, so application is unchanged
    needs: an ordinal position in op order for a slot, and nothing else — no element, no comment marker, and no id
    the_sequence_itself: the static sequence a fragment's slots interleave with is identified by a content address, so an ordinal names a slot within a sequence and the address names the sequence
  independence: the upper tier preserves state and the lower tier removes bytes, and neither depends on the other
entry_shape:
  decided: 2026-08-08, the decomposing entry yields records as an iterator sequence rather than collecting them
  matches: the RenderAsync entry of api:render-html-chain, so the decomposed path has the same two-entry shape the byte path has
  forced_by: a fragment may contain an await boundary, so the fragments of one render do not all exist at one moment
  what_collecting_would_cost: requirement:suspense-html-streaming, since nothing could be sent until the slowest boundary settled
static_sequence_delivery:
  decided: 2026-08-08, fetched by content address rather than sent inline
  unit: one sequence per fragment shape, not per delivery, per chunk, or per row; a list of five hundred rows shares the sequence a list of five uses
  three_lifetimes:
    per_template: the static sequence — public and immutable, valid until a template changes
    per_request: the document or the decomposed response — private, one chunked response, ended by rule:stream-termination-marker
    per_subscription: the live mode — private, chunked, open for the subscription's lifetime
    argument: riding a static sequence on a request response makes it inherit that response's cache policy and lifetime, both of which are wrong for it
  it_is_the_only_shareable_thing_here: a static sequence derives from the template rather than from a request, so it is the only response on this wire that can be public and cached at an edge; every other one is private
  cache_policy: public and immutable, keyed by the sequence address, so a deploy invalidates by producing new addresses and no explicit invalidation exists
  constraint: a sequence request is a render mode on a URL the caller chooses, never a path this package mounts; requirement:caller-addressed-redraw removed the last mounted registry route and adding one back would restore what it deleted
  no_client_held_address_list:
    decided: 2026-08-08 by the owner; a client never sends the sequence addresses it holds
    why_it_is_unnecessary: the choice between an assembled fragment and its spans is a heuristic rather than a contract, because both branches are correct — spans a client cannot resolve cost it one fetch, and an assembled fragment where spans would have done costs a few bytes
    what_the_server_already_knows: the returned data:component-update-manifest distinguishes a fresh navigation from a same-page re-render by which chain validators match, so new-versus-same is derived rather than declared
    avoided: the request-size cost decision:manifest-state-ownership already carries for validators, doubled for a second per-instance list
    caller_policy: a fresh navigation may send assembled and omit the layout the outgoing page shared; a same-page re-render may send spans and trim further; both are the caller's to tune
    no_waterfall_moves: the property that a first paint waits for nothing is now the caller's to hold, by choosing assembled for a cold client, rather than the module's to guarantee
  a_sequence_must_be_data_independent:
    found: 2026-08-08, while starting the implementation
    the_shortcut_that_fails: emitting the statics flat, as the render happened to produce them, with a digest of those bytes as the address
    why_it_fails: a flat sequence depends on which branches ran and how many times a loop repeated, so the server cannot reproduce one from its address; a fetch endpoint would have to have stored every sequence it ever emitted, which is the server-side per-document state decision:manifest-state-ownership refuses
    consequence: the sequence must be a generation-time artifact, which is what makes it fetchable, permanently cacheable, and public
    reading: fetchability is not an optimisation layered on the encoding; it is what decides the encoding
  a_sequence_is_a_tree_not_a_list:
    resolves: how a data-independent sequence covers a conditional and a loop without enumerating paths
    rejected: one address per instruction path, which is exponential in the conditionals a component holds
    chosen: one address per component, whose sequence is a tree of static text, slots, conditional nodes, and repeat nodes
    where_the_path_goes: into the values rather than into the identity — which branch ran and how many times a loop repeated travel with the data, and the client walks the tree with the same choices the server made
    consequence: a five-row list and a six-row list share one address, which is what the earlier flat reading could not give them
  sequence_identity:
    decided: the plan fingerprint, one per component, since the tree above removes the need for a path selector
    superseded_proposal: the plan fingerprint plus a selector for the instruction path taken, rather than a digest of the statics
    why_not_a_digest: the statics depend on which branches ran, so a digest would be computed per render, and decision:cache-key-derivation states this module hashes nothing in the runtime
    properties_kept: stable across renders taking the same path, changed by a template edit through the plan fingerprint, and cheap because the renderer already knows which branch it took
    accepted_waste: two paths whose statics happen to be identical get two addresses, which costs a client one redundant sequence and nothing in correctness
    not_a_leak: a path selector reveals which branch ran, and the client receives that branch's span values in the same response, so it learns nothing more
    no_runtime_hashing: an identity computed at generation time also settles the tension with decision:cache-key-derivation, which states this module hashes nothing at request time
  a_sequence_is_not_a_flat_list:
    finding: a fragment containing a for loop has statics of the form prefix, body repeated, suffix, and the repetition count is data
    consequence_if_flat: a five-row list and a six-row list would produce different addresses, which defeats the sharing the sequence exists for
    shape: a sequence carries a repeat node referencing a sub-sequence, so a loop body is its own address and the spans group per iteration
    why_the_reporter_had_this: their original sketch gave a row its own template identity with one value list per item, for exactly this reason
    keys_not_needed: for transfer the groups travel in order, so decision:list-item-key is not involved; a key decides DOM pairing, which a reparse does not do
slot_spans_are_cheap:
  mechanism: a plan is an ordered op list and each op writes one contiguous range, so recording each non-static op's start and end makes the statics the gaps between them
  identity: a slot is named by its position in op order, because a fixed op path cannot reorder or omit one
  no_emitter_change: this is a renderer recording ranges, not an emitter restructuring what it emits; the earlier estimate that every attribute op had to grow a static frame applied to splitting for application, not for transfer
  control_flow: an if or a for changes which ops run, so each distinct op path is its own static sequence, which is what a content address already keys
the_reparse_dissolves_the_hard_parts:
  reading: every difficulty recorded in requirement:structured-render-output came from splitting so a client could apply a value to the DOM; splitting only for transfer removes them
  escaping: values travel already escaped, exactly as the module writes them today, because the client concatenates and reparses rather than assigning; a client needs no escaping knowledge at all and the requirement:url-attribute-scheme-safety check stays server-side by construction
  optional_attribute: a slot is one whole attribute including its name and quotes, so an absent value is the empty string rather than a change of structure
  boolean_attribute: the same, being either the attribute text or empty
  mixed_attribute_value: one string, produced by the closure that already assembles it, so no double escaping arises
  raw: unchanged, since the fragment is reparsed anyway
  consequence: the reporter's original flat statics-and-dynamics sketch was right for transfer; the objection to it was an objection to splitting for application
  what_it_does_not_give: the no-reparse property, so focus, selection, form state, media, and animations inside a fragment are still lost on replacement
measurement_recovered:
  correction: this concept first recorded that the reporter's threefold figure did not transfer, which was true of boundary decomposition alone and is false once slot spans are included
  why: the reporter's projection assumed every value sent and the skeleton amortized to once per connection, which is exactly the slot-span shape
  v1: send every slot value on every delivery and the static sequence once; omitting unchanged slots would need per-instance client state and a manifest for it, and the static parts dominate without it
static_boundaries:
  claim: a boundary whose whole subtree contains no dynamic op is knowable at generation time
  how: the same call-graph walk that already computes HasAwaitBlock, HasLiveBlock, Assets, and Vary; Plan carries no such flag today
  what_it_buys_over_a_validator:
    validator: requires the client to have sent one and the server to digest the render, and omits only when they match
    static: settled at build time, so it needs no digest and no client hint, and is keyed by the build identity alone
  consequence: a static boundary is never transferred again under one build, on a same-page redraw or any other request
  where_the_value_is: the larger the static subtree, the more the decomposition is worth, which is the owner's own reading of why this is the piece worth building
not_a_new_mode:
  decided: this is what the delta and redraw bodies become, not a fourth render mode
  why: requirement:component-redraw-endpoint json_body already moves a redraw to the ops-and-head shape the navigation delta writes, and ops is already a list of kind, id, and html
  what_is_added: the boundary list record, the retained placeholders data:component-delta-response already designs, and a static marker
  what_it_avoids: a longer requirement:render-mode-negotiation table and one more branch in every caller
what_it_does_not_reach:
  application_half: a fragment is reassembled and reparsed, so focus, selection, form state, playing media, and animations inside it are still lost on replacement
  the_reporter_s_ranking: the downstream named that half as the one a caller cannot approximate, so this answers the transfer half in full and defers the other
  deferred_to: requirement:structured-render-output, which keeps its analysis and becomes a later stage of decision:dom-application-strategy
what_it_retires_from_this_round:
  - rule:dynamic-slot-kinds as an application contract, since a client assigns nothing; escaped values and a concatenation are all it handles
  - client-side marker insertion and the post-parse addressing problem, since the client addresses no node
  - the emitter changes for mixed attribute values, optional attributes, and per-attribute static frames, which the span recording makes unnecessary
  kept_from_it: content-addressed static sequences and their fetch, which the slot-span tier needs in order to send the statics once
  reading: what the full split costs is the ability to apply a value without reparsing; the transfer saving comes for far less
constraints:
  - a boundary renders exactly one root element, per decision:update-manifest-transport, which is what makes a placeholder expressible
  - rule:template-context-safety is untouched, because every fragment is produced by the ordinary render path
  - a caller that selects no fragments still receives a well-formed structure, since the boundary list stands alone
acceptance:
  - one render yields a fragment per boundary plus the tree that composes them
  - a parent fragment carries a placeholder per nested boundary and is installable without them
  - a hole with no fragment in the response is retained rather than emptied
  - a boundary whose subtree is entirely static is identified without rendering it and omitted under an unchanged build
  - a redraw and a navigation delta return the same decomposition, differing only in what each selected
open_questions:
  - whether the static flag is per boundary or per component, given a component may be a boundary in one composition and not in another
  - whether a caller selecting fragments does so before or after they are rendered, since rendering is what a selection would avoid paying for
  - whether the boundary list carries validators, or stays structure alone and leaves comparison to requirement:component-delta-rendering
related:
  - data:component-delta-response
  - requirement:component-delta-rendering
  - requirement:component-redraw-endpoint
  - requirement:partial-update-boundaries
  - requirement:structured-render-output
  - decision:dom-application-strategy
  - rule:preserved-client-subtree
  - decision:partial-transfer-seams
```
