---
id: requirement:template-server-functions
type: requirement
title: Template Server Functions
---
Let a template name a Go handler as a form action, and generate two entry points to it: a self-posting form route and a hash-addressed route for client code.

```yaml
priority: should
source:
  - concept:filesystem-html-routing
  - user server-function request 2026-07-27
  - user entry-point decision 2026-07-27
review_gate: proposed
model:
  reference: '<form serveraction="Update"> names an exported Go handler instead of a URL string'
  generated: two POST entry points to that handler, plus the real action and method attributes and the hidden fields the form needs
  implementation: ordinary routes built on the existing concept:request-binding path, so no second binding mechanism appears
  inspiration: React 19 server functions, reduced to what a server-rendered form and a small script actually need
why:
  url_strings: a hand-written action URL is an untyped string that no compiler checks against the handler it targets
  two_places: today a form and its handler are edited separately, and a renamed field breaks at runtime
  gap: decision:route-handler-shape serves GET pages, so without this a form has nothing in the route tree to post to
syntax:
  chosen: 'a reserved serveraction attribute on a form element, whose value is a static handler name'
  grammar_cost: none; requirement:html-template-v1 already requires static attribute names, and a literal value needs no expression machinery
  reservation:
    scope: the attribute name is reserved on form only, so it stays an ordinary name everywhere else
    precedent: requirement:html-template-v1 already reserves slot as an element name while leaving slot ordinary as an attribute
    emitted: never; the generator replaces it with the real action and method attributes
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
  open: the exact attribute spelling, against alternatives such as action-server or a tb-prefixed name
resolution:
  location: an exported function in the route package, beside the template that references it
  identity: resolved at generation through rule:go-types-symbol-identity, never by name lookup at runtime
  unresolved: a reference with no matching function is a generation error naming the template position and the symbol
  exposure:
    rule: a handler becomes reachable only when a template references it
    reason: an unreferenced handler in the route package stays ordinary code, so nothing is published by merely existing
    reserved: Page is the page's own function from requirement:colocated-route-logic and is never a server function
signature:
  shape: 'func Update(w http.ResponseWriter, r *http.Request)'
  decided: an ordinary http.HandlerFunc, approved 2026-07-27
  input: the handler calls api:bind itself, exactly as a flat-mode handler does today
  one_function: the same handler serves both entry points below and needs no knowledge of which one was used
  reason:
    - a form action legitimately needs redirects, conditional statuses, downloads, and streaming, which no fixed typed return could cover
    - it matches the rung 3 shape of decision:route-handler-shape, so a route package has one raw handler idiom rather than two
    - it keeps the generator out of the response body, which is where every other decision in this feature already put it
entry_points:
  why_two: a native form submit and a script call have different requirements, and one URL scheme cannot satisfy both without penalising one
  form:
    url: the page's own pattern, so a form on /users/{id} posts to /users/{id}
    method: POST, which is free because decision:route-handler-shape gives the page GET only
    selection: a generated hidden field carrying the handler's hash, since the URL no longer identifies it
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
    caller: generated client code, from an event handler rather than a form submit
    csrf: the request header form of policy:html-update-csrf-protection, because a script can set one
    default_response: none added; the handler's output is the response verbatim
    reason_separate_url: a script needs a stable address it can call without a form and without a page navigation
progressive_enhancement:
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
    behavior: the client runtime may intercept the submit and post with fetch, applying the response to a requirement:partial-update-boundaries boundary
    optional: an optimization over the plain path, never a precondition for it
client_stub:
  purpose: let script code call a handler without writing its URL by hand
  shape: a generated function per exposed handler, with the direct entry point URL already embedded
  gain: the same rename safety the form gets, extended to the call site in script
  caching: the deterministic hash is what lets emitted and cached script stay valid across rebuilds
  status: proposed; shape and delivery are open below
hash:
  input: the normalized route path and the handler name
  form: the first 12 hexadecimal characters of a SHA-256 digest; fixed, not configurable
  prefix: the fixed reserved path /_action; not configurable, because one reserved prefix is enough
  deterministic:
    rule: no build salt, so regeneration reproduces the same value
    gain_form: a page held open across a deploy still submits to a selector the server recognizes
    gain_script: an emitted or cached client stub keeps working across rebuilds
  collision: two handlers hashing alike is a generation error, detectable because every input is known at generation
  no_salt: no configurable salt, because obscurity_not_authorization means an unguessable value would buy no security anyway
obscurity_not_authorization:
  rule: the hash hides structure; it is not a capability token and grants nothing
  consequence: both entry points are publicly reachable, so the handler still performs its own authentication and authorization checks
  csrf: policy:html-update-csrf-protection applies to both
  stated_because: an opaque identifier invites the assumption that guessing it is the attack, which it is not
form_attributes:
  emitted: action and method on the form, replacing the serveraction attribute
  hidden_fields: the handler selector and the CSRF token
  visibility: emitted fields are generated markup, so an author never writes or maintains them
  conflict: an author-written action, or a method other than post, on a form carrying serveraction is a generation error
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
    - checking is therefore best-effort, and reports what it could not verify rather than passing silently
  status: proposed as a should, separable from the rest of this requirement
not_published:
  openapi:
    rule: neither entry point ever enters an OpenAPI document
    reason: OpenAPI describes a published API contract, and these are implementation details of one page
    mechanism: concept:openapi-generation discovers routes from user-written registration call sites, and these are registered by generated code, so exclusion is the natural outcome rather than a filter
    stated_anyway: relying on that side effect would leave the exclusion undefined the moment discovery changes
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
  - the generator writes no response body; the only thing it may add is the form entry point's default redirect
acceptance:
  - a form naming a handler posts to a working endpoint with no URL written anywhere
  - the same form submits correctly with JavaScript disabled, keeps the page URL in the address bar, and survives a reload
  - a handler that writes nothing produces a redirect back to the page; a handler that writes anything produces exactly that
  - a handler calling http.Redirect sends the browser where it chose, not back to the page
  - script calling the generated stub reaches the same handler with no form and no navigation
  - renaming the Go handler fails generation at the template that references it
  - a submitted form without a valid CSRF token is rejected before the handler runs
  - regenerating an unchanged project produces the same hash on both entry points
  - an exported handler no template references is reachable at no URL
  - a generated OpenAPI document contains no entry point and no page route
  - a handler is callable directly from a test with httptest and no registration
open_questions:
  - the attribute spelling recorded in syntax.open
  - the hidden selector field name, and whether it carries the hash alone or the readable handler name too
  - client stub shape and delivery, and whether it is a module, a runtime call, or generated per page
  - how a handler exposes itself for script use when no template form references it
  - how a validation failure returns field errors to the same page without losing user input, given that the handler owns the response
  - whether a handler may be referenced from a template in a different route
```
