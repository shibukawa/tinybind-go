---
id: api:generator-artifacts
type: api
title: Per-Source Artifact Generation
---
One context-aware API returns data:generation-artifact values for a package without writing any file.

```yaml
status: required
public_shape:
  - "func (g *Generator) GenerateArtifacts(ctx context.Context, request GenerateRequest) ([]Artifact, error)"
  - "func (g *Generator) GenerateArtifactsWithRoutes(ctx context.Context, request GenerateRequest) ([]Artifact, *parser.Result, error)"
routes_variant: GenerateArtifactsWithRoutes adds the run's route analysis per requirement:route-table-export; same phases, same one cached parse, still no file written
request: same GenerateRequest as api:generator-execution
behavior:
  - normalize data:generator-options once, as api:generator-execution does
  - run the same template, binding, configbind, and OpenAPI phases
  - attribute every emitted declaration to the source file that caused it
  - emit no runtime declaration and no package-shared artifact, per decision:generated-runtime-in-module
  - return artifacts in deterministic order independent of filesystem iteration
  - write, create, or remove no file, so no data:generation-stamp is produced or consulted
  - type-check the package once per call, per decision:shared-package-load
  - return typed errors and diagnostics rather than formatted process output
  - check context cancellation between analysis and emission
relation_to_package_api:
  - api:generator-execution keeps the package-aggregated artifact names for compatibility
  - both APIs share one analysis and one intermediate representation
consumers:
  - framework CLI that names outputs after the owning source
  - check mode comparing artifacts against on-disk files
  - editor integrations and tests needing generated source without file effects
related:
  - requirement:route-table-export
  - data:generation-artifact
  - decision:shared-package-load
  - requirement:per-source-generation-artifacts
  - rule:generated-source-self-contained
  - api:generator-execution
  - api:generator-main
  - flow:code-generation
```
