---
id: rule:dynamobind-driver-passthrough
type: rule
title: dynamobind Driver Passthrough
---
The binding layer adds typing only; it never absorbs an error, a retry, or a page boundary that the driver made visible.

```yaml
status: required
errors:
  - wrap with %w or not at all
  - errors.Is against ErrItemNotFound, ErrConditionalCheck, ErrThrottled and every other driver sentinel keeps working
  - errors.As to *dynamodb.Error keeps working, including Retryable and RequestID
  - a GetItem miss stays ErrItemNotFound and is never converted to a zero value
retry:
  - add no retry loop
  - reason: the driver already retries with backoff and documents that a write can be delivered attempts x 2 times, so a second loop multiplies that silently
  - unprocessed batch entries are returned to the caller, never resubmitted here
pagination:
  - api:dynamobind-operations QueryPage stays public even though the iterator exists
  - the iterator is a convenience over QueryPage, not a replacement, because it makes the request count invisible
  - LastEvaluatedKey is returned, not hidden; a non-nil key means more pages whatever Count says
options:
  - driver option values pass through untouched; dynamobind defines no parallel option type
related:
  - api:dynamobind-operations
  - system:tinygodriver-dynamodb
  - requirement:dynamobind-product-goals
```
