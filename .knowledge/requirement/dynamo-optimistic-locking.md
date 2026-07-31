---
id: requirement:dynamo-optimistic-locking
type: requirement
title: Optimistic Locking From A version Tag
---
A version tag makes a write conditional on the version it read, and bumps it, so the most repeatedly hand-written DynamoDB pattern stops being hand-written.

```yaml
status: proposed
source: decision:dynamo-framework-requests
tag: "dynamo:\"version,version\""
field_type: an integer attribute; a non-integer field is a generation error
behavior:
  first_write: a zero version stores version 1 with a condition that the attribute does not exist
  later_write: version n stores n+1 with a condition that the stored version is still n
  conflict: the driver answers ErrConditionalCheck, which reaches the caller unchanged per rule:dynamobind-driver-passthrough
  read: the decoder fills the field like any other, so the value a caller writes back is the one it read
consequence_for_store:
  problem: "Store[T ItemEncoder] takes its value, so a bumped version cannot reach the caller's struct"
  effect: a caller that writes twice from one value would send the same version twice and lose the second write to a conflict it did not cause
  options:
    pointer_receiver: a versioned type's write takes *T, so the bump is visible; it splits the signature by whether a tag is present, which is the kind of surprise generation should not add
    returning_form: "StoreVersioned returns the stored version, and the caller assigns it"
    caller_rereads: no signature change, and the pattern's whole point is avoiding the extra read
  recommendation: the returning form, since api:dynamobind-operations already pairs a plain call with a returning one and this is the same shape again
  open: which of the three, decided when this is implemented rather than now
interaction_with_conditions:
  fact: the generated condition occupies the ConditionExpression the caller may also want
  rule: a caller-supplied dynamodb.WithCondition and a version tag on the same write is an error rather than a silent merge, until an expression builder exists to combine them
  why: silently ANDing two expressions written by different authors is how a condition stops meaning what its author read
scope:
  in: Store and the batch-free single-item writes
  out: StoreAll, because BatchWriteItem takes no condition at all; a versioned type in a batch write is a generation error rather than an unchecked write
related:
  - rule:dynamo-tag-options
  - api:dynamobind-operations
  - rule:dynamobind-driver-passthrough
  - decision:dynamo-framework-requests
```
