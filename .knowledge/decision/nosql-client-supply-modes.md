---
id: decision:nosql-client-supply-modes
type: decision
title: Three Ways To Supply A NoSQL Client, One Default
---
Reopen the one_surface clause of decision:dynamo-context-client-api and decision:firestore-context-client-api: keep the Context form as the default and generated shape, and add a parameter form and a framework resolver as generation options, because both requests are the same axis rather than two features.

```yaml
status: implemented
proposed: 2026-08-05
implemented: 2026-08-05
source: user request 2026-08-05, reopening a scope call recorded as settled
supersedes:
  clause: "one_surface", in decision:dynamo-context-client-api and decision:firestore-context-client-api
  what_it_said: no client-taking form, no suffixed variant and no generation option
  what_still_holds: the reason for the default, that a client is a deployment fact fixed for a process, so a parameter repeats at every call site what one setup line already said
  what_does_not: that the reason is strong enough to be the only form; it is strong enough to be the default
  why_reopened: the clause priced a call-site property against a parameter, and never priced it against the ctx.Value lookup, which requirement:framework-context-bundle is about
the_axis: who supplies the client and the deployed name
modes:
  context_own_key:
    what: dynamobind.WithClient and firestorebind.WithClient, resolved by the package's own private key
    status: default, unchanged, and what a run with no new option generates
    cost: one context node and one type assertion per package, measured at +37,812 bytes in requirement:dynamobind-verification
  parameter:
    what: requirement:dynamo-parameter-api and requirement:firestore-parameter-api
    for: a caller that already holds the client, and a program that wants none of the Context cost
    cost: a handle argument at every call site, which is the thing the default exists to avoid
  framework_resolver:
    what: requirement:framework-context-bundle, generalizing the SQLExecutorResolver of decision:sql-context-executor-api
    for: a framework that already carries its own Context value and wants tinybind to read that one instead of installing a second node
    cost: a generation option and a named function, with no runtime type added here
not_three_features:
  reading: the parameter form and the framework bundle are the same request made twice, once about the signature and once about the lookup
  evidence: a framework holding one bundle reads it once in middleware and then has a client in hand, which is a parameter call; and a resolver is what the same framework needs where the call site is generated and it cannot pass one
  consequence: the two requirements are written apart because they land in different code, and are decided together because either alone leaves the other's call sites paying the old cost
ctx_is_not_removed:
  fact: every driver entry of system:tinygodriver-dynamodb and system:tinygodriver-firestore takes a leading context.Context for cancellation
  therefore: the parameter form drops ctx.Value, not ctx; the first argument is unchanged in every mode
  stated_because: "receives connection info as a parameter" reads as a signature with no Context, and that signature cannot exist
why_not_a_method:
  wanted: handle.Load(ctx, table, key), which needs no suffix and no option
  blocked: Go forbids type parameters on methods, and every item entry is generic in T and PT per decision:dynamobind-static-dispatch
  consequence: the parameter form is a free function taking the handle, so it needs a name distinct from the Context form
  scope: the language constraint, not a preference; nothing in this module can lift it
package_wide_not_per_call:
  rule: the mode is fixed at generation time for the whole package, as context_only_mode of decision:sql-context-executor-api already is
  why: one declaration must yield one signature, per the required_not_optional clause of requirement:dynamo-typed-queries
runtime_surface_is_not_the_generated_surface:
  fact: the item entries of api:dynamobind-operations and api:firestorebind-operations are library functions a human calls, and a generation option cannot change a library signature
  therefore: both forms exist in the runtime under distinct names in every build, and the option decides only which one generated code calls
  suffix: unavoidable for the runtime pair, and invisible in generated code, which keeps the declared name in either mode
implementation_direction:
  primitive: the parameter form, taking the handle
  wrapper: the Context form, resolving then delegating, as wrapper_behavior of decision:sql-context-executor-api already specifies for SQL
  why_this_direction: one implementation per operation, and the Context cost becomes provably additive and droppable by a program that calls neither WithClient nor the Context form
settled:
  suffix_spelling: "On", shipped; "Direct" and "WithClient" were considered, and "With" was rejected because the With prefix already means option application in both packages
  default_unchanged: a run adding no option generates exactly what it generated before, asserted by a test per package
implementation_findings:
  resolver_returns_a_handle:
    was: a resolver handing back a client and a resolved name, TableFromContext's own shape
    is: "func(context.Context) (Handle, error)"
    why: the loose pair fits no runtime entry, so it would have needed a third form taking a raw client; the Handle feeds the On entries this decision already adds, and one contract then serves both packages
  the_handle_unified_more_than_expected:
    what: WithClient now stores the same Handle NewHandle builds, so the Context path and the parameter path share one type, one ClientOption list and one resolution method
    effect: the three modes are three ways to obtain one value rather than three code paths, which is why the Context entries could become one-line delegations
  cost_confirmed: requirement:dynamo-parameter-api measured the parameter form 39,189 bytes smaller under tinygo, against the 37,812 the Context form was measured costing, so the saving is that machinery rather than something else
related:
  - decision:dynamo-context-client-api
  - decision:firestore-context-client-api
  - decision:sql-context-executor-api
  - decision:framework-integration-seams
  - decision:runtime-package-boundaries
  - requirement:dynamo-parameter-api
  - requirement:firestore-parameter-api
  - requirement:framework-context-bundle
```
