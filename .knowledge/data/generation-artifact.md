---
id: data:generation-artifact
type: data
title: Generation Artifact
---
One formatted Go source unit plus the source file that owns it, returned instead of a written path.

```yaml
status: required
fields:
  SourcePath: real on-disk path of the owning source file; empty only for package-shared artifacts
  Kind: ArtifactKind
  OutputBase: suggested output base name without directory, extension, or generated suffix
  PackageName: Go package name of the generated unit
  GoSource: formatted Go source bytes
kinds:
  html_template: one HTML template source
  sql_template: one SQL template source
  binding: binders, writers, JSON codecs, validation, and scanners for the types declared in one Go source file
  configbind: config definitions and subcommands for one Go source file
  openapi: package OpenAPI fragment
  package_shared: template runtime helpers reused by per-source template artifacts
owning_source:
  templates: the template file itself
  binding: the Go file declaring the type
  configbind: the Go file containing the discovered call
  package_wide: empty SourcePath for package_shared and openapi
naming:
  base: owning source base name minus its template suffix or Go extension
  caller_owns_suffix: caller maps OutputBase to its own generated file name
  examples:
    - handlers/home.pw.html -> OutputBase home
    - queries/users.pw.sql -> OutputBase users
    - handlers/user_handler.go -> OutputBase user_handler
content:
  - standard generated-file header marking the file as generated and not editable
  - self-contained per rule:generated-source-self-contained
related:
  - api:generator-artifacts
  - requirement:per-source-generation-artifacts
  - api:generator-execution
  - requirement:configurable-template-file-patterns
```
