---
id: requirement:route-table-export
type: requirement
title: Route Analysis Result Export
---
A generation run hands parser.Result to the caller, so downstream route checks read the existing analysis instead of re-implementing it.

```yaml
priority: should
status: implemented 2026-08-20; sister method on the artifact API plus a Routes field on GenerateResult, over one cached parse per run
source:
  - downstream Popcorn Web request 2026-08-20; its data/route-table spec and rule/route-and-template-checks are already written against this output
  - claims verified against local HEAD 65f7fe7
problem:
  today: api:generator-artifacts returns []Artifact only, and Artifact has no place for routes; api:generator-execution GenerateResult carries artifact paths and diagnostics but not parser.Result
  reproduction_trap: parser.ParseLoadedPackage and parser.Config are public, but the generation run passes Options.normalized().parserConfig, both unexported; a caller reproducing the parse must copy the normalization and reload go/packages, which is the duplicate analysis petitweb decision/shared-check-catalog forbids
  downstream_effect: petitweb api/cli-doctor reports PW02xx checks as not-run for want of a route table; its editor route view ships without Go-registered routes
already_present:
  - parser.Result is public with JSON tags, Routes plus Diagnostics
  - parser.Diagnostic records unresolved sites with position and reason code, exactly the data/route-table unresolved entry
  - Result.Normalize gives deterministic order
missing: only the exit; no new analysis
shape_as_built:
  sister_method: 'GenerateArtifactsWithRoutes(ctx, req) ([]Artifact, *parser.Result, error); a single Result rather than the proposed map, because the API is per-package and the caller''s loop is the map'
  execution_field: GenerateResult.Routes on api:generator-execution, non-breaking; nil on check, report-only, and cached results, which run no fresh analysis
  rejected_artifact_kind: nothing lands on disk, so petitweb policy/generated-artifacts and api/cli-check drift comparison stay untouched
  parse_reuse: realized; packageLoad caches one parser.Result beside the type-checked package, OpenAPI and transform-route phases read it, and TestRunParsesRoutesOnce pins the at-most-once property beside the load counter's
opportunity:
  parse_reuse: the three ParseLoadedPackage call sites the request counted now share the cached seam, so a fasthttp-plus-OpenAPI run parses once where it parsed up to three times; the request paid for itself, extending decision:shared-package-load from load to parse
out_of_scope_downstream_work:
  - framework mounts, health, readiness, OpenAPI, asset paths; petitweb adds them with their enabling config keys
  - page-tree-derived routes; petitweb rule/static-route-discovery excludes them from registration-site analysis
  - cross-backend table diff; needs only two exported tables
consumers:
  - petitweb rule/route-and-template-checks PW02xx, duplicate patterns, mount collisions, unreachable routes, route-page pairing
  - petitweb api/cli-doctor route checks
  - petitweb requirement/editor-route-explorer
acceptance:
  - one generation run yields resolved routes and unresolved sites, both positioned via requirement:route-registration-positions; TestGenerateArtifactsWithRoutes pins it
  - dynamic patterns appear as positioned unresolved entries, not silently dropped, honoring requirement:analysis-diagnostics; TestGenerateArtifactsWithRoutesReportsUnresolvedSites pins it
  - caller neither loads go/packages itself nor copies Options normalization
  - output order is stable for identical input; Result.Normalize runs inside the parse
omits_openapi_resolved:
  - route existence is answered by Routes membership, never by the flag; a site-level failure that prevents discovery carries OmitsOpenAPI true and no Route, while a model-level finding on a discovered route carries false beside the Route it annotates
  - so the flag reads as OpenAPI impact only, and the downstream's exists-as-route question needs no second signal
related:
  - requirement:route-registration-positions
  - api:generator-artifacts
  - api:generator-execution
  - decision:shared-package-load
  - requirement:analysis-diagnostics
  - rule:analysis-diagnostics-check
  - concept:route-discovery
  - concept:openapi-generation
```
