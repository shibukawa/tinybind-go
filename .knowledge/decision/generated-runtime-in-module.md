---
id: decision:generated-runtime-in-module
type: decision
title: Generated Template Runtime Lives in the Module
---
Generated template code declares no runtime type, interface, or helper; it references the module runtime packages, so no package-shared artifact exists.

```yaml
status: accepted
source: user design review 2026-07-25
problem:
  - every generated template file redeclared the same runtime types and helpers
  - duplicate declarations forced a package_shared hoisting pass in api:generator-artifacts
  - hoisting is driven by a hard-coded name list, so an unlisted declaration collides; the SQL builder arg method is such a case and breaks two SQL sources in one package
  - each generated package owned an incompatible Statement, SQLExecer, SQLQuerier, and TrustedHTML type
  - a caller could not pass a generated value across package boundaries without conversion
rule:
  - a generated artifact declares only the types, statements, and components its own source declares
  - all runtime support is imported from decision:runtime-package-boundaries packages
  - no artifact of kind package_shared is produced; the kind is removed
sql_runtime:
  package: github.com/shibukawa/tinybind-go/sqlbind
  moved:
    Statement: sqlbind.Statement
    SQLExecer: sqlbind.Execer
    SQLQuerier: sqlbind.Querier
    _tinybindSQLBuilder: sqlbind.Builder
    _tinybindSQLBuilder.arg: sqlbind.Builder.Arg
    _tinybindStatement: sqlbind.Builder.Statement
    _tinybindSQLArgs: sqlbind.AppendValues[T]
    _tinybindSafeMutation: deleted, not moved; rule:sql-static-mutation-safety replaces it with a generation-time proof
  import_scope:
    - the runtime package is imported whenever the source declares any statement
    - database/sql survives only where a signature still names it: sql.Result for exec and sql.ErrNoRows for one
    - strconv and strings leave generated SQL entirely
  placeholder_style:
    api: sqlbind.NewBuilder(style) with Dollar and Question constants
    phase: fixed at generation time per decision:sql-dialect-generation-time and rule:sql-placeholder-emission
  executor_reuse: existing sqlbind.SQLExecutor satisfies Execer and Querier, so decision:sql-context-executor-api resolvers stay unchanged
  generic_helper: AppendValues is a package function because a Go method cannot introduce its own type parameter
html_runtime:
  package: github.com/shibukawa/tinybind-go/htmlbind
  moved:
    TrustedHTML: htmlbind.TrustedHTML
    TrustedCSS: htmlbind.TrustedCSS
    TrustedJavaScript: htmlbind.TrustedJavaScript
    ScriptJSON: htmlbind.ScriptJSON
    _tinybindBool: htmlbind.FormatBool
    _tinybindInt: htmlbind.FormatInt
    _tinybindFloat: htmlbind.FormatFloat
    _tinybindJSONQuote: htmlbind.JSONString[T ~string]
    _tinybindJSONBool: htmlbind.JSONBool
    _tinybindJSONInt: htmlbind.JSONInt
    _tinybindJSONFloat: htmlbind.JSONFloat
    _tinybindJSONOptional<T>: htmlbind.JSONOptional[T]
    _tinybindJSONArray<T>: htmlbind.JSONArray[T]
  existing: htmlbind.Escape already owns the escaping policy
  json_split:
    generated: one encoder per declared record, named after that record and owned by its source
    module: scalars, enums, optionals, and slices are encoded by the generic helpers above
    reason: the scalar encoders are not source-specific, so without the move two HTML sources of one package would redeclare them
  note: the HTML emitter already imports htmlbind, so only declarations move
generator_effects:
  - splitTemplateArtifacts stops separating shared from per-source declarations
  - the runtime-name list and its stale entries are deleted with it
  - per-source artifacts become non-conflicting by construction, not by hoisting
  - aggregated api:generator-execution output shrinks by the same declarations
breaking_change:
  - Build<Component> returns sqlbind.Statement instead of a package-local Statement
  - executor parameter types become sqlbind.Execer and sqlbind.Querier
  - trusted HTML values are constructed as htmlbind.TrustedHTML
  - generated SQL packages now import sqlbind unconditionally, so database/sql reaches TinyGo builds only through SQL mode as before
benefits:
  - one canonical type per concept across every generated package
  - generated values cross package boundaries without conversion, supporting requirement:cross-template-components
  - runtime behavior is versioned with the module instead of frozen into each checkout's generated files
  - runtime fixes ship without regeneration
related:
  - requirement:per-source-generation-artifacts
  - data:generation-artifact
  - api:generator-artifacts
  - decision:template-package-boundaries
  - rule:generated-source-self-contained
```
