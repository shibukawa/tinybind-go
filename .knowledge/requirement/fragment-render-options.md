---
id: requirement:fragment-render-options
type: requirement
title: Fragment Render Options On The Update Entries
---
Let the caller pass htmlbind render options to the two update entries that render a fragment, because both render under package defaults the same component never gets on the page it sits in.

```yaml
priority: must
as_built:
  shipped: 2026-08-08
  redraw: 'Options.Redraw(w, r, reg, options ...htmlbind.Option) bool; every existing call still compiles'
  action: 'WriteUpdate(w, r, updates []Update, options ...htmlbind.Option) and WriteUpdateStatus with its status, because a trailing options variadic cannot follow the updates variadic that was there'
  routing: both render through Options.renderOptions, so the boundary prefix and the build-identity validator tag arrive without a caller supplying them
  context: 'htmlbind.WithContext(r.Context()) leads the option list on both paths, so a caller may still override it and a cancelled request stops the work its externals started'
  verified:
    - TestRedrawRendersAnUnsafeFormWhenGivenAToken, which asserts the bare render still fails with a 500 and the token render succeeds
    - TestActionRendersAnUnsafeFormWhenGivenAToken, the same on the path whose documented purpose it is
    - TestRedrawAppliesTheConfiguredURLSchemes, an app scheme neutralised without options and rendered with them
    - TestRedrawStillNeutralisesAHostileScheme, so nothing about this widens the allowlist
    - TestRedrawReachesTheCacheStore and TestRedrawRendersUnderTheRequestContext
  blast_radius: three test call sites and two documentation examples, both languages; go build, go vet, gofmt and the whole test suite are clean
  the_sweep_was_incomplete:
    found: downstream framework report 2026-08-09, which asked why Options.Render still took none
    missed: Options.Render and Options.RenderStream, four sites passing renderOptions(nil)
    why_the_first_sweep_missed_them: it grepped for htmlbind.Render and htmlbind.RenderChain, and these two render through delta.CollectChain and delta.RenderDelta instead
    severity: the worst of the set, since Options.Render is the entry an ordinary page reaches first, so a page containing an unsafe form could not render at all
    fixed: 2026-08-08; every entry takes the variadic and no renderOptions(nil) site remains
    verified: TestEveryRenderEntryTakesRenderOptions, a subtest per entry asserting the bare render fails and the token reaches the markup
    reading: sweeping by the call being made rather than by the surface being fixed leaves the sites that reach it through another package
  found_while_building:
    version_echo_reason_expired:
      what: decision:caller-owned-wire-versioning as_built says the action and redraw paths echo a bare mode name because they take no request
      already_false_for_redraw: Redraw has taken the request since requirement:caller-addressed-redraw, and echoes bare anyway
      now_false_for_both: the action path takes it too
      not_changed: the echo stays bare, because changing it is a wire change this round did not ask for; the stale reason is recorded so the choice can be made deliberately
  deliberately_not_done: the failure body is still plain text; requirement:update-error-hook default_form moves it to problem details and is its own change
source:
  - downstream framework partial transfer report 2026-08-08, against v0.4.2
  - requirement:redraw-cache-policy
  - api:cache-store
review_gate: proposed
reported:
  what: Options.Redraw takes no htmlbind.Option, so a decision:cache-component-declaration component redrawn on its own runs its body every time while the same component is cached on the page around it
  reporter_framing: small, raised only because it was hit while wiring the cache and has no workaround downstream
verified_and_wider_2026_08_08:
  two_entries: redraw.go writeRedraw and action.go WriteUpdateStatus both call htmlbind.Render with the fragment and nothing else
  correction: the report lists the action path among the ones that reach htmlbind through an entry taking options; WriteUpdate and WriteUpdateStatus take none either
  every_option: thirteen options exist and both entries pass zero, so the cache is one instance of the gap rather than the gap
  the_ones_with_options: RenderStreamAsync and RenderLiveStream take the variadic, which is the shape the ask names and the shape these two should match
what_absence_costs:
  csrf_is_not_a_default_it_is_a_failure:
    measured: rendering a plan whose ops are Static, CSRFField, Static with no options returns 'htmlbind: form needs a CSRF token', because CSRFField requires WithCSRFToken or WithoutCSRFToken and neither was supplied
    redraw: the error becomes FailureRenderFailed and a 500, so no component containing an unsafe form can be a redraw endpoint at all
    action: WriteUpdateStatus returns the error before writing, so the caller gets a failure instead of a response
    its_own_documented_purpose: WriteUpdateStatus is documented as the way a failed validation returns 422 and rewrites the form region with its errors, and a form region is exactly what cannot render
    severity: this is a defect rather than a missing capability, and it is what raises this concept from the reporter's should to a must
  url_policy_diverges:
    measured: a url of 'myapp://open/42' in an href renders as '#tb-blocked-url' with no options and unchanged under WithURLSchemes with that scheme added
    consequence: a component renders one way inside its page and another on the redraw or action response that replaces it
    direction: stricter rather than looser, so it is a correctness divergence and not a hole
    contradicts: requirement:url-attribute-scheme-safety acceptance asks the redraw path and the render path to apply the same allowlist so neither is a way around the other; they do not, in the direction that costs an app its own scheme
  boundary_prefix:
    what: a redrawn component owning an await or live boundary writes placeholders under DefaultBoundaryPrefix while the page wrote them under the configured one
    same_shape: requirement:caller-addressed-redraw found_while_building recorded one half of this, where the client read a literal attribute name while the generator wrote the configured one; this is the other half, in generated output rather than in the client
  context:
    what: htmlbind.Render with no WithContext renders under context.Background()
    consequence: request cancellation reaches neither a context-taking external nor a shared cache store on these two paths
    contradicts: api:cache-store gives the leading context argument precisely so a network-backed store honours request cancellation and deadlines
  cache: the reported item, and the smallest of the five
downstream_reproduction_2026_08_08:
  what: the reporter reproduced the CSRF failure against its own action entry with a region holding a form, and got 500 with an application/problem+json body
  its_own_documented_pattern: the reporter's guide documents returning 4xx with the validation-error regions as the worked example, which is the case that cannot render
  no_workaround_downstream: Update carries a Fragment and nothing else, writeActionBody and the delta body types are unexported, and the render happens inside WriteUpdateStatus; the only route around it is re-implementing the action writer against requirement:update-wire-contract, which is the duplication decision:client-runtime-ownership removed
  reading: this is why the variadic is the fix rather than an improvement, and why it belongs in a patch release
blast_radius_measured_downstream:
  url_divergence: does not reach the current caller, which configures no allowlist; it reaches the first application that does, and silently, because the divergence is stricter
  prefix_divergence: does not reach the current caller either, whose prefix is DefaultDataAttributePrefix by an earlier deliberate choice to keep one spelling in a document; an application overriding the prefix finds its redrawn boundaries unaddressable, since the client locates by 'data-<prefix>-id'
  reading: both defects are masked by one caller's configuration choices rather than by anything in the code, so neither would have surfaced from use
ask:
  shape: an options variadic on Options.Redraw and on WriteUpdate and WriteUpdateStatus, matching RenderStreamAsync
  found_while_implementing:
    what: WriteUpdate already ends in 'updates ...Update', so a trailing options variadic is not expressible and the ask is not literally satisfiable
    resolved: updates becomes a slice and options becomes the trailing variadic, which is the RenderStreamAsync shape — payload in slices, options last
    signature: 'WriteUpdate(w, r, updates []Update, options ...htmlbind.Option) error, and the same for WriteUpdateStatus with its status'
    rejected: options as a leading slice with updates left variadic, which keeps the shorter call at the cost of a nil in every call site that wants no options
    redraw_is_unaffected: Redraw takes no variadic today, so adding one leaves every existing call compiling
    blast_radius: three test call sites and two documentation examples
  routing: both entries go through the existing Options.renderOptions helper, so the boundary prefix and the build-identity validator tag stop being absent without a caller supplying them
  source_compatible: a variadic added to an existing signature keeps every call site compiling
settled_2026_08_08:
  no_token_stays_a_failure:
    decided: a render reaching CSRFField with no token supplied keeps failing rather than emitting an empty one
    confirmed_by: the party taking the 500, which still prefers it
    reason: an empty token renders a form that submits, is rejected, and points at nothing, which is the failure mode WithoutCSRFToken's own documentation names
    acknowledged: it is a behaviour change on a released path and therefore a decision rather than a default
  write_update_takes_the_request:
    decided: WriteUpdate and WriteUpdateStatus gain the request, so the context and the session's CSRF token come from where Redraw already has them
    removes: the asymmetry between the two entries, which is the whole shape of this concept
    alternative_offered: the caller passes WithContext and WithCSRFToken through the variadic instead, which the reporter accepts as more typing rather than a design problem
not_asked:
  store_on_options: the store is a caller resource with a caller lifetime and belongs per render, which WithCache's own documentation states; Options is process-scoped configuration and a store there would be the package state that documentation refuses
constraints:
  - a caller passing nothing keeps today's behaviour, so the change is additive rather than a new default
  - rule:redraw-input-trust is unchanged; supplying options changes what a render may do and nothing about the arguments being attacker controlled
  - a caller relaxing the URL allowlist on these paths takes the same responsibility it takes on the page path
acceptance:
  - a cached component redrawn on its own reuses the store the page render used
  - a component containing an unsafe form renders through the redraw and action entries when the caller supplies a token
  - a configured URL scheme renders the same way on the page, on a redraw, and in an action response
  - a redrawn boundary writes placeholders under the configured prefix
  - a redraw under a cancelled request stops the work its externals started
  - a caller supplying no options gets exactly the output v0.4.2 produced
open_questions:
  - whether requirement:redraw-cache-policy open question about the delta and stream paths is answered by the same variadic, since those two already take one
related:
  - requirement:redraw-cache-policy
  - requirement:caller-addressed-redraw
  - requirement:component-redraw-endpoint
  - requirement:action-response-update
  - api:cache-store
  - requirement:url-attribute-scheme-safety
  - decision:partial-transfer-seams
```
