---
id: rule:firestorebind-driver-passthrough
type: rule
title: firestorebind Driver Passthrough
---
The binding layer adds typing only; it never absorbs an error, a retry, a transaction restart, or a batch boundary that the driver made visible.

```yaml
status: proposed
proposed: 2026-08-03
errors:
  - wrap with %w or not at all
  - errors.Is against ErrNoSuchEntity, ErrAlreadyExists, ErrAborted, ErrFailedPrecondition and every other driver sentinel keeps working
  - errors.As to *datastore.Error keeps working, including Retryable, Status and Op
  - a Load miss stays ErrNoSuchEntity and is never converted to a zero value
  - never key on an HTTP status; ABORTED and ALREADY_EXISTS are both 409 and mean opposite things, so any code here that discriminates does it on Status as the driver does
retry:
  - add no retry loop
  - reason: the driver retries with backoff and documents that a request can be delivered attempts x 2 times, so a second loop multiplies that silently
  - deferred lookup keys are returned to the caller, never resubmitted here
transactions:
  - the closure re-run on ABORTED is the driver's, and firestorebind neither adds nor suppresses one
  - a wrapper never swallows ErrAborted to make a transaction look like it succeeded
  - the closure-runs-more-than-once warning is repeated at the firestorebind entry point, because a caller reading only this package's godoc would otherwise not see it
pagination:
  - api:firestorebind-operations QueryPage stays public even though the iterator exists
  - the iterator is a convenience over QueryPage, not a replacement, because it makes the request count invisible
  - EndCursor and MoreResults are returned, not hidden; MoreResults says why a batch ended, and flattening it to a bool discards the reason
  - SkippedResults is carried, since those entities were read and billed
lookup_results:
  - found, missing and deferred stay three lists; a missing key is not an absent value and a deferred key is not a missing one
  - LoadAll returns values in the driver's reply order and does not sort them back to the caller's key order
options:
  - driver option values pass through untouched; firestorebind defines no parallel option type
  - ReadOption, WriteOption and TxOption stay the driver's separate interfaces, so a consistency option on a write remains a compile error
limits:
  - the driver exports its service limits as constants; firestorebind names them and declares none of its own
  - reason: a parallel constant is a copy, and a copy of a published number is what drifts when the service changes it; that was the argument for asking upstream to export them and it applies again one layer up
  - no literal limit appears in this package or in generated code; MaxLookupKeys, MaxRequestBytes and the rest are read from the driver
  - contrast: api:dynamobind-operations exported MaxBatchWrite and MaxBatchGet itself, which was right when the driver named neither and is now the thing to undo
consistency:
  - strong is the driver's default and firestorebind does not change it
  - WithEventualConsistency stays a caller decision, since it trades correctness for cost and nothing here can weigh that
precision:
  - a timestamp round trip truncates to microseconds, and the codec does not round or pad to hide it
  - an Integer stays text end to end; nothing routes it through float64
related:
  - api:firestorebind-operations
  - system:tinygodriver-firestore
  - requirement:firestorebind-product-goals
  - decision:firestore-transaction-scope
  - rule:dynamobind-driver-passthrough
```
