---
id: api:generator-execution
type: api
title: Reusable Generator Execution
---
One context-aware API executes all enabled generation phases without parsing process arguments or terminating the process.

```yaml
public_shape:
  - "func (g *Generator) GeneratePackage(ctx context.Context, request GenerateRequest) (GenerateResult, error)"
per_source_variant: api:generator-artifacts
GenerateRequest:
  fields:
    - package directory
    - output directory
    - artifact file names
    - check-only mode
    - per-run generate-all and SQL context switches
    - per-run OpenAPI enable switch bounded by data:generator-options
    - per-run force switch bypassing rule:generation-input-hash
GenerateResult:
  fields:
    - generated artifact paths by kind
    - analysis diagnostics
    - no-work state
    - skipped state, set when the recorded input hash still matches
behavior:
  - normalize data:generator-options once
  - return the recorded paths without writing when rule:generation-input-hash matches
  - run template, mapping, configbind, and OpenAPI phases consistently
  - type-check the package once per run, per decision:shared-package-load
  - stamp every written file with data:generation-stamp
  - preserve the existing configbind-only and package-local artifact rules
  - return typed errors rather than formatted process output
  - check context cancellation between analysis and write phases
  - never call os.Exit
  - never parse os.Args
consumers:
  - api:generator-main generate command
  - framework build command before compiler invocation
  - framework watch command after a filesystem change
  - tests and editor integrations
related:
  - flow:code-generation
  - flow:configbind-codegen
  - requirement:extensible-generator-command
  - requirement:modular-package-generation
  - requirement:incremental-generation
  - api:generator-artifacts
  - rule:generated-source-self-contained
  - rule:generation-input-hash
  - decision:shared-package-load
```
