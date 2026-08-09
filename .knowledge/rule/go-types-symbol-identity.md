---
id: rule:go-types-symbol-identity
type: rule
title: go/types Symbol Identity
---
Resolve call sites with go/types so only the listed stdlib and tinybind symbols participate in route and handler discovery.

```yaml
status: implemented
requirement: requirement:strict-symbol-identity
method: go/types type-checked AST (host generator)
identity: package path + object name (and receiver type for methods)
not: bare Ident/Selector name equality alone
symbol_set:
  default: built-in identities below
  custom: exact identities from requirement:configurable-generator-discovery

route_registration_only:
  - net/http.Handle
  - net/http.HandleFunc
  - "(*net/http.ServeMux).Handle"
  - "(*net/http.ServeMux).HandleFunc"

tinybind_calls_only:
  package: github.com/shibukawa/tinybind-go
  functions:
    - Bind
    - Write
    - WriteStatus
    - WriteError
    - WriteStream
    - DecodeJSON
    - EncodeJSON

  error_constructors:
    - BadRequest
    - Unauthorized
    - Forbidden
    - NotFound
    - Conflict
    - PayloadTooLarge
    - Internal
    - Validation
  optional_helpers_if_scanned:
    - Field
    - BindError
    - AsHTTPError

alias_import:
  required: true
  example: |
    import hb "github.com/shibukawa/tinybind-go"
    hb.Bind[T](r)           # recognized via types
    hb.BadRequest(...)      # recognized via types
  forbid: matching only default name "httpbind"

false_positive_reject:
  - otherpkg.HandleFunc(...)
  - otherpkg.Bind[T](...)
  - local type method named Write/Bind
  - mux-like third-party routers named HandleFunc

applies_to:
  - concept:route-discovery
  - rule:request-model-discovery
  - rule:response-model-discovery
  - rule:error-response-discovery
  - DecodeJSON/EncodeJSON type-arg discovery in generator
  - flow:handler-parse
  - requirement:configurable-generator-discovery

resolution_precondition:
  rule: a load whose hand-written files name an import that resolved to no package fails, rather than being analyzed
  why: identity matching is by resolved package path, so an unresolved runtime import makes every pattern miss at once. Nothing errors — the pass finds no call sites and reports a package with no uses
  what_that_looked_like: a Firestore package with a Store call and a Load call generated a decoder and no encoder, so the write half looked like a discovery rule that had never been written. The decoder came from the .tb.firestore declaration, which requirement:firestore-typed-queries parses from disk and which therefore needs no types; the two calls were both invisible. Reported by the downstream framework 2026-08-05 against v0.3.6 as missing ArgumentType discovery, which exists and works
  not_this: a type error. A bound type has no EncodeEntity until the codec is generated, which is every first run, so failing on type errors would fail generation at the only moment it is needed. The two are told apart by whether the import produced a package, not by the error text
  generated_files_excluded: a codec pass writes the first import of a runtime package into a package that had none, so between passes the module can name a dependency go.mod does not yet require. Discovery does not read generated files, so no call site is behind that import and the second pass has nothing to miss. ast.IsGenerated is the test, on the header rule:generated-source-not-discovered already requires
  built: generator/load.go unresolvedImports, checked in loadPackage so every phase inherits it

implementation_notes:
  - load package with go/packages or equivalent types.Config
  - use types.Info.Uses / Selections to resolve *types.Func
  - compare func.Pkg().Path() and func.Name(); for methods also receiver Named/Pointer
  - keep TinyGo/runtime free of go/types (analysis is host generator only)
  - go/packages returns non-nil Types and TypesInfo for a package whose imports failed, so a nil check does not catch this

related:
  - requirement:strict-symbol-identity
  - concept:handler-discovery
  - concept:route-discovery
  - concept:error-helpers
  - rule:error-response-discovery
  - rule:request-model-discovery
  - rule:response-model-discovery
  - api:bind
  - api:write
  - api:write-error
  - api:new-stream
  - api:decode-json
  - api:encode-json
  - flow:handler-parse
  - concept:code-generation
```
