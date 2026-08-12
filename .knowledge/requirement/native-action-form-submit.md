---
id: requirement:native-action-form-submit
type: requirement
title: Native Action Form Submit
---
Make a form carrying server-action reach its handler when the browser submits it natively, in every build, by emitting the scripted and the native lowerings together rather than choosing between them.

```yaml
priority: must
status: implemented 2026-08-12, both halves; the markup, the page POST route and its dispatcher, the runtime default on either transport, and an end-to-end fixture submitting natively
as_built:
  markup: templates/htmlbind/emit.go emitServerAction writes method="post" beside the URL attribute, and emitActionFormFields writes the hidden selector once the start tag closes
  token: templates/htmlbind/csrf.go unsafeForm now treats a form carrying server-action as a POST form, which is what makes the token appear at all
  selector_supply: GenerateOptions.ServerActionSelectors, a second name-keyed map beside ServerActions, so the existing two-pass dataflow carries it and no signature moved
  route: routetree registers the page's own POST beside its GET and generates a dispatcher branching on the selector, per Symbols.RoutePostPattern and the actiondispatch template block
  runtime: httpbind.ActionSelector and httpbind.DispatchAction, with fasthttpbind counterparts, because observing whether a handler wrote a response means wrapping the transport's own writer
  tests: templates/htmlbind/action_test.go for the markup, routetree/action_test.go for the registration, action_dispatch_test.go for the default, and internal/pagesfixture for a native submit reaching a handler with its path value
registration_is_narrower_than_designed:
  found: 2026-08-12, while building; neither side named it
  what_broke: registering the page POST for every discovered action collided with internal/pagesfixture/compose_test.go, which registers its own POST on a page pattern and which the framework-owner guide documents as supported; the stdlib panics on a duplicate pattern, so it is a startup crash
  the_rule_now: the page POST is registered only for a handler a template names from a form element, which htmlbind.Result.ActionRefs already reports through its Element field
  why_it_is_right_and_not_a_workaround: a bare button has no native submit channel at all, so a POST registered for it serves nothing while taking an address an application may want; the registration follows the markup that needs it
  cost_that_remains: a page that does declare a form owns its POST, so that one address stops being an application's to take, which requirement:template-server-functions constraints had already reserved and nothing had yet enforced
  compatibility_gained: a project whose actions all sit on bare buttons regenerates byte for byte, which both page fixtures confirm
the_defect_survives_where_a_framework_owns_the_route:
  case: requirement:external-action-resolution, a template outside the route tree whose address a framework's own route table supplied
  behavior: no selector exists for such a name, so the form keeps the scripted attribute alone, stays a GET form, and carries no token
  why_not_fixed_here: this module registers no route for that address and cannot know what method the framework's own route accepts, so emitting a POST form would point a native submit at something it has not been told exists
  the_framework_s_move: supply a selector through the same option, or own the fallback, since it already owns the route
  stated_because: the fix reads as total and is not; one path still emits the GET form this concept exists to remove
the_field_name_is_not_settable_through_the_tree:
  found: 2026-08-12, while documenting
  what: routetree forwards neither CSRFFieldName nor CSRFMode, so a generated tree always emits _csrf and always emits it
  predates_this: an author-written form carrying method=post already reached the same insertion with the same fixed name
  why_it_matters_now: a server action form carries a token where it carried none, so a framework whose middleware expects another name meets the gap for the first time
  the_open_question_it_instantiates: policy:html-update-csrf-protection asks whether an existing middleware's field name forces this to be a deployment setting rather than a generation one; this is that question arriving with a caller behind it
  interim: compile those templates directly rather than through the tree
a_form_makes_every_render_of_that_page_need_a_token:
  found: 2026-08-12, while building
  what: requirement:csrf-token-rendering fails a render that reaches an unsafe form with no token rather than emitting an empty one, and the emitted form is now unsafe by construction
  consequence: adopting a form action turns the page's render option from optional to required, for every render of that page including ones that never submit
  correct_but_unstated: the failure is the designed behavior; what was unstated is that this feature is what triggers it, since before this a server-action form carried no token and so never reached the check
  seen_in: internal/pagesfixture, where adding one form required a token on every render of the tree
source:
  - downstream framework change request 2026-08-11, ask 1
  - decision:client-handler-seams
  - decision:server-action-lowering
review_gate: proposed
defect:
  emitted_today: '<form server-action="Save"> becomes <form data-tb-action="/_action/<hash>/Save">, with no action, no method, no selector, and no token'
  what_that_is: a form declaring no method is a GET form to the current URL, so a native submit puts the fields in the query string and the handler never runs
  measured_worse_than_reported: 2026-08-12 in the browser; the submit replaced the page's own query rather than appending to it, so /users/123?tab=x became /users/123?field=one. The failure is not only that the mutation is skipped: the page's own state is discarded and the user lands on a URL the application never produced.
  csrf_agrees: templates/htmlbind/csrf.go unsafeForm requires a static unsafe method, so the same absence that makes it a GET form also keeps the token out
  not_a_missing_feature: the markup is not inert without a runtime; it is a working form doing the wrong thing, which is why this is a correctness fix rather than a phase
  reported_as_derived: the downstream reader derived it from the lowering rule and had no fixture to observe; templates/htmlbind/emit.go emitServerAction confirms it
  partial_workaround_exists: templates/htmlbind/action.go permits method="post" and rejects every other value, so an author writing it gets a POST form and the token; the selector and the page-pattern route are what remain missing
the_action_attribute_is_deleted_not_emitted:
  finding: a form with no action submits to the document URL, and a POST preserves that URL's query rather than replacing it, so the page pattern is already the target
  measured: 2026-08-12 in the browser, against a probe server, on a page served at /users/123?tab=x
  measured_post: a form with no action and method=post reached POST /users/123?tab=x carrying the fields in the body, so both the path and the page's own query survived
  measured_base: the same held on a document carrying a base element, with that element confirmed active because a relative action on the same page resolved through it; an absent action resolves nothing, so no base can move it
  corrected: an earlier draft claimed an absent action is more correct than action="" because the empty string would resolve against the base element. Measured false: both reached the document URL on a base-carrying page, since the specification defines an empty action as the document URL rather than as a URL to resolve. Either spelling works and the module emits neither, so the claim was decorative and is removed rather than repaired.
  deletes: decision:server-action-lowering form_action_url, a render option carrying the request path and read through renderer state
  why_that_channel_was_expensive: the concrete request path is not held at the typed rung of decision:route-handler-shape, so supplying it meant a render-time channel every project would pay for
emits:
  scripted: the URL-carrying attribute of decision:server-action-lowering, unchanged
  native_on_a_form: method="post", the hidden selector field, and the token policy:html-update-csrf-protection then contributes
  native_action: nothing; see above
  one_compile: both sets from one generation, because the bytes are the same for every client and the runtime's presence is what selects which mechanism drives them
  not_cloaking: no per-request switch and no per-build divergence, which is what decision:script-free-render-mode switch_placement refused and this does not reintroduce
why_exclusivity_falls:
  was: decision:script-free-render-mode lowering_sets kept the two apart because the selector, the page-pattern POST registration, and the render-time path channel are costs the scripted set does not otherwise pay
  now: the path channel is gone, and the remaining two are static markup and one route registration
  the_reporter_s_argument: a link works because it is a link and a runtime makes it faster; every other fallback in that runtime already has this shape
  the_reporter_s_constraint: its acceptance criteria require one build that works with no browser runtime and one that uses it, and a per-build mode makes those two deployments of one application
  mode_survives: decision:script-free-render-mode still turns off the runtime bootstrap, partial updates, and async streaming; this removes one of its five jobs
a_bare_button_stops_being_an_error:
  was: decision:script-free-render-mode authoring_rule made server-action on anything but a form a generation error under that mode
  why_it_must_change: with no mode selecting markup, that rule would apply always, and a bare button is the common shape under today's default; making it an error is a breaking change to the ordinary case
  now: a bare button lowers to the URL attribute alone, works under a runtime, and has no native fallback
  diagnostic: available as an opt-in report naming the position, since only the author knows whether an ancestor form encloses the element; never an error
  portability_note_survives: putting server-action on a form is still what buys the native path, which is guidance rather than a rule
costs:
  page_pattern_post: routetree.Route.Pattern returns GET only today, so every page declaring a form action gains a POST registration
  dispatcher: generated, branching on the selector field, per requirement:template-server-functions entry_points.form.selection
  paid_by_everyone: a project whose clients all run a runtime still carries the selector and the registration, which the reporter accepts explicitly
  not_paid: the render-time request path channel, deleted above
  profile_escape: decision:action-lowering-profile may expose this through the lowering profile instead of the default if a framework wants the narrower output; the reporter needs the markup rather than a particular seam
interactions:
  extra_fields_on_the_direct_entry_point: a scripted post now collects the selector and the token alongside the author's fields; harmless, because nothing in the binder rejects a field the input type does not declare
  field_checking: the proposed control-name check of requirement:template-server-functions must exempt generated fields, or it reports the ones it emitted
  response_meaning: the page pattern answers 303 by default and the direct entry point answers verbatim, so emitting both makes that asymmetry permanent rather than per-mode; requirement:action-response-update owns what a runtime posting to the page URL then receives
  cache_identity: decision:script-free-render-mode cache_identity loses its reason, because one markup is emitted rather than two
acceptance:
  - a form carrying server-action submits natively to its handler with scripting disabled, keeps the page URL in the address bar, and survives a reload
  - the same form carries the URL attribute a runtime binds to, from the same build
  - the emitted form carries a CSRF token, which today it does not
  - a form carrying an author-written action is still a generation error, and method="post" is still the only method accepted
  - a bare button carrying server-action compiles, lowers to the URL attribute, and reports no error
  - one page holding several named handlers dispatches to the submitted one with scripting disabled
  - no generated form carries an action attribute
  - a project referencing no server action regenerates byte for byte
related:
  - requirement:template-server-functions
  - decision:server-action-lowering
  - decision:script-free-render-mode
  - policy:html-update-csrf-protection
  - requirement:external-action-resolution
```
