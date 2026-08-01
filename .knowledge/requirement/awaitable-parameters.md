---
id: requirement:awaitable-parameters
type: requirement
title: Caller Started Awaitable Parameters
---
Let a caller start slow work itself and pass the pending result in as a typed parameter or record field, so an await boundary renders when that value settles instead of starting the call.

```yaml
priority: could
source:
  - concept:html-render-runtime-extensions
  - requirement:async-external-functions
  - user design discussion 2026-07-27
review_gate: approved and implemented 2026-07-27
problem:
  current: every slow call starts inside the template, from a decision:async-boundary-syntax await clause header
  limits:
    - the caller cannot start work before rendering, so request parsing and layout rendering never overlap with the fetch
    - a value the handler already had to compute for a status or header decision cannot be reused; the template starts its own call
    - the callee is fixed at generation time, so no caller can substitute a test double or a per-route implementation
    - work produced outside the same-package external surface, such as a channel an existing service already returns, has no way in
template_type:
  form: prefix modifier on a type, written `async int`, `async User`, `async Order[]`, approved 2026-07-27
  meaning: a value that will settle once into T or into a failure; not a function and not callable
  usage: legal only as the source expression of a decision:async-boundary-syntax await binding
  binding: the primary subtree binds an ordinary T, exactly as for an async external call
  non_value: joins requirement:template-language-core non_value_types; never an operand, never rendered, never compared
  mixed_clause: one await clause may bind async external calls and async-typed expressions together; they settle together and the first failure in declaration order decides the boundary
  reason_no_new_clause: wait site, fallback, recover, scoping, cancellation, and streaming are already decision:async-boundary-syntax; only the source of the awaited value is new
  naming:
    reuses: the `async` of `external async`, so one word means one thing across declarations: this arrives later
    no_collision: decision:async-boundary-syntax rejected `async` as a clause keyword; a type modifier is a declaration, not a wait site, which is what that rejection asked for
    go_pair: the Go handle keeps the noun `Pending[T]`, because a Go type names the state a value is in and `pending` is already the decision:async-boundary-syntax term for that state
positions:
  parameter: a component parameter
  record_field: a record field, so one record carries settled and pending members together, approved 2026-07-27
  partial_read: the synchronous fields of such a record are ordinary values readable outside any await clause, so a fallback subtree renders `user.name` while `user.orders` is still pending
  await_source: any expression of async type, so field access is a binding source beside a call
  per_item: an array of records each holding an async field, which is how a for body awaits its own item; that shape replaces a dedicated array-of-pending type
  precedence: the modifier applies to the whole type expression, so `async Order[]` is one pending array rather than an array of pending values
  nesting: async may not modify another async
  optional: the existing postfix `?` composes, so `async User?` is a pending optional and needs no `(async User)?` form or grammar change
absent_value:
  decided: an unset handle is a legal value wherever the awaited type is optional, approved 2026-07-27
  settles: immediately as absent; the primary subtree renders with an absent optional and the boundary never opens a wait
  not_a_failure: absence is data, not data:async-render-error, so it never routes to a recover subtree
  covers:
    - a caller with nothing to pass for that parameter or field leaves it at its zero value
    - a caller whose work legitimately produced nothing settles the same way
    - both collapse to one optional binding, so a template handles them with the null check it would write anyway
  non_optional: an unset handle where the awaited type is not optional stays a caller bug, reported as the validation diagnostic below
no_panic:
  guarantee: no shape of an unset or misused handle panics or blocks forever, approved 2026-07-27
  zero_read: a read goes through a method that inspects the zero value first, so it never dereferences a nil internal channel and never waits on one
  non_optional_unset: fails validation during the initial pass with a message naming the parameter or field, which is an ordinary error response rather than a panic
  work_panic: htmlbind.Go recovers a panic in the work it started and turns it into the error, as requirement:async-external-functions does for an async external
  complete: every handle comes from Go, Resolved, or Failed, so there is no goroutine the package did not start and no unrecovered path into a boundary
  optional_path: awaiting a field of an optional record follows the ordinary null-check rules of requirement:template-language-core, so an unchecked path is a compile error rather than a render-time nil
go_representation:
  chosen: `htmlbind.Pending[T]`, a settle-once read-many handle holding the value and the error
  model: a JavaScript Promise minus chaining; it resolves once and every later read observes the resolved state, which is the property a bare channel lacks
  internals: a done channel closed on settle, which is what lets a read select against the request context; it is never part of the API
  reader: wait for the done signal or the request context, then read the fields; nothing is consumed, so a later read sees the same values
  constructors:
    start: `htmlbind.Go(ctx, func(context.Context) (T, error)) Pending[T]` runs the work in its own goroutine and recovers a panic into the error
    settled: `htmlbind.Resolved(v)` and `htmlbind.Failed(err)` for tests and for a value the caller already has
  no_channel_constructor:
    decided: the package exposes no channel-taking constructor, approved 2026-07-27
    reason: a service that returns a channel is adapted by receiving from it inside the Go closure, which is one line and keeps every handle one the package started
    gain: the no_panic guarantee below covers every handle the package can produce, with no adopted-goroutine exception
    consequence: a channel is an implementation detail on both sides of the boundary and appears in no signature
  reason:
    - a resolved handle can be read by every boundary that holds it, which is what a template composed from a layout and a page needs
    - no runtime memo table and no keying on channel identity, so decision:reflection-free stays a non-issue rather than a caveat
    - the wait stays the select the runtime already performs for cancellation and timeout
    - a handle is a value, so the template language still needs no callable type
  cost: the caller goes through a constructor instead of writing a bare channel, accepted because the adapt constructor keeps a hand-rolled goroutine one call away
scope:
  decided: the handle is a render-parameter type, not a general future, approved 2026-07-27
  means: it exists to carry one pending value from a caller into an await binding, and its API stops there
  excluded:
    - combinators such as All, Race, Then, or Map; a caller that combines work does so in its own code before constructing a handle
    - a caller-facing wait; the read surface is htmlbind runtime API for generated code, in the sense of decision:runtime-package-boundaries, not a concurrency primitive for application logic
    - storing a handle beyond the render that receives it, which request_binding below already forbids
  handler_reuse: a handler that needs the value itself computes or awaits it with its own code and passes htmlbind.Resolved, so the template never starts a second call
  reason:
    - a general future invites a second concurrency vocabulary next to goroutines and errgroup, which the project does not need to own
    - the narrow surface keeps the settle-once read-many guarantee small enough to state on the type
    - htmlbind stays a transport-neutral rendering leaf rather than a utility package
rejected_forms:
  bare_channel:
    shape: `<-chan htmlbind.Result[T]` passed straight in as the parameter
    why: a channel delivers one value to one reader, so a second await of the same value blocks until the boundary deadline; recovering read-many needs a per-render memo table keyed by channel identity, which is more runtime machinery than the wrapper it was avoiding
    also: an abandoned boundary never receives, so the contract had to require a buffered send to keep the sender from leaking
    kept: nothing; a service that already returns a channel is received from inside the Go closure
  function_value:
    shape: a `func() (T, error)` parameter called from the await header
    why: adds a callable type to requirement:template-language-core, still starts the work at await time so it overlaps nothing, and reopens who cancels it and how often it may run
    keeps: the one thing it buys is per-iteration arguments inside a for body, which an array of records with an async field covers instead
settlement:
  once: a handle settles exactly once and stays readable afterwards; the constructor owns that guarantee, so no template rule depends on it
  zero_value: an unset handle settles as absent when the awaited type is optional, and otherwise fails validation before commit, per absent_value and no_panic
  timeout: the boundary deadline bounds the wait, never the caller's work
  no_per_parameter_deadline:
    decided: an async parameter carries no deadline of its own, approved 2026-07-27
    reason: the constructor already takes a context, so a caller that wants a shorter bound sets it there, on the work it owns; the render only bounds how long it waits
    consequence: one less template annotation and one less place where two deadlines could disagree
  cancellation: request cancellation stops the wait; the work is not cancelled, because the caller owns it and stops it through the context it captured
  unawaited: a handle no branch awaits is dropped; its goroutine finishes on its own and the result is discarded, with nothing left blocked
sharing:
  read_many: every await of one handle sees the same settled result and the work runs once, because sharing is a property of the type rather than of the runtime
  cross_component: a chain layout and its page may hold the same handle; generation cannot see that, and with a resolved handle it does not need to
  repeat_await: two await bindings of one parameter are legal and settle together; the second is not a second invocation
  contrast: requirement:async-external-functions starts one invocation per boundary instance; a handle settles once however many boundaries read it
request_binding:
  reach: a parameter of async type, or of a record type holding an async field at any depth
  fragment: such a fragment is bound to one request and is not reusable, unlike the requirement:html-component-api default
  cache: requirement:component-output-cache must not key on or store a component instance carrying one, for the reason in requirement:render-value-provider
  capabilities: data:component-render-capabilities marks it request-bound, beside partial update boundaries and request-keyed caches
  sync_entry: the decision:async-component-signature sync entry blocks on the handle and renders the settled subtree in place, unchanged
validation:
  generation: an async-typed expression in a non-await position, a nested async type, and a field or parameter whose Go type is not the handle
  before_commit: the unset-handle check runs during the initial pass of requirement:chain-render-pipeline, reaching record fields as well as parameters, so it can still produce an error response
  scope: that check skips optional awaited types, because absence is legal there
  placement:
    hoisted: a check reachable from the component's own parameters becomes the plan's Check, which runs at chain assembly and again before that component writes its first byte; the ordinary layout and page shape therefore fails with nothing written at all
    positional: a check rooted in a loop item or an enclosing boundary's scope stays an op at the boundary, because the value it reads does not exist until rendering arrives there
    duplication: a chain member's check runs twice, at assembly and at execution, which is cheaper than tracking which values were already seen and is safe because it is a pure predicate
acceptance:
  - a handler that starts a fetch before rendering serves fallback bytes without waiting, and the boundary settles from the caller's handle
  - one handle awaited by both a layout and its page runs the work once and both read the same value
  - a record with `name: string` and `orders: async Order[]` renders the name in the fallback subtree and the orders in the primary one
  - a for body awaits its own row's async field and opens one boundary per iteration, per decision:async-boundary-syntax
  - an `async User?` parameter left unset renders the primary subtree with an absent value, opens no boundary, and does not panic
  - an unset handle on a non-optional type fails before any byte is written, names the parameter or field, and does not panic
  - a service that returns its own channel is adopted by receiving from it inside the Go closure, with no change to that service and no channel in any htmlbind signature
  - a component taking an async parameter still references no net/http identifier
go_surface:
  handle: htmlbind.Pending[T] with IsSet and Wait; the done channel stays unexported
  constructors: htmlbind.Go, htmlbind.Resolved, htmlbind.Failed
  unset: htmlbind.ErrUnsetPending and htmlbind.UnsetPendingError carrying the declared path
  plan: htmlbind.Plan.Check for the hoisted form and htmlbind.Require for the positional one
  assembly: Fragment.Validate and Wrapper.Validate, run by chain assembly before anything is written
open_questions:
  - whether a settled-absent optional still emits a fallback and completion pair or renders inline from the start
```
