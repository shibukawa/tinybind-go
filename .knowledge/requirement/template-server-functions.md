---
id: requirement:template-server-functions
type: requirement
title: Template Server Functions
---
Let a template name a Go handler instead of a URL, and generate two entry points to it: a self-posting form route and a hash-addressed route for client code.

```yaml
priority: should
source:
  - concept:filesystem-html-routing
  - user server-function request 2026-07-27
  - user entry-point decision 2026-07-27
  - user framework-arrangement discussion 2026-07-29
review_gate: proposed
model:
  reference: '<form server-action="Update"> names an exported Go handler instead of a URL string'
  generated: two POST entry points to that handler, plus the markup decision:server-action-lowering writes in place of the attribute
  implementation: ordinary routes built on the existing concept:request-binding path, so no second binding mechanism appears
  inspiration: React 19 server functions, reduced to what a server-rendered form and a small script actually need
why:
  url_strings: a hand-written action URL is an untyped string that no compiler checks against the handler it targets
  two_places: today a form and its handler are edited separately, and a renamed field breaks at runtime
  gap: decision:route-handler-shape serves GET pages, so without this a form has nothing in the route tree to post to
syntax:
  chosen: 'a reserved server-action attribute whose value is a static handler name'
  spelling_and_lowering: decision:server-action-lowering, which also widens the reservation beyond form
  grammar_cost: none; requirement:html-template-v1 already requires static attribute names, and a literal value needs no expression machinery
  reservation:
    scope: the attribute name is reserved wherever a lowering applies, and stays an ordinary name everywhere else
    precedent: requirement:html-template-v1 already reserves slot as an element name while leaving slot ordinary as an attribute
    emitted: never; the generator replaces it with the URL-carrying markup of decision:server-action-lowering
  value_form:
    static: a string literal, because the symbol must resolve at generation
    expression: forbidden; a computed value could not be resolved or type checked
  rejected_expression_sigil:
    shape: '<form action={@Update}>'
    url_context: requirement:html-template-v1 gives action a URL insertion context requiring a url-typed value, so this puts a generation-time entity where a URL value belongs
    collision: decision:template-annotation-syntax already uses @name for declaration annotations; the grammar separates them but one symbol would carry two unrelated meanings
    conclusion: the attribute form avoids both problems and needs no new syntax at all
  rejected_declaration:
    shape: a template-level action declaration restating the Go signature, then referenced by name
    consistency_argument: requirement:template-file-scope has external declarations restate their contract locally
    why_not: requirement:colocated-route-logic already resolves its func Page from the package Go sources without a template declaration, and a server function is found the same way in the same package
resolution:
  location: an exported function in the route package, beside the template that references it
  outside_tree: requirement:external-action-resolution lets a framework supply the URL for a name no route package exports, with the declaring package still winning
  identity: resolved at generation through rule:go-types-symbol-identity, never by name lookup at runtime
  unresolved: a reference with no matching function is a generation error naming the template position and the symbol
  exposure:
    rule: every exported function in the route package with the handler signature gets a direct entry point, whether or not a template references it
    decided: 2026-07-29
    supersedes: an earlier rule publishing a handler only when a template referenced it, on the grounds that nothing should be published by merely existing
    why_safe_here: a route package under decision:html-route-go-package-model exists to serve one route and is imported by nothing but the generated registry, so an exported symbol in it is that route's surface rather than a general API
    why_chosen: it needs no declaration syntax at all, and it makes Go's own visibility rule mean what a reader already expects
    filter: the signature; an exported function of any other shape stays ordinary code
    opt_out: lower-case the function, because generated code in another package cannot reach an unexported symbol and therefore cannot publish it
    reserved: Load is the page's own entry point from decision:route-handler-shape and is never a server function
    cost_accepted: a handler-shaped exported function meant for internal use becomes reachable, so the route table lists every entry point and the documentation must state this plainly
    unchanged: reachable is not authorized; obscurity_not_authorization still requires each handler to check its own caller
    rejected_declaration:
      shape: a compile-time assertion such as `var _ tinybind.ServerAction = Rename`
      buys: intentional exposure, type-checked and discoverable by rule:go-types-symbol-identity
      why_not: it costs a declaration for every action to restate what the package boundary already says
      scope_narrowed:
        when: 2026-08-13, by requirement:typed-server-action
        holds: for this shape, where the signature admits and a declaration would only restate the boundary
        does_not_hold: for a function of an arbitrary signature, where no shape filter can exist and the declaration carries the fact that nothing else can
        written_here_because: a reader of this clause would otherwise apply the rejection to a declaration that is not the one rejected
signature:
  shape: 'func Update(w http.ResponseWriter, r *http.Request)'
  decided: an ordinary http.HandlerFunc, approved 2026-07-27
  input: the handler calls api:bind itself, exactly as a flat-mode handler does today
  binder_gap:
    discovered: implementation 2026-07-29
    reported_cause: binder generation is driven by user-written route registrations, so a handler registered by generated code is never discovered
    actual_cause: found 2026-07-30; rule:request-model-discovery reads every api:bind call site in the package it analyzes and never consults a registration, so nothing was filtering the handler out. No run analyzed the route package at all.
    effect: api:bind inside a server function fails at runtime with no registered binder, so a handler reads its input through the standard library instead
    fix: requirement:action-request-binding, which reports the tree's packages so a run covers them
    status: implemented 2026-07-30; a server function binds a typed request through the binder of its own route package
  one_function: the same handler serves both entry points below and needs no knowledge of which one was used
  reason:
    - a form action legitimately needs redirects, conditional statuses, downloads, and streaming, which no fixed typed return could cover
    - it matches the rung 3 shape of decision:route-handler-shape, so a route package has one raw handler idiom rather than two
    - it keeps the generator out of the response body, which is where every other decision in this feature already put it
  reason_is_about_the_caller:
    found: 2026-08-13, by requirement:typed-server-action
    what: every clause above is true of a caller holding a document waiting for a page, which is what a form is
    consequence: narrowing the caller to a script holding the answer lapses all three at once, which is what admits a second shape rather than refuting this one
    unchanged_here: this shape stays fixed, stays what a form reaches, and stays the one a redirect or a stream needs
entry_points:
  why_two: a native form submit and a script call have different requirements, and one URL scheme cannot satisfy both without penalising one
  which_is_used: decision:script-free-render-mode selects it; the default mode uses the direct entry point for every element, and the script-free mode uses the form entry point
  form:
    used_by: the script-free mode only
    url: the page's own pattern, so a form on /users/{id} posts to /users/{id}
    method: POST, which is free because decision:route-handler-shape gives the page GET only
    selection: a generated selector carrying the handler's hash, since the URL no longer identifies it
    several_forms: each form on a page carries its own selector, so one POST registration serves every handler the page declares and the generated dispatcher branches on that field
    csrf: the hidden token field of policy:html-update-csrf-protection
    path_parameters: already in the URL, so the generator emits no hidden fields for them and api:bind reads them from the path as usual
    default_response:
      rule: 303 redirect back to the same page URL
      why: post-redirect-get, so a reload does not resubmit and the address bar keeps showing the page
      expresses: the redirect lands on a fresh GET, which renders the state the mutation produced
    override:
      rule: if the handler wrote a status, a header, or a body, that response stands and no redirect is added
      mechanism: the generated wrapper observes whether the handler wrote anything, so the handler needs no flag and no framework type
      covers: redirecting elsewhere with http.Redirect, rendering the page inline with validation errors, or returning any other status
  direct:
    url: '/_action/<hash>/<HandlerName>'
    example: /_action/9f3c2ab1e4d7/Update
    caller: client code, from an event handler rather than a form submit; usually a framework runtime bound to the attribute decision:server-action-lowering emitted
    csrf: the request header form of policy:html-update-csrf-protection, because a script can set one
    default_response: none added; the handler's output is the response verbatim
    reason_separate_url: a script needs a stable address it can call without a form and without a page navigation
    static: the URL holds no path parameter, so the lowering can write it as a compile-time constant
progressive_enhancement:
  status: a property of decision:script-free-render-mode, not of the default
  demoted:
    was: the baseline this requirement was designed around, with the scripted path an optimization over it
    now: an opt-in mode, decided 2026-07-29
    why: a native submit is a full navigation, so scroll position, focus, and client state reset on every mutation
    recorded_because: the argument below is not refuted, only outweighed by that cost
  works_without_script:
    mechanism: the form is a real form with a real action, a real method, and a hidden selector, so a native submit reaches the handler
    address_bar: correct throughout, because the post target is the page itself
    reload: safe, because the default response is a redirect
    conclusion: the plain path needs no compromise, which is why the form entry point posts to the page rather than to the hash URL
  next_js_comparison:
    same: posting to the page URL and identifying the action through a hidden field is what React does for a server-component form
    different_hash: React action ids are non-deterministic per build, so a page held open across a deploy can submit to an id the server no longer knows
    our_choice: a deterministic hash, so that failure mode does not exist
  enhanced_path:
    behavior: the client runtime posts with fetch and applies the response to a requirement:partial-update-boundaries boundary
    default: this is what the default mode does; the plain path above is what selecting the script-free mode buys back
    input_preserved: the DOM is never replaced, so a validation failure loses nothing the user typed and needs no value refilling
    encoding: api:bind already accepts JSON, urlencoded, and multipart for one input struct, so a runtime chooses the encoding and the Go handler is unchanged
    ownership: what the runtime collects and where it writes the response is the framework's, per decision:action-lowering-profile
client_stub:
  purpose: let script code call a handler without writing its URL by hand
  shape: a generated function per exposed handler, with the direct entry point URL already embedded
  gain: the same rename safety the form gets, extended to the call site in script
  caching: the deterministic hash is what lets emitted and cached script stay valid across rebuilds
  narrowed: decision:server-action-lowering already writes the URL into the element, so a stub is needed only for script that no template element references
  supply: every exported handler in the route package has an entry point under the exposure rule above, so a stub or a published URL can be emitted for all of them without any declaration
  status: proposed; shape and delivery are open below
hash:
  input: the declaring package's route-relative directory and the handler name
  input_correction:
    was: the normalized route path and the handler name
    discovered: implementation 2026-07-29
    why: a layout is compiled once but renders under every page below it, so a route path would give one handler a different URL per page and break the caching the deterministic hash exists for
    same_for_pages: a page's directory and its route path are derived from each other, so nothing changes for the ordinary case
  form: the first 12 hexadecimal characters of a SHA-256 digest; fixed, not configurable
  excludes_prefix: the mount prefix is not hashed, so remounting an application changes the URL without changing the identity underneath
  deterministic:
    rule: no build salt, so regeneration reproduces the same value
    gain_form: a page held open across a deploy still submits to a selector the server recognizes
    gain_script: an emitted or cached client stub keeps working across rebuilds
  collision: two handlers hashing alike is a generation error, detectable because every input is known at generation
  no_salt: no configurable salt, because obscurity_not_authorization means an unguessable value would buy no security anyway
prefix:
  default: /_action
  configurable: yes, decided 2026-07-29, so a framework mounting under a sub-path or owning its own URL namespace is not blocked
  supersedes: an earlier rule fixing the prefix on the grounds that one reserved prefix is enough
  form: a literal path prefix; a prefix containing a dynamic segment is a configuration error
  shadow_check:
    lost: the default was safe for free, because route discovery ignores directories beginning with an underscore and can never produce /_action
    required: a configured prefix has no such guarantee, so generation must reject one that shadows a discovered route pattern
  profile: decision:action-lowering-profile carries the setting
selector:
  value: one opaque string spelled as the hash and the handler name together, matching the tail of the direct entry point URL
  readable_name: included, decided 2026-07-29, so a reader of the DOM or of a network trace sees which Go function runs
  authority: the whole string is compared as one key, so no mismatch between a hash and a name is representable
  disclosure: naming the function changes no security property, because obscurity_not_authorization already grants the hash no authority
obscurity_not_authorization:
  rule: the hash hides structure; it is not a capability token and grants nothing
  consequence: both entry points are publicly reachable, so the handler still performs its own authentication and authorization checks
  csrf: policy:html-update-csrf-protection applies to both
  stated_because: an opaque identifier invites the assumption that guessing it is the attack, which it is not
emitted_markup:
  owner: decision:server-action-lowering, which states what each element kind lowers to
  names: chosen by decision:action-lowering-profile, so a framework can target an existing client library
  visibility: emitted attributes and fields are generated markup, so an author never writes or maintains them
  untouched: every attribute other than server-action survives to the output unread
field_checking:
  goal: check each form control name against a field of the input type the handler binds
  input_type_recovery:
    problem: an http.HandlerFunc signature names no input type, so the type cannot come from the declaration
    mechanism: concept:handler-discovery and rule:request-model-discovery already recover a request model by finding the api:bind call inside a handler body, which is this project's core analysis
    result: the typed check survives the handler shape, using machinery that already exists rather than new machinery
  gain: a renamed Go field fails generation at the form that feeds it, which is the main reason to name the handler instead of a URL
  limits:
    - a handler that never calls api:bind offers no type to check against, so checking is skipped and reported
    - a control produced inside a loop or a condition may not be statically attributable
    - a payload a client runtime assembles is not visible in the template, so only the form lowerings of decision:server-action-lowering can be checked
    - checking is therefore best-effort, and reports what it could not verify rather than passing silently
  status: proposed as a should, separable from the rest of this requirement
not_published:
  openapi:
    rule: neither entry point ever enters an OpenAPI document
    reason: OpenAPI describes a published API contract, and these are implementation details of one page
    mechanism: rule:generated-source-not-discovered; route discovery skips the generated registry, so the only registrations of a page or an endpoint are invisible to it
    was_believed: exclusion followed from a handler registered by generated code never being discovered
    corrected: 2026-07-30; the registry itself is a discoverable call site, so analyzing the route root did put every page route and action endpoint into a document until the filter above was added
    stated_anyway: relying on a side effect would have left the exclusion undefined, which is exactly how it broke
  page_routes: the same reasoning excludes the GET page routes of the route tree, because an HTML page is not an API surface
  framework_override: a downstream framework that wants either documented adds it through its own artifacts, per decision:route-feature-ownership
non_goals:
  - a general remote procedure call surface
  - replacing ordinary API routes for JSON clients, which the existing flat path already serves
  - a documented, versioned, or externally addressable endpoint
constraints:
  - no reflection; symbol resolution, hashing, and URL derivation happen at generation
  - the handler is an ordinary http.HandlerFunc in an ordinary package, so gopls, go test, httptest, and linters apply with no wrapper
  - the page pattern carries GET for the page and POST for the form entry point, and no other method
  - a generated entry point never collides with a page route, and a collision is a startup error
  - the generator writes no response body; the only thing it may add is the form entry point's default redirect. Scoped 2026-08-13 to this shape: requirement:typed-server-action moves the line for a declared function, whose glue encodes a result
acceptance:
  - a form naming a handler posts to a working endpoint with no URL written anywhere
  - an element other than a form naming a handler carries its endpoint URL and every other attribute it was written with
  - one form holding several named handlers dispatches to the clicked one with JavaScript disabled
  - the same form submits correctly with JavaScript disabled, keeps the page URL in the address bar, and survives a reload
  - a handler that writes nothing produces a redirect back to the page; a handler that writes anything produces exactly that
  - a handler calling http.Redirect sends the browser where it chose, not back to the page
  - script calling the generated stub reaches the same handler with no form and no navigation
  - renaming the Go handler fails generation at the template that references it
  - a submitted form without a valid CSRF token is rejected before the handler runs
  - regenerating an unchanged project produces the same hash on both entry points
  - an exported handler no template references is still reachable at its direct entry point
  - an unexported handler is reachable at no URL and is named by no generated artifact
  - a generated OpenAPI document contains no entry point and no page route
  - a handler is callable directly from a test with httptest and no registration
open_questions:
  - client stub shape and delivery, and whether it is a module, a runtime call, or generated per page
  - how a validation failure returns field errors to the same page without losing user input, given that the handler owns the response
  - whether a handler may be referenced from a template in a different route
```
