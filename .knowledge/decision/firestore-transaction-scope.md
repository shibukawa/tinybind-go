---
id: decision:firestore-transaction-scope
type: decision
title: Transactions Are Bound, Because Here They Are The Only Conditional Path
---
firestorebind wraps RunInTransaction with typed reads and writes, reversing the call requirement:dynamobind-product-goals made for DynamoDB; the closure still re-runs, and nothing hides that.

```yaml
status: implemented
proposed: 2026-08-03
implemented: 2026-08-04, in firestorebind/tx.go and the version handling of firestorebind/entity.go
why_the_answer_flips:
  dynamodb: the driver declares no TransactWriteItems, so there was nothing to bind, per system:tinygodriver-dynamodb transaction_note
  datastore: RunInTransaction is in the driver, and it is the only way to express a read-modify-write, since the wire has no condition expression and property transformations are excluded
  consequence: leaving it unbound would mean a caller drops to raw datastore.Entity for exactly the operation most likely to be written wrong
three_levels_of_conditional_write:
  level_1_verbs:
    what: Insert fails if the key exists, Update fails if it does not
    binding: "Insert[T EntityEncoder](ctx, v T) (datastore.Key, error)" and "Update[T EntityEncoder](ctx, v T) error", beside Store which upserts
    value: put-if-absent and put-if-present are the two conditions callers write most often, and here they cost nothing
    contrast: requirement:dynamo-optimistic-locking had to generate a condition expression to get the first of these
  level_2_version:
    what: a version tag, per rule:firestore-tag-options
    behaviour: the decoder fills the field from Entity.Version, and a write carrying a non-zero version sends datastore.WithBaseVersion
    how_the_runtime_sees_it: the generated EntityVersion method satisfies firestorebind.Versioner, which Store and Update assert for; a type without the tag implements nothing and writes unconditionally
    which_verbs: Store and Update, and their transaction forms. Insert takes none, because it already fails if the key exists and a precondition on an entity that must not exist yet says nothing
    caller_supplied_option: a caller's own WithBaseVersion is overridden by the tag's, since the two cannot both be right and the tag's is the one the decoder filled
    no_bump: the server assigns the next version, so nothing is incremented here and the returning-form question requirement:dynamo-optimistic-locking left open does not arise
    conflict: the driver answers ErrFailedPrecondition, which reaches the caller unchanged per rule:firestorebind-driver-passthrough
    no_expression_collision: baseVersion is a mutation field rather than a condition expression, so the tag takes nothing away from a caller who also wants a filter or an ancestor; what requirement:dynamo-optimistic-locking had to forbid was a generated condition consuming the one ConditionExpression a caller might also need, and there is no such slot here
    zero_version: a value never read has version zero and sends no precondition, so a first write is an ordinary Store; a caller wanting insert-only uses the verb
  level_3_transaction:
    what: read inside, decide in Go, commit
    needed_for: any predicate over a property value, since nothing on this wire evaluates one
    binding: below
typed_transaction:
  entry: "func Run(ctx context.Context, fn func(*Tx) error, opts ...datastore.TxOption) error"
  read_only: "func RunReadOnly(ctx context.Context, fn func(*Tx) error, opts ...datastore.TxOption) error"
  tx_type: a firestorebind.Tx wrapping *datastore.Tx, so a typed read inside a transaction is the same generic shape as outside
  reads: LoadTx, LoadAllTx, QueryPageTx and CountTx, each taking the tx after the ctx
  why_a_suffix_rather_than_an_overload: Go methods cannot take type parameters, so a typed read cannot be a method on Tx, and a package-level Load taking a tx would collide with the one that does not. The suffix is what the language leaves; an earlier draft wrote these as Load[T](tx, key), which does not compile
  writes: "tx.Store(v)", "tx.Insert(v)", "tx.Update(v)", "tx.Remove(v)"; queued and returning nothing, matching the driver
  why_not_reuse_the_top_level_functions: a transactional read must go through the transaction handle, and a Context-carried handle would make one call site mean two different things; the tx is an argument, per decision:firestore-context-client-api
  client: still from the Context; the transaction adds a handle, not a client
what_is_not_hidden:
  closure_re_runs: ABORTED restarts the whole closure, and the godoc says so at the entry point rather than in a limitations list
  side_effects: a closure that writes a file or sends a message can do it several times; stated, not enforceable
  round_trips: begin, then the reads, then commit; three at minimum, and the wrapper adds none
  retry_budget: the driver's WithTxRetries, passed through; firestorebind adds no second loop, per rule:firestorebind-driver-passthrough
  queued_writes_are_not_sent_yet: tx.Store returns nothing because nothing happened; a caller who expects an error there is expecting the wrong shape, and the naming says so by returning none
what_stays_out:
  cross_type_helpers: a transaction spans types, so there is no single-type generic that covers one; the Tx methods are per-value and the composition is the caller's
  generated_transaction_bodies: a transaction is application logic, not an access pattern; requirement:firestore-typed-queries declares reads, not read-modify-write sequences
  a_retry_wrapper_of_our_own: the driver already restarts on ABORTED, and a second loop multiplies the deliveries its own documentation bounds
open:
  tx_in_declared_queries: whether a declared query emits a second form taking a *Tx, or one form taking an interface both satisfy; decided when requirement:firestore-typed-queries is built
  single_use: the driver folds begin into the first call where the shape allows; nothing here needs to know, unless a measurement later says the wrapper defeats it
related:
  - api:firestorebind-operations
  - rule:firestore-tag-options
  - rule:firestorebind-driver-passthrough
  - decision:firestore-context-client-api
  - requirement:firestore-typed-queries
  - system:tinygodriver-firestore
  - requirement:dynamo-optimistic-locking
```
