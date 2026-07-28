---
id: decision:route-segment-notation
type: decision
title: Route Segment Notation
---
Mark a dynamic route segment with a trailing underscore instead of brackets, so a route directory can hold ordinary compiled Go.

```yaml
source:
  - concept:filesystem-html-routing
  - user logic-placement decision 2026-07-27
review_gate: proposed notation requires user approval
constraint: rule:go-safe-route-directory
notation:
  static: 'users -> /users'
  dynamic: 'id_ -> /{id}, parameter named id'
  catch_all: 'slug__ -> /{slug...}, parameter named slug'
  rule: the marker is a suffix, so the parameter name is the directory name with its trailing underscores removed
  parameter_name: must be a legal Go identifier and a legal net/http wildcard name
package_clause:
  requirement: a route directory package clause matches its directory name
  works_because: a trailing-underscore name is still a legal Go identifier, so 'package id_' is valid
  effect: gopls and package-name linters see a conventional package and report nothing
tradeoff:
  lost: visual parity with the Next.js '[id]' spelling, which the user explicitly asked for
  gained: gopls completion, go-to-definition, refactoring, go test, go vet, golangci-lint, and delve on route logic
  decided: full Go tooling on request-parsing and data-fetching code outweighs the spelling
rejected:
  bracket_directories:
    shape: 'app/users/[id]/page.go'
    why: rule:go-safe-route-directory shows one such file breaks ./... for the whole module
  bracket_plus_build_ignore:
    shape: '[id] directories whose Go files all carry //go:build ignore'
    why: keeps the spelling but the file compiles nowhere, so it loses the tooling this decision exists to keep, and a single missing tag breaks the module
  template_embedded_logic:
    shape: a Go block inside the page template, extracted at generation with //line mapping
    why: preserves the spelling and maps diagnostics correctly, but no editor resolves symbols inside a .tb.html file and the loader cannot be unit tested directly
    kept_from_it: the //line technique, which generation still uses for template-derived output
  route_groups_in_parens:
    shape: 'app/(marketing)/'
    why: '( is an illegal import path character; a non-URL-contributing group needs a different marker'
open_questions:
  - marker for a route group that contributes no URL segment
  - optional segment notation
  - whether a static segment ending in an underscore needs an escape form
  - shared component placement, given that a leading-underscore private folder loses ./... coverage
```
