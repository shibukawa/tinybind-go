---
id: requirement:configurable-template-file-patterns
type: requirement
title: Configurable Template File Patterns
---
Template discovery accepts independent HTML and SQL file globs through data:generator-options and api:generator-main while preserving existing defaults.

```yaml
scope: direct files in one Go package directory
matching: filepath.Match against file base name
defaults:
  html: '*.tb.html'
  sql: '*.tb.sql'
configuration:
  api:
    html: HTMLTemplatePattern
    sql: SQLTemplatePattern
  cli:
    html: --html-template-pattern
    sql: --sql-template-pattern
rules:
  - empty API patterns use defaults
  - discovery does not descend into child directories
  - each matched file is parsed by its associated template language
  - invalid globs fail generation
  - a file matching both patterns fails as ambiguous
consistency:
  - one configured pattern drives discovery, parse, type check, diagnostics, and artifact ownership
  - generation never creates, renames, or removes a copy of a source file to satisfy the default suffix
  - data:generation-artifact records the real matched path, not a normalized or default-suffix path
diagnostics:
  - file name, line, and column identify the real on-disk source
  - a custom-suffix source never reports a default-suffix path
acceptance:
  - a package containing only custom-suffix templates generates without any default-suffix file present
  - a parse error in a custom-suffix template reports that file path
  - default configuration behavior is unchanged
related:
  - data:generator-options
  - api:generator-main
  - api:generator-execution
  - requirement:per-source-generation-artifacts
  - requirement:custom-framework-generation-profile
```
