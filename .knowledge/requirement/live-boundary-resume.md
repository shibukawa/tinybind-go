---
id: requirement:live-boundary-resume
type: requirement
title: Selective Live Boundary Resume
---
Re-establish live delivery after a dropped connection by re-requesting the same page in live mode, transferring only the live boundaries' renders and leaving every other region of the document untouched.

```yaml
priority: should
source:
  - concept:live-boundary-updates
  - user reconnect discussion 2026-07-30
review_gate: proposed
status: not implemented; the requirement:component-delta-rendering blocker cleared when that shipped, and nothing re-read this line, per decision:update-composition-seams headline_finding
transport: decision:live-transport-boundary
mode: decision:response-mode-header
request: data:live-boundary-subscription
runtime_flow: flow:live-boundary-stream
same_path:
  requirement: the first live request and every reconnect are the same request to the same route in the same mode
  reason: a reconnect is the common case over the life of a screen, so it must not be a second mechanism that can drift from the first
  consequence: the server holds no per-client subscription state a reconnect has to find, and the client sends no capability to prove what it may watch
  authorization: the page's own check runs again, because the page itself runs again
selectivity:
  transferred: only live boundary deliveries
  untouched: the body render is executed and not transferred, so a static region, a settled await boundary, and every ancestor receive no operation and cannot repaint
  reason: a dashboard whose one live chart reconnects must not repaint the panels around it; skipping the body transfer achieves that without the client naming what to preserve
  narrowing: a page with several live boundaries subscribes to all of them by default; addressing a subset is an optimization, not the mechanism
reconstruction:
  by_execution: the route handler, layouts, and page run again for the same URL and credentials, so live bindings see the arguments they saw before
  no_continuation: no capability token, no server-side input rebuild, and no client-held arguments, per decision:live-transport-boundary execution_is_the_reconstruction
  identity: rule:component-instance-identity is deterministic across executions, so boundary IDs match what is on screen with nothing sent to align them
  changed_inputs: a page whose data changed since the document render produces a different first delivery, which is correct rather than a mismatch; the client applies it like any other delivery
  cost: the page's work runs again per reconnect, which requirement:live-boundary-lifecycle counts and requirement:live-mode-plan-slice proposes to narrow
  identity_depends_on_it: a sliced live-mode render must allocate the same positional ids the document render did, so the id rule above is a constraint on that optimization rather than a consequence of it
stateless_v1:
  decided: the live-mode request carries no per-boundary revision or validator in the first milestone
  consequence: every restarted subscription sends its first delivery, even when it matches what the client displays
  why_acceptable: a live region's value has usually changed during the outage, so the redundant transfer is one render of one region
  deferred: an optional header carrying instance revisions and content validators, which would let the server suppress an unchanged first delivery and let a client narrow the subscribed set
  placement_note: that state cannot travel in the URL, because the page must execute as its own URL; a header or a request body is the only place for it
missed_deliveries:
  behavior: deliveries produced while the client was disconnected are not queued and not replayed
  correct_by_construction: a snapshot delivery makes the newest render sufficient, so a gap costs freshness during the outage and nothing after it
  contrast: an append-only source would need a cursor and a server-held backlog, which decision:live-boundary-syntax deferred along with the append clause
  visible_cost: a chat list reconnecting shows the current list, so nothing is lost, but no arrival animation for the missed messages is possible
unexpected_identifier:
  decided: a reconnect that produces a boundary id the client does not hold is a structural change, and the client stops rather than reconciling, approved 2026-07-31
  premise: a reconnect re-executes the same page and its ids are positional, so the same structure produces the same ids; a different id means the structure itself changed
  example: a panel added to a dashboard the user has been watching, so the render now holds a boundary the screen never had
  client_action:
    - stop the update connection rather than applying anything
    - tell the user to reload; a plain alert is an acceptable first implementation, because the case is rare and correctness matters more than polish here
  why_not_reconcile: inserting a boundary correctly means placing it in the right position in a document the client did not render, which is the navigation delta problem rather than the resume problem; doing it badly puts a panel in the wrong place
  why_not_silent: quietly ignoring the unknown id leaves the user watching a screen that is missing a panel the server thinks is there, with no signal that anything is wrong
  server_side: nothing special is required; the server renders what the page renders, and the client compares against what it holds
failure_of_the_reconnect:
  incompatible_render_version:
    behaviour: the response instructs a full reload, because a deploy changed the generated code behind every boundary ID
    worst_case: a rolling deploy turns every open screen into a full document load, which costs more than a reconnect, so deployment is the heaviest event this design has
    open: whether compatibility can be judged per boundary rather than per generated version, which would let most screens keep their subscriptions across a deploy
  boundary_gone: a live boundary the page no longer produces simply yields nothing; the client drops that region from its live set rather than retrying
  handler_error: an error before any delivery keeps its ordinary status, because nothing is committed
  redirect: a requirement:redirect-error from the page becomes a navigate instruction, per decision:response-mode-header, never a 3xx the fetch would follow into a body
  no_partial_authority: a reconnect that fails authorization is an ordinary failed page request, so there is no per-boundary rejection to report
ordering:
  monotonic: a boundary's revision continues to increase across reconnects, so a late delivery from the previous response cannot be applied after a newer one
  discard: the client applies a delivery only when its base revision matches the boundary's current state, per rule:live-boundary-delivery
duplicate_responses:
  case: a client whose previous live response has not been torn down server-side issues a new one
  behavior: both are valid; the revision rule makes the older response's deliveries unapplicable rather than harmful
  cost: the old execution holds resources until its context cancels, which requirement:live-boundary-lifecycle bounds
when_to_reconnect:
  from_document: the document response's rule:stream-termination-marker marker says whether a live-mode request is expected, so a page with no live boundary triggers none
  truncated_document: no terminal marker while document.readyState is no longer loading means the document was cut off; recovery is a navigation-mode refresh first, because a live-mode request repairs neither a stranded await fallback nor lost body content
  from_live: a retry close, or a stream that ended with no terminal record, is a reconnect; a done close is not
  never_from_transport: load, DOMContentLoaded, readyState, and a resolved fetch reader are not completion signals on their own
reconnect_policy:
  owner: the caller's client script, per decision:live-transport-boundary
  module_contract: the mode, the request shape, the revision rule, the terminal records of rule:stream-termination-marker, and the reload instruction
  expectation: backoff, an attempt cap, and pausing a hidden tab are client policy the module does not dictate
acceptance:
  - a dropped connection is re-established by re-requesting the same URL in live mode, with no separate endpoint involved
  - a dashboard with one live region and several static ones transfers nothing for the static ones on reconnect
  - boundary IDs from a reconnect address the same DOM nodes the document render created, with no token sent to align them
  - a delivery from the previous response cannot overwrite content applied after the reconnect
  - a generated-version change across a deploy produces a reload instruction rather than misapplied operations
  - a live-mode request without the mode header returns the ordinary full document
  - a document whose render produced no live boundary triggers no live-mode request, so it costs no speculative page execution
  - a truncated document is detected from the missing terminal marker and recovered without waiting for a timeout
open_questions:
  - whether the deferred revision header ships together with narrowing the subscribed set, since both need the same transport
  - whether a client that reconnects after a long outage should prefer a navigation-mode refresh over a live-mode reconnect, given a truncated document already prefers one
  - whether a server-suggested reconnect delay is worth carrying, or backoff stays entirely client policy
```
