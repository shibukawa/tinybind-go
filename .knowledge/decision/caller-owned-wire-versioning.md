---
id: decision:caller-owned-wire-versioning
type: decision
title: Caller Owned Wire Versioning
---
Stop deciding a protocol version in this module and let the caller own mismatch and reconciliation, because the caller now writes both ends of the wire and the build identity already carries a compatibility axis whose value the caller sets.

```yaml
source:
  - user versioning decision 2026-08-04
  - downstream framework caller-owned runtime report 2026-08-04, against v0.3.3
  - decision:client-runtime-ownership
review_gate: approved 2026-08-04
supersedes: decision:update-protocol-version ownership, comparison, and validator_binding
position:
  module_emits: the data items a client needs, named and shaped
  module_does_not: decide a version number, compare one, or arbitrate a mismatch
  caller_owns: the version axis, its spelling, and what a mismatch means on send and on receive
  reason: decision:client-runtime-ownership already puts the client with the caller, so a module-owned version is one side versioning a contract it only half implements
why_the_module_can_give_it_up:
  build_identity_is_already_the_axis: Options.BuildID is caller-overridable and already compared on every update request, so the module keeps a compatibility check whose value the caller supplies
  it_is_the_stronger_one: a build identity covers a changed template, a changed external function, and a changed client, none of which a protocol integer sees
  the_version_never_paid_for_itself: requirement:live-mode-token-contract changed the mode spellings and kept the version at 1, which by decision:update-protocol-version bump_when was exactly a bump
  coordinated_release: a module-owned version is the mechanism that forces the coordinated deploy the whole ownership split exists to avoid
what_survives_and_is_not_versioning:
  fallback_invariant: requirement:client-update-rollout enabling_invariant, that every unrecognized condition degrades to an ordinary full browser navigation
  why_separate: Negotiate resolving an unrecognized mode name to a complete document is a total function on its input, not a version comparison, and it holds with no version at all
  kept: the module never fails a request because it did not recognize it
what_changes_in_the_code:
  negotiate: parse 'name;v=N', carry N through on Negotiated, and stop comparing it to a module constant
  grammar: the ';v=N' shape stays, because it becomes the caller's field to fill rather than a shape to remove
  echo: a response still echoes the served mode, which requirement:render-mode-negotiation needs so a proxy-substituted body stays detectable
  version_constant: htmlupdate.Version and htmlbind.ProtocolVersion stop naming a contract and become at most a default the caller may ignore
two_places_the_version_is_not_a_header:
  delta_body: the buffered body and each stream record carry a 'v' field the client checks
  validator_digest: decision:update-protocol-version validator_binding mixes the version into every digest so two versions can never compare equal
  replacement: the build identity takes both roles, since it is the axis that actually moves and it is already compared
  consequence: a validator computed under one build never compares equal to one computed under another, which is what the version was bought for
cost:
  admitted: a caller that gets the wire wrong now fails silently where a version comparison would have fallen back
  accepted_because: the caller writes both ends, so the disagreement is inside one project rather than across two
  mitigated_by: requirement:update-wire-contract, which is what a caller checks itself against
as_built:
  shipped: 2026-08-04
  negotiate: parses 'name;v=N' with the version optional, carries N on Negotiated, and compares nothing; an unparseable version reads as none rather than as a refusal, because refusing would cost the page its update over a field this package does not interpret
  echo: renderToken writes back the version the request claimed, and a bare mode name when it claimed none; the action and redraw paths take no request and so echo bare
  removed: htmlupdate.Version, htmlbind.ProtocolVersion, and the 'v' field from the delta body and from every stream record
  digest_tag: htmlbind.WithValidatorTag replaces the ProtocolVersion seeding in both the frame and the input digest; htmlupdate passes o.buildID() from renderOptions, so every entry gets it without a call-site change
  client: the reference runtime carries no version at all and reads the served mode through a helper that ignores any ';v=N' a caller layered on, so it conforms under a caller that versions and one that does not
  open_question_resolved: htmlbind.ProtocolVersion was removed rather than kept, because nothing used it once the digests took the build identity and an exported constant naming a contract this module no longer owns is the misleading thing the decision removes
  swept_after:
    path_prefix_left_the_browser: RuntimeConfig.Prefix and the client's PREFIX were removed once redraw stopped building a URL from them; the asset arrives on the script tag's own src, so nothing in the client had a use for the prefix
    reads_back_on: requirement:update-protocol-naming-ownership, which cited the prefix reaching the browser as evidence that a path namespace is a deployment choice the client must learn; the client no longer learns it
  found_while_building:
    version_zero: a version of zero is indistinguishable from an absent one on the wire, so it echoes bare; stating it is worth a test because the alternative is inventing a number to echo, which is what this package stopped doing
    stream_entries: OpenStream and OpenLiveStream take no request, so they echo bare; the internal openStream gained the version parameter and the negotiated entries pass it
related:
  - decision:update-protocol-version
  - decision:client-runtime-ownership
  - requirement:update-wire-contract
  - requirement:render-mode-negotiation
  - requirement:live-mode-token-contract
resolved:
  delta_body_v_field:
    decided: removed, 2026-08-04 by the user
    where: the buffered body carries it as Version int with a 'v' json tag, and every stream record repeats it
    why: a field nothing compares is not a version, it is a constant the module asks every response to carry and every client to ignore
    caller_may_add_one: the body is what this module emits, so a caller wanting its own version field puts it beside the emitted shape rather than asking for this one back
    lands_in: requirement:update-wire-contract delta_records and action_response, which describe a body with no version field
  digest_tag: rule:update-validator-computation hashing swaps the protocol-version tag for a build-identity tag, so two builds never produce comparable validators; its authority line reads build-mismatched rather than version-mismatched
  precedent: requirement:redraw-cache-policy already varies its keyed ETag on the build header, so the build axis was carrying this weight in one of the three digest surfaces before the version was given up
```
