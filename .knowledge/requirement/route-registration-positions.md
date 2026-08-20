---
id: requirement:route-registration-positions
type: requirement
title: Route Registration Site Positions
---
parser.Route carries the registration site as file, line, and column, at the precision parser.Diagnostic already records for unresolved sites.

```yaml
priority: should
status: implemented 2026-08-20; parser.Position with File, Line, Column; Route.Site json site, filled from the registration call position
source:
  - downstream Popcorn Web request 2026-08-20, for its data/route-table spec
  - claims verified against local HEAD 65f7fe7
problem:
  today: parser.Route has Method, Path, Handler, and typed metadata, but no position; parser.Diagnostic has File, Line, Column, so the analysis provably visits positions and drops them for resolved routes
  downstream_need: petitweb rule/route-and-template-checks PW02xx duplicate-pattern and framework-mount-collision must name both registration sites so a reader chooses which to move; pattern-only findings cannot
  asymmetry: an unresolved site is reported with a position and a resolved one is not, so the better-behaved code gets the worse report
shape:
  as_built: a new parser.Position struct on Route.Site; Diagnostic keeps its flat File, Line, Column fields so its JSON shape does not move
  ordering: Site is the final Normalize sort key, so two registrations of one pattern by one handler order deterministically
  goldens: every testdata expected.json gained a site object; the golden normalizer already reduces any file key to its base name, Site included
consumers:
  - petitweb rule/route-and-template-checks PW02xx via requirement:route-table-export
  - petitweb requirement/editor-route-explorer route view
acceptance:
  - a project registering the same pattern at two sites yields both positions; testdata/duplicate_pattern and TestRouteSitePositions pin it
  - resolved routes and unresolved diagnostics report positions at the same precision
open_questions:
  - registration-site position only for now; the handler body position waits for the downstream route-without-page check, which is not yet designed
related:
  - requirement:route-table-export
  - concept:route-discovery
  - requirement:analysis-diagnostics
  - rule:analysis-diagnostics-check
  - flow:handler-parse
```
