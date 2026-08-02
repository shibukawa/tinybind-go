---
id: decision:update-composition-seams
type: decision
title: Composition Seam Requests From The Downstream Framework
---
Accept the fifth downstream round, and record that its headline item is not a divergence to settle but a convergence neither side could see, because a family of live design concepts still claim to be blocked on a requirement that shipped.

```yaml
source:
  - downstream framework composition seam report 2026-08-02, against v0.3.1
  - decision:framework-integration-seams
  - decision:update-runtime-ownership-seams
review_gate: proposed
round:
  when: 2026-08-02, the fifth round from this reporter
  previous: generation seams 2026-07-30, live integration 2026-07-31, component and asset seams 2026-07-31, runtime ownership 2026-08-01
  reporter_position: not blocked; one item is a wire contract that is cheap now and expensive once either side ships against it, and the rest are gaps with visible workarounds
  reporter_id: the report carries the same content in machine form as a requirement named tinybind-update-composition-seams, recorded in that project's catalog rather than this one
  previous_round_closed: the reporter states v0.3.1 answered its round in full, and asks nothing about it again
verification:
  method: every claim checked against the v0.3.1 source and against this catalog
  no_live_token_in_go: confirmed; the only mode constants are navigation, action, and redraw, and nothing parses a live token
  no_live_token_in_the_client_either: confirmed and stronger than reported; the shipped runtime's live entry sends 'navigation;v=N', so both halves agree with each other and neither agrees with the guide
  guide_publishes_one: confirmed; the mode table publishes 'X-Tinybind-Render: live;v=N' and the availability table marks live delivery and reconnection available
  requirement_still_lists_the_spelling_as_open: confirmed; requirement:live-reconnect open_questions names the mode spelling and the body form, while its status line says delivered
  head_absent_from_redraw_and_action: confirmed; the navigation delta sets a head field and neither the action body nor the redraw response has one
  backoff_linear: confirmed; the client multiplies a base delay by the attempt count and reloads at a fixed attempt cap
  slot_fragment_head: confirmed, and already recorded here; requirement:fragment-capability-introspection slot_parameter_propagation states that Bind copies only the plan's own head
  asset_set_and_registration_unimplemented: confirmed; requirement:component-asset-requirements and requirement:builtin-element-registration are designed and unbuilt
two_corrections_to_the_reporter_s_reading:
  where_the_first_delivery_lives:
    reported: the module holds the document response open and its tail keeps carrying deliveries, so its live mode covers reconnect only
    true_of: htmlbind.RenderChainLive, the transport-neutral entry a caller drives directly, which is what the htmlbind guide documents
    false_of: htmlupdate, whose document path settles a live boundary in place through the blocking op and lets the document finish
    so: htmlupdate already does what the reporter described as its own divergent choice, which is also decision:live-transport-boundary chosen.document_mode and first_delivery_inline
  reconnect_only:
    reported: the live mode covers the reconnect and not the first connection
    actual: there is no live mode; the client's live entry opens a navigation-mode record stream, and the first connection and every reconnect are that same request
    so: the two-phase shape the reporter presents as its own is the shape both catalogs chose and the shape htmlupdate ships
convergence_is_larger_than_either_side_thought:
  agreed_already: whole-region state rather than increments, no cursor and no replay, positional boundary ids reproduced by a re-render, navigation supersedes, a version mismatch falls back to a document, the document response finishes, and a second connection carries deliveries
  agreed_for_the_same_reason: the reporter argues that a response deliberately held open is what produces the proxy timeout and sleep-resume drops a reconnect exists to repair; decision:live-transport-boundary rejected_endless_document_response made that argument first, independently, and lists four consequences rather than one
  what_actually_remains:
    - the token, which is published and implemented nowhere
    - the record vocabulary, where the reporter distinguishes a finished stream from a healthy close at a lifetime bound and this module has end, navigate, and error
    - the handoff marker naming whether a live request is expected at all, which rule:stream-termination-marker specifies and nothing emits
    - whether a live body carries validators, which the reporter declines and requirement:live-reconnect still asks
  reading: the round reads as a divergence to negotiate and is a small set of unfinished pieces around a shape both sides already share
headline_finding:
  what: requirement:live-reconnect, requirement:live-boundary-resume, rule:stream-termination-marker, and decision:live-transport-boundary all carry 'not implemented; blocked on requirement:component-delta-rendering'
  but: requirement:component-delta-rendering is delivered, and has been since requirement:client-update-rollout m1 and m2
  meanwhile: a live entry shipped by delegating to the streaming navigation entry, the stream terminator shipped inside that stream, and the guide published a mode token no code on either side sends
  effect: four concepts claim to be waiting for something that arrived, one of them also says delivered, and a reader checking any of them gets a different answer about whether the live mode exists
  same_shape_as_the_previous_round: decision:update-runtime-ownership-seams found a recorded deviation whose exit was never scheduled; this is a recorded blocker whose clearance was never noticed
  new_reading: a status line naming a blocker is a claim that decays, because nothing re-reads it when the blocker ships; a blocked concept needs the unblocking concept to name it back, or the claim outlives its truth
accepted:
  - what: requirement:live-mode-token-contract
    value: highest; it is the only item that is a wire contract, and the reporter is right that settling it after either side ships costs a coordinated deploy
    cost: low for the documentation half, which is a defect either way; medium for the token and the handoff marker
    covers: the token, the body form, the handoff marker, and the guide
    takes_as_input: the reporter's control vocabulary, its done-versus-retry distinction, and its reset-on-healthy-close backoff, offered rather than asserted
  - what: requirement:fragment-response-head
    value: high; it is the one item in the round that silently renders wrong output, and the navigation path already added the field that prevents it
    cost: low on the action path, which builds the same body the navigation delta does; higher on the redraw path, which writes a subtree and has nowhere to put a head today
  - what: requirement:component-asset-requirements
    verdict: already designed here; the ask is priority rather than shape
    why_it_moves: it is what makes the item above solvable without a mid-swap fetch, and its hardest property, the statically known required set, is exactly the part the reporter needs
    third_round_in_a_row: raised 2026-07-31, unchanged 2026-08-01, named again here as the blocker under a concrete defect
  - what: requirement:builtin-element-registration
    verdict: already designed here; unchanged from the 2026-07-31 round
    not_blocked: a synchronous external returning html with the render context is a working interim shape, so this costs one declaration per template file rather than the feature
  - what: requirement:slot-fragment-head-merge
    verdict: promoted from a finding to a requirement, because a second reporter reached it from a third direction
    value: medium; it makes an existing safety check incomplete rather than absent
withdrawn_before_it_was_sent:
  what: a delta-response CSRF token refresh channel
  why: the reporter found the defect was in its own transport, a page-embedded token with no refresh channel, and moved to a cookie read at request time and refreshed by set-cookie
  effect_here: policy:html-update-csrf-protection can retire its delta-response token refresh header question, since the one caller that wanted it no longer does
  worth_recording: the reporter states its reasoning rather than only its conclusion, which is what let this catalog retire a question rather than merely drop it
carried_forward:
  requirement:live-mode-plan-slice: unchanged; a live render still executes the whole composed chain per reconnect
  requirement:live-boundary-liveness-signal: unchanged; liveness is reported per chain and nowhere per boundary
  reading: both were accepted in decision:live-integration-seams and neither was built, so a fifth round listing them is the reporter keeping an accepted item visible rather than re-asking
sequencing:
  chosen: the guide first, then the token and the handoff marker, then the action head, then the slot head, then the redraw head behind the asset set
  why_the_guide_first: it is a factual error in the repository, it costs nothing, and it is what let the reporter build against a token that does not exist
  why_the_token_second: it is the item whose cost grows with every day either side ships against it, which is the reporter's own argument and it is correct
  why_the_action_head_third: it is the same body the navigation delta already builds, so it is one field
  why_the_redraw_head_last: a redraw writes a subtree with nowhere to carry a head, so it wants requirement:component-asset-requirements rather than a fourth ad hoc channel
  not_a_disagreement: the reporter sequenced by what each item unblocks and put the token first; this agrees on the token and puts a documentation defect ahead of it only because that costs nothing
principle:
  applies: the decision:framework-integration-seams rule unchanged, widen a seam whose default output stays identical and whose contract stays the caller's
  strained_by: the token, which is not a seam but a wire contract; widening does not apply, and settling does
not_asked_for:
  - the delta protocol, the validators, the manifest encoding, or the operation kinds
  - anything settled by v0.3.1
  - a second convergence of the live transport, which the reporter names as its own decision rather than a module gap
related:
  - decision:live-transport-boundary
  - requirement:live-reconnect
  - requirement:live-boundary-resume
  - rule:stream-termination-marker
  - policy:html-update-csrf-protection
```
