---
id: requirement:csrf-token-rendering
type: requirement
title: CSRF Token Rendering
---
Put the session's CSRF token everywhere a request can carry it, without an author writing anything, by taking the token as a render option and emitting the hidden field and the request header from generated code.

```yaml
priority: should
status: delivered 2026-08-03; cache exclusion approved by the user, who observed that splitting a cached list from an uncached form is the composition a project wants anyway
source:
  - user CSRF specification 2026-08-03
  - policy:html-update-csrf-protection open_questions
  - requirement:builtin-element-registration, whose worked example this was
supersedes_as_the_worked_example: requirement:render-value-provider csrf-token; the seam stays and CSRF stops being what motivates it
problem_with_the_element_form:
  authoring: a security control an author has to remember to write is one an author will forget, and the omission renders a working page that fails only on submission
  plumbing: the element form makes every framework resolve a provider symbol, name its result type, thread a context, and agree on an element name, for a value that is one string
  interop: two frameworks on one module each invent their own element name and their own provider, so nothing about CSRF is portable between them
  reading: CSRF is not a framework extension. It is a property of every unsafe form the module renders, which is what makes it the module's to emit rather than a seam for someone else to fill
division:
  module_owns:
    - emitting the hidden field into every unsafe form, with no author action
    - emitting the request header from the browser runtime on every update it issues
    - refusing to render an unsafe form when no token was supplied
    - the field name, the header name, and their configurability
    - reading the token back out of a request, header first and body second
  framework_owns:
    - creating the token, storing it in the session, and destroying it on logout or session regeneration
    - the session cookie and its SameSite, Secure, and HttpOnly attributes
    - wiring the verification into middleware and choosing the failure response
    - Origin and Fetch Metadata validation, which is a transport check on requests the module never sees
  why_here: htmlbind has no session, no cookie, and no net/http, and htmlupdate has no session either; a module that grew one would be claiming the largest thing it has so far declined
is_the_token_still_needed:
  question: if cross-site requests are rejected by Origin and Fetch Metadata, and cookies are SameSite=Lax, what does a token add
  honest_answer: for a single-origin deployment with strict Origin equality and no sibling subdomains, not much; Go 1.25 shipped http.CrossOriginProtection on exactly that reading
  where_origin_defenses_do_not_reach:
    sibling_subdomain:
      fact: SameSite is computed on the registrable domain, so blog.example.com to app.example.com is same-site and the cookie is sent
      fetch_metadata: that request carries Sec-Fetch-Site same-site, so a rule that rejects only cross-site lets it through
      only_caught_by: strict Origin equality, or the token
      reachable_by: subdomain takeover, a user-content subdomain, a compromised sibling app, shared hosting
    absent_origin:
      fact: a deployment that must accept requests carrying neither Origin nor Fetch Metadata has to choose between rejecting real clients and allowing forged ones
      token: settles it without the choice, because the token is present or it is not
    proxy_rewriting: an intermediary that rewrites or strips Origin removes the whole origin defense silently, with no failing request to notice
    credentialed_cors_misconfiguration: reflecting an arbitrary origin with credentials disables the origin check itself, and the token is the layer that does not share that failure
  independence: the value is not that either check is weak, it is that the two fail for unrelated reasons; every origin-based defense is one proxy or CORS setting away from being off at once
  cost_is_not_zero: the cache exclusion below is real, so this cannot be an unconditional behavior a deployment has no way to decline
  decision:
    default: on, because it fails safe and an automatic token costs an author nothing
    switchable: a generation option turns it off for a deployment that has settled on origin checks alone and wants its cached form-bearing components back
    not_switchable_per_form: the whole point is that an author cannot forget, so the choice is a deployment's rather than a template's
    what_the_switch_turns_off:
      is: the token — the hidden field, the runtime header, and the per-request marking that excludes a form-bearing component from a cache
      is_not: origin or Fetch Metadata validation, which this module never performs and therefore cannot switch
      why: those are checks on an inbound request before any render, so they belong to middleware; a deployment turns them off by not wrapping its handlers, and Go's own http.CrossOriginProtection is what it wraps them with
no_cookie_deployments:
  shape: no session cookie at all, with the credential held in script and sent explicitly on every fetch
  csrf: genuinely not a risk, because a cross-site form cannot set an Authorization header and nothing is attached ambiently
  but_it_does_not_raise_xss_resistance:
    claim_examined: that holding the credential in script rather than in a cookie is safer overall
    finding: it is worse, and the direction is the opposite of the intuition
    httponly_cookie: script cannot read it, so XSS can act as the user for as long as the page lives but cannot take the credential anywhere
    script_held_token: script can read it, so XSS exfiltrates a portable credential that works from outside the browser and outlives the tab
    reading: this trades a risk with good structural defenses for one whose only defense is not having the bug
  no_js_cost: a plain form cannot set a header, so a no-cookie design cannot authenticate a form submission without script; that contradicts the reason the hidden field exists in the first place
  what_actually_raises_xss_resistance:
    - rule:template-context-safety contextual escaping, which is this module's core property rather than a deployment choice
    - a Content Security Policy with no inline script, which this module already permits because no path it owns writes any
    - HttpOnly on the session cookie
  the_token_is_readable_and_that_is_fine: XSS can read a CSRF token out of the DOM, but XSS can already act as the user, so the token was never the thing protecting against it; policy:html-update-csrf-protection says the same
lifecycle:
  create: at login, or at session creation for a deployment protecting pre-login forms
  store: in the session, one token per session
  destroy: at logout and at session regeneration
  rotation: not per request; a per-request token buys nothing here and costs multi-tab correctness, because a second tab's form carries a token the first tab's submission already replaced
  stability_is_load_bearing: requirement:render-value-provider stability already requires one value per session, and this makes it structural rather than a contract a framework has to honour
supply:
  shape: a render option carrying the token string
  why_not_the_context:
    fact: htmlbind cannot read a token out of a context, because the context key belongs to the framework and the module has nothing to look up
    consequence: an option is not a worse threading story than a context here, it is the only one
  call_sites: a framework passes it once inside its own render entry, from its own context accessor, rather than at each handler
  absent:
    rule: a chain that renders an unsafe form with no token supplied fails the render, naming the template
    why: emitting the field empty produces a form that submits and is rejected, which is a failure with no diagnostic attached to the thing that caused it
    escape: an explicit option says this render has no CSRF, for a mail body, a static export, or a test
field_name:
  when: generation time, so the whole tag but its value folds into static bytes
  default: _csrf
  configurable: yes, per the same rule as every other name this module puts in a document
  header_name:
    default: X-CSRF-Token
    note: this one is a de facto standard rather than a tinybind namespace, so it does not follow the header prefix
insertion:
  where: as the first child of the form element, so a later field cannot displace it
  which_forms:
    included: method post, put, patch, and delete
    excluded:
      get:
        rule: never
        why: a GET form's fields become the query string, and a token in a URL reaches history, logs, and referrers, which policy:html-update-csrf-protection forbids
      cross_origin_action:
        static: a generation error, because inserting the token would hand the session's secret to a third party
        dynamic: inserted, because the action is not knowable at generation time; the opt-out below is what a caller posting off-origin uses
  opt_out: an author attribute on the form, following the data-attribute prefix, for the rare form that must not carry it
  no_double_insert: a form that already carries a field of the configured name is left alone, so a hand-written token still works
runtime_header:
  what: the browser runtime adds the header to every request it issues — navigation, action, redraw, and live
  source: the bootstrap configuration already carried on the runtime script tag
  why_both_channels: a form must submit with scripting disabled and cannot set a header; a fetch can set a header and should, because the body may not be a form at all
  agreement: both channels carry the same value, which is what requirement:render-value-provider stability exists to guarantee
verification:
  home: htmlupdate, which is where net/http already is
  shape: a helper taking a request and the expected token, reading the header first and the form body second
  constant_time: yes, per policy:html-update-csrf-protection
  not_included: the session lookup that produces the expected token, which is the framework's
consequences:
  cache_exclusion:
    fact: a component rendering an unsafe form now carries a per-request value, so it cannot be a requirement:component-output-cache body
    scale: this is a behavior change for a project caching a component that contains a form, and generation can detect it statically exactly as it detects a per-request builtin element
    direction: a generation error rather than a stored token, because a cached body serving one session's token to the next is the failure this whole requirement exists to prevent
  frame_validator:
    fact: the token is rendered bytes, so it enters the boundary's frame validator
    stable: a session-scoped token means the frame is stable across renders, so deltas are unaffected
    on_rotation: logging in invalidates every form-bearing boundary, which is correct: those forms genuinely changed
  no_js: a document rendered with no runtime still submits, because the field is in the markup rather than added by script
non_goals:
  - session storage, or any opinion about where a session lives
  - origin and Fetch Metadata validation; the module emits and reads the token and never inspects where a request came from
  - setting or reading cookies
  - Origin and Fetch Metadata validation, which belong to the middleware that sees the request first
  - protecting an API authenticated only by an explicit Authorization header, where a cross-site form cannot set the header at all
acceptance:
  - a template author writes no token markup and every unsafe form carries one
  - a deployment that turned the token off regenerates form-bearing components that are cacheable again
  - a page with two forms carries the same token in both, and the runtime sends that same token as a header
  - a GET form carries no token
  - a form whose action is a static cross-origin URL fails generation
  - rendering a chain with an unsafe form and no supplied token fails, naming the template
  - a project with no unsafe form regenerates byte-identical Go and renders byte-identical output
  - a cached component containing an unsafe form fails generation
related:
  - policy:html-update-csrf-protection
  - requirement:render-value-provider
  - requirement:template-server-functions
  - requirement:html-runtime-bootstrap
open_questions:
  - whether the opt-out is a form attribute, a generator option naming exempt routes, or both
  - whether a stateless signed double-submit token needs a different supply shape, given it is derived from the session rather than stored
  - whether the off switch should also drop the runtime's request header, or keep it so a deployment can turn verification back on without a rebuild
  - whether the field name has to agree with a framework's existing middleware, which would make it a deployment setting rather than a generation one
```
