---
id: api:register-generated-html-routes
type: api
title: Register Generated HTML Routes
---
Install every generated and hand-written route handler on an application-owned ServeMux using generated patterns.

```yaml
source: requirement:generated-route-registration
conceptual_signature: RegisterRoutes(mux *http.ServeMux, options...) error
behavior:
  - validate mux, generated route conflicts, and required security providers
  - register the requirement:colocated-route-logic function of each route under its derived GET pattern from data:html-render-route-plan
  - register generated component update and partial navigation endpoints when enabled
  - return startup error before serving when configuration is incomplete
options:
  - route exclusion, so an application can register its own pattern instead
  - error and invalid-parameter mapping, applied on the generated typed path only
  - policy:html-update-csrf-protection provider and origin configuration
  - runtime asset and document bootstrap configuration
  - data:html-route-dependencies value, when a route template declares external functions
option_scope:
  typed_routes: the mapping options apply, because the generated handler owns that response
  raw_routes: no option applies to a raw handler body, per decision:route-handler-shape
  not_offered:
    auth_context_hook: the application already wraps the mux, which is where request-scoped policy belongs
    middleware_hook: the same; middleware composes around an http.Handler the ordinary way
    reason: an option here would duplicate what the application already controls at the mux
constraints:
  - registration is an explicit startup call, not package init side effect
  - application owns http.Server, ServeMux, middleware order, lifecycle, and observability
  - manual handlers may coexist when patterns do not conflict
  - a handler is an ordinary http.HandlerFunc, so it is testable without calling this at all
```
