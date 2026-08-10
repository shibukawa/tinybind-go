---
id: requirement:live-mode-token-contract
type: requirement
title: Live Mode Token Contract
---
Make one live token mean one body, emit the handoff marker that says whether a live connection is expected, and make the guide describe what the code does rather than what the design planned.

```yaml
priority: must
status: delivered 2026-08-03; see as_built
as_built:
  protocol_version:
    kept_at_1: the mode spellings and the record framing are what the version identifies, so this would have been a bump; nothing has been released under it yet, so there is no client holding a page rendered by the older shape and nothing for a bump to protect
    decided: 2026-08-03 by the user, before release
    if_it_had_shipped: a version 1 client falls back to a complete document and arrives holding the new runtime, which is the mechanism that would have made settling this late cheap rather than a coordinated deploy; it is still there for the first bump that has someone to protect
  token: ModeLive parses and echoes 'live;v=N'; Negotiate resolves navigation and live and still resolves everything else to a document
  entries: RenderLiveStream serves document, navigation, and live from one chain and keeps subscriptions open only in the live mode; RenderStreamAsync serves the first two and answers a live request as a terminated navigation
  defect_fixed_on_the_way: RenderLiveStream previously set WithLiveSubscriptions unconditionally, so an ordinary navigation delta to a live route would never have terminated
  validators:
    answer: a delivery carries none and the opening delta does
    why: a live delivery is a region the server already decided to repaint, so there is nothing to compare; deliveries travel as await-completion records, which never carried a frame
    also: a live stream no longer restates unchanged boundaries, and the client merges into its manifest instead of rebuilding it, so a client carrying no validators at all loses nothing
  handoff_marker: response header '<prefix>-Live: 1', a 'live' field on the buffered delta body, and an 'end' record whose reason is live_pending; all three absent when the composition owns no live boundary
  terminator_reasons: final, live_pending, failed, done, retry, per rule:stream-termination-marker; retry carries the optional retryMs server hint that rule left deferred
  open_record: the head record carries the build, which is the reporter's open record merged into the record that already opens every stream
  client_policy: the live entry sends the live token, resets the attempt count on a healthy close, backs off exponentially with jitter on a fault, reloads on a build change, and stops without retrying when the route is not served live
  not_built: the requirement:live-boundary-lifecycle bounds that would make the server emit a retry record on its own; Retry is the seam and nothing calls it yet
entry_unused_downstream_2026_08_10:
  reported: downstream framework survey 2026-08-10, explicitly not as an ask
  fact: the reporting framework delivers live boundaries on both transports without calling RenderLiveStream, having tried it on the second backend and withdrawn it
  reason: its net/http half does not call it either, because it layers admission control, a watchdog, digest suppression seeded from the client manifest, a boundary bound and render telemetry on top, none of which this entry has an equivalent for
  what_calling_it_would_have_cost: a poorer stream on one transport than the other, with nothing in the response able to report the difference
  their_answer: their own protocol — close reasons, watchdog, admission, keyed digest, manifest parse, record writers — moved into a leaf both their halves read, which is the move made here for the error types and then for the update types of decision:update-core-shared-leaf
  reading_for_this_module: the entry ports cleanly and is still not what a framework serving live at scale calls; the settle items above are what would change that, and none of them is what the reporter withdrew over
  nothing_asked: the reporter needs nothing here for it
source:
  - downstream framework composition seam report 2026-08-02, against v0.3.1
  - requirement:live-reconnect open questions
  - decision:live-transport-boundary
review_gate: proposed protocol surface requires user approval
three_states_that_disagree:
  guide: the mode table publishes 'X-Tinybind-Render: live;v=N' and the availability table marks live delivery and reconnection available
  go: no live token is parsed; the mode constants are navigation, action, and redraw
  client: the shipped runtime's live entry sends 'navigation;v=N', so a live connection is a navigation-mode record stream
  design: decision:live-transport-boundary specifies a live mode on the page's own route, and requirement:live-reconnect still lists the spelling and the body as open while its status says delivered
  consequence: a downstream built against the published table and arrived at the same token by a route this catalog did not intend, which is how the defect surfaced
what_is_actually_shipped:
  document: a live boundary settles in place through the blocking op, so the document response commits first content and finishes
  deliveries: a second connection in navigation mode carries a record stream held open for the life of the subscriptions
  reading: this is decision:live-transport-boundary chosen.document_mode and first_delivery_inline, built; only the mode that carries the deliveries was never given its own name
  so: the work is naming and finishing rather than designing
settle:
  token:
    question: whether the deliveries connection keeps the navigation token or takes a live one
    for_a_live_token: a live connection is not a navigation; it has no target URL change, it never ends on its own, and requirement:render-mode-negotiation logs the served mode, so a live stream held open for hours is indistinguishable from navigation traffic today
    for_the_navigation_token: it is what ships, and the body is already the delta record stream
    recommendation: a live token, because the two differ in duration and in termination and a caller cannot route, time out, or bound them separately while they share a name
  body:
    question: requirement:live-reconnect asks whether reconnection reuses the navigation delta body or a delivery stream
    answer_from_the_code: the delta record stream already carries both, since a delivery is written as a boundary operation with its frame validator
    remaining: whether a live body carries validators at all; the downstream carries none, and this module writes one per record
    recommendation: keep the record shape and make validators optional on the live path, so a client that skips unchanged non-live boundaries can and one that does not need not
  handoff_marker:
    what: rule:stream-termination-marker specifies a terminal record naming whether more work exists, so a client knows whether to open a live connection at all
    shipped: the terminator exists in the delta stream; nothing names what follows, and no document response carries one
    cost_of_absence: a page with no live boundary either opens a speculative connection or the caller hardcodes which pages are live
    ask: emit it, so a page with no live boundary costs no speculative request
  control_vocabulary:
    offered_by_the_reporter: a stream closed because every source finished, distinguished from one closed healthy at a lifetime bound with a retry hint, plus an open record carrying the build
    here_today: end, navigate, and error
    reading: the distinction is real; this module's end record cannot say whether a reconnect is wanted, and the client infers it from whether the terminator arrived, which conflates two different healthy closes
  backoff:
    here: linear, multiplied by the attempt count, reloading at a fixed cap
    offered: exponential with jitter, resetting the count on a healthy close
    why_the_reset_matters: a working screen closed at its own lifetime bound currently spends an attempt, so a long-lived screen walks its own budget down to a reload
    status: client policy, so it is a default rather than a contract; the reset is a defect fix and the jitter is a thundering-herd fix
documentation_defect:
  ask: make the table say what the code does, or wire the token up
  either_way: the availability table must stop marking a mode available that no code implements
  scope: docs/httpbind_reloadable_componet.md and its Japanese mirror, plus the status lines named in decision:update-composition-seams
why_now:
  reporter_argument: a token is a wire contract, and settling it after either side ships costs a coordinated deploy
  strengthened_here: the build identity exists precisely to avoid coordinated deploys, so a wire contract settled late spends the mechanism the module built to avoid spending
constraints:
  - a page with no live boundary is byte-identical and issues no extra request
  - an unrecognized token still resolves to a complete document, which is what made the downstream's own token inert rather than an error
  - a live connection stays an ordinary authenticated request to the page's own route, so authorization is the page's check, per decision:live-transport-boundary
acceptance:
  - the live token means one body, whichever side implements it
  - the guide's mode table and availability table describe what the code does
  - a page with no live boundary opens no live connection
  - a client can tell a finished stream from a healthy close at a lifetime bound without inferring it
  - a long-lived screen closed at its bound does not spend its reconnect budget
related:
  - requirement:render-mode-negotiation
  - requirement:live-boundary-lifecycle
  - requirement:streaming-delta-response
  - decision:update-protocol-version
open_questions:
  - whether a caller-owned token space is worth publishing as a fallback, which the reporter offers if convergence fails
resolved:
  live_version_axis: decision:caller-owned-wire-versioning removes the module-owned version entirely, so there is no axis for the live token to need its own of; the caller decides whether its live body versions separately
  reads_back_on: as_built.protocol_version, whose kept_at_1 reasoning was that nothing had shipped yet; the same absence of a client to protect is what made giving the version up cheap two days later
```
