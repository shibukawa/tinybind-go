---
id: requirement:custom-framework-generation-profile
type: requirement
title: Custom Framework Generation Profile
---
A downstream framework reproduces its published developer experience from tinybind generation alone, without workarounds around the generator.

```yaml
priority: must
source: downstream framework CLI requirements for tinybind v0.1.12
prohibited_workarounds:
  - copying or renaming source files to a tinybind-owned suffix before generation
  - rewriting generated Go source after generation
  - declaring alias wrappers only to rename generated functions
  - running a goimports-equivalent pass over generated source
capabilities:
  discovery: api:generator-call-registration with data:generator-call-pattern
  input_suffixes: requirement:configurable-template-file-patterns
  artifacts: requirement:per-source-generation-artifacts
  html_shape: requirement:html-component-api
  sql_shape: decision:sql-context-executor-api context-only mode
  output_hygiene: rule:generated-source-self-contained
integration_fixture:
  layout:
    - handler.go
    - page.pw.html
    - users.pw.sql
    - config.go
  registered_calls:
    - framework request parse wrapper over api:bind
    - framework API response wrapper over api:write
    - framework config registration wrapper over api:configbind-bind
    - framework subcommand wrapper over api:configbind-subcommand carrying name and help
  options:
    - custom HTML and SQL template patterns
    - SQLContextOnlyAPI enabled with a framework SQLExecutorResolver
acceptance:
  - custom package and function names are discovered through the registered call patterns
  - custom-suffix HTML and SQL sources are discovered without renaming or copying
  - generation returns per-source artifacts naming the real source paths
  - generated HTML components have the func(io.Writer, Params) error shape
  - generated SQL functions use the source-declared names and take only context plus typed parameters
  - subcommand name and help strings are read from the registered role sources
  - a fixture written from every returned artifact passes go test ./...
  - diagnostics report the custom-suffix source path, line, and column
related:
  - requirement:framework-wrapper-discovery
  - requirement:configurable-generator-discovery
  - requirement:extensible-generator-command
  - requirement:configurable-template-file-patterns
  - requirement:per-source-generation-artifacts
  - requirement:html-component-api
  - rule:generated-source-self-contained
  - decision:sql-context-executor-api
  - api:generator-artifacts
  - api:generator-call-registration
```
