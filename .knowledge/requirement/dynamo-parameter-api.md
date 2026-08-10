---
id: requirement:dynamo-parameter-api
type: requirement
title: A DynamoDB Call That Takes Its Client
---
Export the client and table naming dynamobind already stores in the Context as a Handle a caller can pass, give every runtime entry a parameter-taking twin, and let one generation option publish declared queries in that form instead of the Context one.

```yaml
status: implemented
proposed: 2026-08-05
implemented: 2026-08-05
built:
  handle: dynamobind/context.go, with NewHandle, WithHandle, HandleFromContext and Handle.Table
  entries: the On twins in dynamobind/item.go, query.go and batch.go
  option: generator/options.go, DynamoParameterAPI, and -dynamo-parameter-api on the CLI
  emitter: generator/dynamoquery_emit.go
  tests: dynamobind/handle_test.go and generator/dynamoquery_modes_test.go
source: user request 2026-08-05
decided_by: decision:nosql-client-supply-modes
default: unchanged; a run setting no option generates the Context form of decision:dynamo-context-client-api
handle:
  what: the exported form of the clientEntry WithClient already builds in dynamobind/context.go
  shape: "type Handle struct" carrying the client and an optional TableResolver, opaque rather than a literal, so a field added later is not a breaking change
  constructor: "func NewHandle(c *dynamodb.Client, options ...ClientOption) Handle", reusing the ClientOption list unchanged
  reuse: "WithClient(ctx, c, options...)" stores exactly this value, so the two modes share one type and one option vocabulary
  zero_value: ErrNoClient at the operation, matching a Context carrying no client, per errors_not_panics of decision:dynamo-context-client-api
  table_names: WithTableNames still applies, so a declared name still maps onto a deployed one with no Context in the path
  ctx_still_first: every entry keeps its leading context.Context for cancellation, per ctx_is_not_removed of decision:nosql-client-supply-modes
runtime_entries:
  rule: one parameter-taking twin per entry of api:dynamobind-operations, with the Handle after ctx and before the table
  suffix: "On", per the open clause of decision:nosql-client-supply-modes
  item: "LoadOn[T, PT](ctx, h Handle, table string, key dynamodb.Key, opts ...dynamodb.GetOption) (T, error)", and the same shift for Store, Remove, Update
  returning: StoreReturningOn and RemoveReturningOn
  paged: QueryPageOn and ScanPageOn
  iterated: QueryOn and ScanOn, whose resolver error is yielded once with the zero value as a failed page already is
  batch: StoreAllOn and LoadAllOn
  direction: these hold the implementation and the Context entries delegate to them, per implementation_direction of decision:nosql-client-supply-modes
  discovery: registered as CallPatterns beside the Context entries, per requirement:parameter-api-call-discovery; this requirement shipped the entries and left them undiscoverable, which is what that one repairs
generation_option:
  name: DynamoParameterAPI
  type: bool
  behavior: a declared query of requirement:dynamo-typed-queries generates as "<Name>(ctx, h Handle, params..., opts...)" instead of "<Name>(ctx, params..., opts...)"
  name_unchanged: the declared name, in either mode; the suffix belongs to the runtime pair and never reaches generated code
  scope: package-wide and fixed at generation time, per package_wide_not_per_call
  table: still the declaration's, still a constant beside the expression, and still mapped through the Handle's resolver rather than reappearing as an argument
  no_both_surfaces_mode:
    not_proposed: a counterpart to SQLContextAPI that generates the second form beside the first
    why: no stated need, and one declaration yielding two exported signatures is what required_not_optional of requirement:dynamo-typed-queries refuses
    later: additive, and nothing here forecloses it
what_this_costs_a_caller:
  gains: no ctx.Value lookup on the operation path, and a program calling neither WithClient nor a Context entry links neither
  pays: the argument at every call site, which is precisely what decision:dynamo-context-client-api chose the default to avoid
  who_wants_it: a caller already holding the client, a size-critical program, and a framework that read its own bundle once per requirement:framework-context-bundle
verification:
  size:
    measured_2026_08_05: tinygo 0.41.1, target wasip1, one program storing and reading one item both ways
    context_form: 3,691,708 bytes
    parameter_form: 3,652,519 bytes
    saved: 39,189 bytes, against the 37,812 requirement:dynamobind-verification measured the Context form costing
    reading: the expectation held; a program calling neither WithClient nor a Context entry links none of the Context machinery, and the two figures agreeing to within 4 percent is what says the saving is that machinery rather than something else
  golden: one generated declared query per mode, compared byte for byte
  equivalence: the same declaration in both modes issues the same request against the internal/dynamofixture fake
acceptance:
  - a run setting no option generates output identical byte for byte to today's
  - NewHandle and WithClient produce the same behaviour for the same client and options
  - a zero Handle returns ErrNoClient rather than panicking, in every entry including the iterators
  - a Handle built with WithTableNames maps a declared name with no Context involved
  - DynamoParameterAPI changes the generated signature and not the generated name
  - the Context entries and the parameter entries share one implementation per operation
related:
  - decision:nosql-client-supply-modes
  - decision:dynamo-context-client-api
  - api:dynamobind-operations
  - requirement:dynamo-typed-queries
  - requirement:dynamobind-verification
  - requirement:framework-context-bundle
  - requirement:firestore-parameter-api
  - requirement:parameter-api-call-discovery
```
