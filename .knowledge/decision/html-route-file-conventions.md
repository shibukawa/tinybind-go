---
id: decision:html-route-file-conventions
type: decision
title: HTML Route File Conventions
---
Discover pages, layouts, and route logic from reserved filenames below an opt-in route root that defaults to pages.

```yaml
source: concept:filesystem-html-routing
review_gate: proposed convention requires user approval
activation: generator configuration explicitly selects one route root
route_root:
  default: pages
  reason: names what the tree holds, and leaves app free for an application package
  override: configurable, because an existing project may already own that directory
preferred_files:
  page: page.tb.html
  layout: layout.tb.html
  document: document.tb.html only at configured route root
  logic: page.go through requirement:colocated-route-logic
  superseded: index.tb.html, and the method-named get.tb.html and post.tb.html form that briefly replaced it
naming:
  page_component: page.tb.html declares one exported component named Page
  layout_component: layout.tb.html declares Layout
  document_component: document.tb.html declares Document
  enforced: a declaration whose name does not follow the file is a generation error naming both
  chain: the file name determines the component name, the generated types from requirement:html-component-api, and which function page.go must declare
  gain: every symbol a route package exposes is predictable from the file names, so nothing has to be looked up
method:
  serves: GET for the page, per decision:route-handler-shape
  function: an optional func Load in page.go, whose presence and signature select the rung; Page is taken by the generated component, per decision:route-handler-shape
  requires: a route is served when page.tb.html exists; page.go is optional
  form_actions: a form posts to a generated requirement:template-server-functions endpoint, whose Go function also lives in page.go
  other_methods: a JSON API endpoint is registered manually outside the route tree, using the existing flat httpbind path
logical_aliases: layout.html and document.html describe roles; accepting them as physical files remains open
segments:
  notation: decision:route-segment-notation
  static: directory name becomes a literal URL segment
  dynamic: 'id_ becomes one named path parameter through requirement:typed-html-route-parameters'
  catch_all: 'slug__ , with binding rules still open'
precedence:
  - literal route outranks a dynamic sibling
  - duplicate normalized route patterns are generation errors
  - at most one dynamic sibling exists at a directory level unless patterns are provably distinct
tree:
  page: page.tb.html owns GET on the exact directory route
  root_pattern: the root page registers as /{$}, because a bare / is a prefix pattern in the standard library and would answer every unmatched path
  layouts: ancestor layout files ordered from route root to page directory
  document: optional root-only shell from decision:html-document-shell; never owns the root URL
no_reserved_metadata_file:
  rule: this convention reserves no filename for page metadata, sitemaps, or robots
  reason: decision:route-feature-ownership leaves those to a downstream framework, which may reserve its own names in the same tree
constraints:
  - each route directory is a Go package under decision:html-route-go-package-model
  - a directory name illegal as a Go import path element is a generation error naming rule:go-safe-route-directory
  - reserved behavior does not apply to existing flat template discovery
  - generated identifiers are package-local, so a route-relative prefix is no longer needed to avoid collisions
open_questions:
  - physical extension aliases and route-root configuration syntax
  - route groups that do not contribute URL segments
  - optional segment notation and catch-all parameter typing
  - whether a directory holding neither a page nor a layout is a silent pass-through or an error
```
