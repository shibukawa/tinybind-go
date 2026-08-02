---
id: requirement:fragment-response-head
type: requirement
title: Head On Redraw And Action Responses
---
Carry the merged head on the redraw and action responses too, or state that the caller guarantees asset presence and give it the means to know, because a component appearing for the first time in either one renders unstyled.

```yaml
priority: must
source:
  - downstream framework composition seam report 2026-08-02, against v0.3.1
  - requirement:delta-head-sync
review_gate: proposed
what_works:
  navigation: the delta body carries the merged head, and the client installs the tags and waits for stylesheets before applying markup
  reason_it_exists: a component reachable for the first time on a new route brings link and script tags the live document head does not have, and its markup landing first flashes unstyled
what_does_not:
  action: the response body is built from operations alone; no head field is set
  redraw: the handler writes a rendered subtree and response headers, and the body has no place for a head at all
  affects: requirement:component-redraw-endpoint and requirement:action-response-update, both of which address a region by an author-written id
  consequence: swapping in a component whose stylesheet or script is not already on the page renders it unstyled, which is exactly the failure the navigation path added the field to prevent
why_it_was_missed:
  navigation_changes_composition: a route change makes a new component reachable, so the need is obvious there
  redraw_and_action_look_local: both rewrite a region that was already on the page, so the component looks like one the document already carried
  when_it_is_not: an action may render a region that was not there before, such as a validation summary or a newly revealed panel, and a redraw may render a variant of a component whose other branch was never taken
asymmetry:
  action_is_cheap: the action body is the same data:component-delta-response the navigation delta builds, so it is one field and one client branch that already exists
  redraw_is_not: the redraw response is a subtree with no envelope, so carrying a head means either an envelope, a header, or a second request
  reading: the two look like one ask and are two, and only one of them is small
options_for_the_redraw_path:
  envelope: wrap the subtree, which changes the response shape a client already parses and loses the plain-fragment property that makes the endpoint testable with curl
  header: carry the merged head in a response header, which is bounded and would need the decision:manifest-state-ownership size treatment
  guarantee_and_report: state that a redrawable component's assets must already be on the page, and give the caller the means to know, which is requirement:component-asset-requirements static required set
  recommendation: the last one, because the module already owes that accessor and the other two add a channel that exists only for this endpoint
caller_workaround_today:
  what: ensure every asset a redrawable or action-rewritten component needs is already on the page
  cost: nothing checks it and nothing reports it, so the failure is a browser observation rather than a build or startup failure
  reading: this is the requirement:component-asset-requirements fragment_path_honesty rule, which already says a response that cannot deliver an asset reports it instead of dropping it, applied to two endpoints that predate it
constraints:
  - a component whose assets are already present must produce byte-identical responses, so the field is absent rather than empty when there is nothing to say
  - the head form stays the requirement:head-merging output, so a client installs it with the code it already has
  - nothing is fetched mid-swap; the point is that the document already carries what a later swap needs
acceptance:
  - an action response that reveals a component for the first time installs its assets before its markup lands
  - a redraw of a component whose assets are absent is reported rather than rendered unstyled
  - a caller can read a chain's required assets before rendering starts
  - a response whose assets are all present is unchanged
as_built:
  action_half_shipped: 2026-08-02
  what: WriteUpdateStatus collects each written fragment's head, dropping later duplicates by the htmlbind.MergeHead rule, and the field is omitted when there is nothing to say
  why_it_was_one_field: the client already installed head on this path; apply calls syncHead before applying operations, so only the server was never filling it
  byte_identical: a component contributing no head produces the response it produced before
  redraw_half_open: it needs one of the three options above, and the recommendation is requirement:component-asset-requirements rather than a fourth channel built for one endpoint
related:
  - requirement:component-asset-requirements
  - requirement:component-redraw-endpoint
  - requirement:action-response-update
  - requirement:head-contribution-provenance
open_questions:
  - whether an action response should carry the head unconditionally or only for a component the client has not seen, which needs the client to say what it holds
```
