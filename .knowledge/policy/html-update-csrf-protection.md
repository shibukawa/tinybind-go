---
id: policy:html-update-csrf-protection
type: policy
title: HTML Update CSRF Protection
---
Protect generated component and navigation update endpoints when browsers attach ambient credentials.

```yaml
evidence:
  - https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
  - https://pkg.go.dev/net/http#CrossOriginProtection
scope:
  required: cookie-authenticated POST, PUT, PATCH, and DELETE update endpoints, including generated requirement:template-server-functions action endpoints
  conditional: safe render-only GET endpoints must remain side-effect-free; origin defense may still apply
  optional: APIs using explicit non-cookie authorization without ambient credentials
primary:
  stateful: synchronizer token unique and unpredictable per login session or request
  stateless: session-bound signed double-submit token; naive unsigned double-submit is forbidden
browser_transport:
  bootstrap: escaped token in data:html-client-bootstrap meta or inert configuration
  request: X-CSRF-Token custom header from requirement:html-runtime-bootstrap
  form_post:
    shape: hidden form field carrying the synchronizer token, emitted by requirement:template-server-functions
    why: a requirement:template-server-functions form must submit with JavaScript disabled, and a plain form cannot set a request header
    scope: same-origin form posts to a generated action endpoint only
    equivalence: this is the classic synchronizer-token transport and carries the same secrecy requirements as the header form
    unchanged: the header remains the transport for every runtime-initiated update
  forbidden: URL, query string, persistent Web Storage, or application logs
server_validation:
  - validate token before parsing boundary capability or executing renderer
  - compare secrets in constant time where applicable
  - reject missing or invalid token with 403 and record safe diagnostics
defense_in_depth:
  - wrap unsafe handlers with Go 1.25+ http.CrossOriginProtection on the repository Go 1.26 baseline
  - do not treat CrossOriginProtection as the token replacement when requests without Fetch Metadata or Origin must also be rejected
  - validate same-origin Origin and Fetch Metadata according to deployment proxy configuration
  - use Secure, HttpOnly, and appropriate SameSite session cookies; SameSite is not the only defense
  - allow credentialed CORS only for exact trusted origins; never combine credentials with wildcard origin
  - preserve rule:template-context-safety and CSP because XSS can read a DOM-exposed CSRF token
cache:
  - inject token outside requirement:component-output-cache and requirement:layout-reuse-boundaries validators
  - do not share personalized token-bearing complete HTML through public caches
settled_2026_08_03:
  by: requirement:csrf-token-rendering, proposed
  token_interface: a render option carrying the token string, because htmlbind cannot read one from a context whose key belongs to the framework
  rotation: per session, created at login and destroyed at logout or session regeneration; per-request rotation costs multi-tab correctness and buys nothing here
  emission: the module emits the hidden field into every unsafe form and the header from its runtime, so an author writes nothing and cannot forget
  division: the module puts the token everywhere a request can carry it; the framework owns the session, the cookie, and the verification middleware
open_questions:
  - whether an existing framework middleware's field name forces the name to be a deployment setting rather than a generation one
  - delta-response token refresh header; the one downstream that wanted it withdrew the ask on 2026-08-02, having moved to a cookie read at request time and refreshed by set-cookie, which is what Django, Laravel, and Spring's SPA configuration all do and needs no module change, per decision:update-composition-seams
  - policy for unauthenticated but computationally expensive render updates
```
