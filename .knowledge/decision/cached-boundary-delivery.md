---
id: decision:cached-boundary-delivery
type: decision
title: Cache Changes The Hit, Not The Semantics
---
Keep delivery a property of the template — inline without an await boundary, fallback with one — and let only a cache hit change it, by making the await synchronous.

```yaml
source:
  - requirement:cached-settled-boundary
  - owner correction 2026-08-14
review_gate: approved 2026-08-14 by the owner
rule:
  without_await: the component renders inline, cached or not
  with_await: the component emits its fallback and settles later, cached or not
  the_annotation_changes_neither: whether output is stored is a deployment question, and delivery is a template question
  the_hit_is_where_it_changes: on a hit the await becomes synchronous — the settled markup is written in place, with no placeholder, no fallback, and no completion frame
why_this_is_not_a_preference:
  existing_principle: requirement:component-output-cache states that without a supplied store the component renders normally, so caching is a deployment choice rather than a template rewrite, and its acceptance list says a render with no store behaves exactly like the same template without the annotation
  what_a_blocking_miss_would_do: make the same template stream in a deployment with no store and wait in a deployment with one
  so: it is not a smaller version of this feature, it is a violation of the rule the feature sits inside
  found_by: the owner, correcting this file's first draft, which had offered it as the cheap candidate and recommended against it on the strength of the user experience alone
shape_of_a_hit:
  equals: what the synchronous render entries already produce, since awaitOp settles in place whenever no coordinator is present
  so_the_stored_value_is: the settled subtree, contiguous, which is the form that path already builds
  nothing_left_to_wait_for: a hit resolves no bindings, so calling it synchronous describes the delivery rather than a call that still happens
implementation_consequence:
  today: execCached builds its buffer renderer without the coordinator, which is what makes a cached subtree settle in place
  required: it has to carry the coordinator instead, or a miss stops streaming and the rule above breaks
  unconditional: a cached component reaching no await boundary has nothing to coordinate, so keeping the coordinator always is simpler than keeping it conditionally and behaves identically there
  storing_the_settled_form:
    problem: with the coordinator present, the buffer holds the placeholder rather than the settled markup
    mechanism: the buffer holds the fence comments this runtime writes around each fallback, and each boundary later yields its settled subtree addressed by the same id, so storing the settled form is replacing each fence span with its content
    not_the_client_apply_logic: the reporter's objection to reassembling downstream is that the wire format is ours; inside the runtime the splice is over bytes this package authored, against ids it issued
    nesting: a boundary inside a settled subtree registers with the same coordinator and settles separately, so the replacement iterates until no fence remains
    timing: after the last boundary of this component settles, which is still inside the response lifetime, because a streamed response is not finished until its boundaries are
    grouping_does_not_exist_yet: asyncCoordinator tracks the whole render's boundaries in one wait group and one result channel, with nothing tying a boundary to the component that owns it, so knowing when one component's set is complete is machinery to build rather than a hook to read
    build_this_first: it is the load-bearing unknown of the change, and the splice is straightforward once a component's settled contents can be collected together
    failure: a boundary that fails stores nothing, so the splice never runs on a partial set
rejected:
  blocking_miss:
    what: keep dropping the coordinator, so a miss resolves the bindings and renders the settled subtree in place with no fallback
    why_not: the deployment-choice violation above; it also drops the first-request fallback, which is the experience the change was asked for
    what_it_looked_like: a one-predicate change, because the hit needs only the generation narrowing; that is what made it worth writing down and refusing explicitly
  double_render:
    what: stream the miss normally and render a second time, blocking, to produce the bytes to store
    why_not: it fetches twice on every miss, which is the cost the cache exists to remove
flag_consequence:
  what: HasAwaitBlock is a plan constant read before rendering by requirement:render-mode-negotiation
  exact_on_a_miss: the boundary really opens, so the flag is right
  conservative_on_a_hit: no boundary opens, so the flag promised something that did not happen
  benign: a streamed response whose boundaries all resolve immediately is a complete document written through the streaming path, which is what a fast page already produces
  but: the flag stops promising that a boundary will open, so anything reading it for more than path selection has to be checked
consequence:
  - the eligibility narrowing at validateCachedComponents is unchanged by this decision; it is the runtime half that this decides
  - api:cache-store is unchanged, because the stored value stays one contiguous range
  - requirement:component-output-cache opaque_unit is preserved, since a settled boundary set stored whole is one unit
```
