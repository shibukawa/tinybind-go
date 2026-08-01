---
id: decision:html-route-go-package-model
type: decision
title: HTML Route Go Package Model
---
Treat each route directory as an ordinary Go package holding its own template, loader, and generated code, with one registry package at the route root.

```yaml
source:
  - concept:filesystem-html-routing
  - user package discussion 2026-07-23
  - user logic-placement decision 2026-07-27
review_gate: proposed package model requires user approval
supersedes:
  earlier_model: route subdirectories are template resources emitted into one configured Go package
  reason: colocated loaders in requirement:colocated-route-logic need real packages, and rule:go-safe-route-directory shows that is only possible with decision:route-segment-notation naming
  cost_accepted: the route tree is now a Go package hierarchy, so directory names are constrained
route_directory:
  role: both a URL segment and a Go package
  package_clause: matches the directory name, which decision:route-segment-notation keeps a legal identifier
  contents: the reserved templates, the optional requirement:colocated-route-logic page.go, generated files, and whatever else the application or its framework puts there
generated_output:
  per_route: the route's component and decoder are emitted into that route's own package
  registry: the route root package owns handler registration and layout composition
  registry_in_root:
    decided: implementation 2026-07-28
    why: a registry in the root and a composer in a leaf cannot coexist, because the leaf would import the root for its ancestor layout while the root imports the leaf for its page
    resolution: composition moved to the registry, so every generated import points down the tree and no cycle exists
    supersedes: an earlier plan for a distinct registry package and a per-route composer
  naming: generated symbols are package-local, so two routes may generate the same symbol name without collision
  per_source_files: each template names its own generated file, so page.tb.html and layout.tb.html in one directory do not claim the same output
import_direction:
  registry_to_route: the root registry imports every route and layout package below it
  route_to_ancestor: not needed and not emitted; the registry holds every wrapper it composes
  route_to_shared: a route package may import ordinary application packages outside the tree
  cycle_freedom: every generated import points down the tree, which is the whole reason composition lives in the root
  rung_3_consequence: a hand-written handler owns its response and composes its own chain if it wants one, because it cannot reach a composer that lives above it
user_logic:
  primary: requirement:colocated-route-logic beside the template
  heavy: ordinary packages outside the route tree, imported by the loader like any Go dependency
  legacy: data:html-route-dependencies injection remains available and is unchanged for flat template mode
tooling:
  gained: gopls, gofmt, go test, go vet, golangci-lint, and delve apply to route logic with no special support
  ./...: every route package is matched, so tests and vet run without extra configuration
compatibility:
  - existing flat template mode retains same-directory package and external function behavior
  - a project not opting into the route tree sees no change
constraints:
  - no runtime reflection or string-based dependency lookup
  - binding between generated code and loader types is type-checked by the Go compiler
  - a directory that would be an illegal Go path element is a generation error naming rule:go-safe-route-directory
open_questions:
  - configuration syntax for the route root
  - registry package name and whether it may be the route root package itself
  - whether generated per-route files may be written to a mirrored output tree instead of in place
  - placement of components shared by several routes without adding a URL segment
```
