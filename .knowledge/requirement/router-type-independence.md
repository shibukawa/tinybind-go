---
id: requirement:router-type-independence
type: requirement
title: Router Type Independence
---
Name the router the generated registry installs on separately from the package supplying Request and ResponseWriter.

```yaml
priority: must
status: implemented 2026-07-30
source:
  - downstream framework integration request 2026-07-30
  - decision:stdlib-servemux
problem:
  registry_writes: 'mux *{{.Symbols.HTTPAlias}}.ServeMux and {{.Symbols.HTTPAlias}}.NewServeMux()'
  same_alias_supplies: Request in the decoder and ResponseWriter in the handler
  effect: moving the router moves the request package with it, so a framework wanting only its own router replaces the whole registry template
implemented:
  symbols: MuxImport, MuxAlias, MuxType, MuxConstructor
  defaults: net/http, http, '*http.ServeMux', http.NewServeMux
  scope: the Register parameter type and the NewServeMux body; nothing else reads them
  registered_through: HandleFunc alone, for both page routes and action endpoints
  written_verbatim: MuxType and MuxConstructor reach the output as spelled, so an interface needs no pointer
  empty_constructor: omits the constructor function, for a router generated code cannot build
  request_import: the registry names the request package only where it generates a handler body, so a tree of raw handlers imports the router alone
  gain: a framework keeps the built-in registry template and supplies its own router type
deferred:
  shape: default the parameter to a one-method interface or a type parameter
  offered_by: a downstream framework wanting its own Register[M Router] to become the upstream default
  why_deferred:
    - decision:stdlib-servemux keeps the concrete stdlib type as the default so core defaults require no framework-specific router
    - MuxType already lets a framework name its own interface, so this integration needs no change to the exported default shape
    - a string-valued MuxType cannot express a type parameter, so generics stay a downstream template choice
  known_feasible: '*http.ServeMux satisfies interface{ HandleFunc(string, func(http.ResponseWriter, *http.Request)) }, because generated code registers through that method alone'
  revisit_if: one generated registry must serve several router types in one build
acceptance:
  - setting the four symbols installs routes on a framework router with the built-in registry template
  - the decoder still names net/http for Request while the registry names another package for the router
  - defaults produce the same registry source as before the split
related:
  - requirement:generated-route-registration
  - decision:stdlib-servemux
  - decision:framework-integration-seams
```
