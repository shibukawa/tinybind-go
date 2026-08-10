---
id: requirement:update-wire-contract
type: requirement
title: Update Wire Contract
---
Publish the emitted wire shape as a normative document plus a conformance harness, so a caller-written client is written against a specification rather than against this module's JavaScript.

```yaml
priority: must
source:
  - downstream framework caller-owned runtime report 2026-08-04, against v0.3.3
  - decision:client-runtime-ownership open_questions
  - decision:caller-owned-wire-versioning
review_gate: proposed
answers: the decision:client-runtime-ownership open question on whether the contract lives in documentation, a versioned schema file, or a conformance test suite
answer: a normative document, plus a conformance harness over observable wire behavior
why_it_blocks_the_rest:
  stated_since: decision:client-runtime-ownership module_owns has listed the normative protocol contract since 2026-07-27, and it does not exist
  effect: 'the caller owns the script' resolves in practice to 'the caller reads htmlupdate/runtime.js and infers the protocol'
  downstream_evidence: the reporter implemented apply, live delivery, and reconnect a second time beside this module's, because there was no specification to implement instead
  reading: an open question under a decision recorded as shipped is a silent blocker on everything downstream of that decision
sections:
  header_namespace: which headers exist, how the configured prefix composes each name, and which are request and which response
  modes: the token grammar, which tokens are request modes, and that an unrecognized token resolves to a complete document
  delta_records: the streamed record shape, one per line, the operation kinds, and the terminator separating a finished render from a truncated one; no record carries a version field, per decision:caller-owned-wire-versioning
  manifest: the encoding of the validator set the client holds and returns, and the oversize rule
  head_operations: the shape, and the ordering guarantee that head is installed before the markup needing it
  redraw_response: body form, the base64-of-JSON head header, the ETag and 304 contract, and each failure status with its meaning
  action_response: the region list shape and the head field
  build_identity: what a mismatch means and what it falls back to, which decision:caller-owned-wire-versioning leaves as the only compatibility axis the module operates
  client_obligations: the marker trigger-source rule, apply-at-most-once, and falling back to ordinary navigation on every failure path
  live_handoff_sequence:
    missing: 2026-08-08; the document states the live marker and not what a client does with it across a navigation
    sequence: abort the outgoing page's live request, apply the navigation operations, then open a new live request when the response marked live
    why_ordered: rule:live-boundary-delivery navigation_ordering requires the abort before application, so a delivery for the outgoing page cannot land on the incoming one; opening before applying would subscribe against a composition not yet on screen
    why_it_belongs_here: it is a three-step obligation no code states, which is the shape decision:client-runtime-ownership protocol_modes already names as the section a second implementation cannot infer
  redraw_invalidates_its_own_validator:
    obligation: after applying a redraw, a client drops its manifest entry for that instance
    why: a reloadable component became an update boundary on 2026-08-08, so the client now holds a validator the page render produced and the redraw just made stale; comparing against it makes the next navigation delta re-send the region and replace DOM the redraw had already put right
    rule_it_applies: rule:delta-consistency-model document_validator invalidation, at instance scope rather than document scope
    until: requirement:component-redraw-endpoint json_body lets the redraw response carry the new validator, at which point the client stores it instead of dropping it
    superseded: 2026-08-08; that body shipped, so a client stores the returned entry and this obligation is gone
  conditional_requests_belong_to_the_browser:
    obligation: a client never sets If-None-Match itself; it issues an ordinary fetch and lets the browser revalidate under the response's own cache policy
    why: the browser reconstructs the full 200 from its store, so head, live, and manifest always reach the client; a client-issued conditional gets a bare 304 and loses all three
    settled: 2026-08-08, requirement:redraw-cache-policy etag_after_the_json_body carries the reasoning
  history_navigation_sends_what_is_on_screen:
    obligation: on a same-document history navigation the client sends the manifest describing the DOM currently displayed, not the one stored with the history entry it is moving to
    failure_it_prevents: going from A to B and back to A restores A's manifest while the screen shows B, so every boundary compares equal and nothing is sent
    invariant: a manifest describes a DOM, and history state is keyed to a URL; decision:manifest-state-ownership carries it
    alternative: send no manifest and take a complete render, which is always safe
  supersession_discipline:
    missing: 2026-08-08; the document states no rule for a response that arrives after a newer one was applied
    what_the_catalog_promised: rule:delta-consistency-model fencing, a navigation sequence and a per-instance revision, neither of which is on the wire
    what_the_code_does_instead: removes each race at its source by aborting the superseded request, which works wherever the client owns both sides
    what_the_document_must_say:
      - a client aborts a pending redraw for an instance before issuing another for the same instance
      - a client aborts every pending redraw before applying navigation operations, alongside the live request it already aborts
      - a response whose request was aborted is never applied, even when its bytes already arrived, because the client owns its own apply queue
    why_it_is_the_whole_fix: with the abort discipline complete, no ordering field is needed; without it stated, two implementations pick different halves and only one of them is safe
    concrete_case_it_closes: a search box redrawing per keystroke, where the shorter query answers last and leaves the region showing its result under the longer query's input
  signal_records:
    written: 2026-08-11, by concept:signal-channel
    states: the data:signal record kind, the name grammar and reserved prefix, and the requirement:client-signal-dispatch obligations, which are registration before dispatch, table-only resolution, ignoring an unknown name, and treating the payload as data
    obligation_seven: table-only name resolution, stated separately in the normative list because it is the one rule whose violation reintroduces code execution
    still_owed: the requirement:runtime-lifecycle-signals reserved set and the moment each fires, which is client behavior with no wire form and therefore has nowhere else to be written
    why_it_belongs_here: it is a client obligation with no code a reader can check, which is the shape mostly_assembly.exception already names
  live_marker_has_three_carriers:
    what: the live response header from markLive, the live field renderDelta sets on a buffered delta, and ExpectLive on a stream
    load_bearing: the header for a document response, which has no JSON body to carry it
    redundant: the delta, which sets both
    why_record_it: three mechanisms for one fact, and a reader who fixes one leaves the others; the document must say which a client reads
mostly_assembly:
  reason: every section above is already recorded in this catalog or readable from one file
  so: the work is collecting it and making it normative rather than designing it
  exception: the obligations, which live in decision:client-runtime-ownership protocol_modes and in no code a reader can check
not_versioned_here: per decision:caller-owned-wire-versioning the document describes emitted data items and client obligations rather than a numbered protocol this module polices
harness:
  tests: observable wire behavior, meaning the requests a client issues, the responses it consumes, and the resulting DOM
  not: a JavaScript entry surface a client must expose, which would be the Go interface decision:client-runtime-ownership constraints refuse, written in another language
  starts_from: requirement:client-update-rollout m1 client_coverage, the node suite over a stubbed DOM covering header construction, validator bookkeeping, supersession, and fallback
  value: two implementations stay honest against each other rather than against a reading
external_review:
  offered: the reporter reads the document against its own independent implementation
  why_take_it: a second implementation is the cheapest way to find where a specification is under-determined, and the only one available before release
constraints:
  - the document states what the module emits and what a client must do, never a Go interface or a JavaScript API
  - a conforming client that never runs this module's reference client is possible, which is the test of whether the document is complete
  - the document publishes one redraw addressing, per requirement:caller-addressed-redraw
acceptance:
  - a caller writes a working client from the document alone, without reading htmlupdate/runtime.js
  - the harness runs against a caller-supplied client and the reference client alike
  - every failure path in the document lands on ordinary navigation
as_built:
  shipped: 2026-08-04
  document: docs/httpbind_update_wire_contract.md, normative, with conformance language and every section this requirement lists
  no_version: the document opens by saying the module owns no protocol version and why, per decision:caller-owned-wire-versioning
  harness: docs the existing node suite as the conformance harness and states that it tests observable wire behaviour rather than a JavaScript entry surface
  linked_from: the reloadable-component guide in both languages, at the point where the build identity is explained
  it_paid_for_itself_immediately: writing the redraw section surfaced that the reference client read a hardcoded 'data-tb-kind' while the generator emits the configured prefix; see requirement:caller-addressed-redraw as_built.found_while_building
  reading: a defect invisible to one implementation is what a specification is for, and this one was found by writing the specification rather than by a second implementation reading it
related:
  - decision:client-runtime-ownership
  - decision:caller-owned-wire-versioning
  - requirement:caller-addressed-redraw
  - requirement:runtime-default-retirement
  - requirement:update-protocol-naming-ownership
resolved:
  where_it_ships: its own file, docs/httpbind_update_wire_contract.md, linked from the reloadable-component guide; a normative document read by someone writing a client is a different artifact with a different audience from a guide read by someone using the module
open_questions:
  - how the harness loads a caller-supplied client without becoming an entry-surface contract
```
