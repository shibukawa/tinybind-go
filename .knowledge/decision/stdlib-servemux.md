---
id: decision:stdlib-servemux
type: decision
title: Stdlib ServeMux Routing
---
Route registration defaults to Go 1.22+ net/http ServeMux; configured compatible routers may be added without making a framework mandatory.

```yaml
status: accepted
router: net/http ServeMux
role: default
extensions: requirement:configurable-generator-discovery
go_version_min: "1.22"
example: |
  mux := http.NewServeMux()
  mux.HandleFunc(
      "POST /orgs/{org_id}/users",
      CreateUserHandler,
  )
path_params:
  - from: route patterns such as {org_id}
    to: path-tagged request fields
invariant: core defaults require no framework-specific router
related:
  - concept:net-http-handler
  - term:http-metadata
  - concept:handler-discovery
  - concept:route-discovery
  - rule:unsupported-route-patterns
```
