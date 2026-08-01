---
id: rule:go-safe-route-directory
type: rule
title: Go Safe Route Directory Names
---
A route directory that holds Go source must be a legal Go import path element, because one illegal element breaks package pattern matching for the whole module.

```yaml
evidence:
  method: measured against go 1.26.0 in a scratch module, 2026-07-27
  scope: directory names only; file names inside a directory are unaffected
illegal_elements:
  chars: '[ ] { } $ @ : = ( )'
  leading: '- and ~ are rejected as an invalid input directory name'
  examples_rejected: '[id]  {id}  $id  @id  :id  =id  (group)  -id  ~id'
legal_elements:
  examples_accepted: 'id_  slug__  userId_  id-  x-id  id.d  group_'
  identifier_subset: a trailing-underscore name is also a legal Go identifier, so the package clause can match the directory name
ignored_elements:
  leading_underscore_or_dot: '_id and .id are importable by explicit path but excluded from ./... , so go build, go vet, and go test skip them'
  consequence: an ignored directory silently loses test and lint coverage, so it is not a safe home for route logic
blast_radius:
  trigger: one .go file present in an illegal directory
  effect: 'go build ./... , go list ./... , go vet ./... , and go test ./... all fail for the entire module, not only that package'
  reason: the failure happens during package pattern matching, before build constraints are evaluated
  no_go_files: a directory holding only template and asset files is invisible to the Go toolchain and is always safe
escape_hatches:
  build_ignore:
    shape: '//go:build ignore on every .go file in the illegal directory'
    effect: pattern matching succeeds again
    cost: the file compiles nowhere, so gopls, go test, and linters all skip it; one missing tag re-breaks the module
    status: rejected as a foundation by decision:route-segment-notation
  line_directive:
    shape: '//line app/users/[id]/index.tb.html:3 in generated output'
    effect: compiler and vet diagnostics report the original path and line even when that path contains illegal characters
    status: retained as a generation technique; it maps diagnostics but never makes a directory importable
applies_to:
  - decision:route-segment-notation
  - decision:html-route-file-conventions
  - decision:html-route-go-package-model
  - requirement:colocated-route-logic
```
