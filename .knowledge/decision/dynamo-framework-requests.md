---
id: decision:dynamo-framework-requests
type: decision
title: DynamoDB Scope After The Downstream Round
---
Three items are this module's, two go upstream to the driver, and four belong to the framework because the surface it needs already exists.

```yaml
source: downstream framework DynamoDB request 2026-07-31
review_gate: proposed
principle:
  seam: decision:framework-integration-seams, widen a seam whose default output stays identical and whose contract stays the caller's
  ownership: decision:route-feature-ownership, which puts developer experience, site-wide artifacts and transport concerns with the framework
this_module:
  - what: requirement:dynamo-typed-queries
    value: highest; the codec typed the item and left the read path a string
    order: first
  - what: requirement:dynamo-optimistic-locking
    value: high per byte, and the tag surface is one option
    order: beside the queries, being independent of that surface
  - what: requirement:dynamo-ttl-attribute
    value: the tinybind half of a session backend
    order: blocked until the driver can apply a TTL
upstream:
  where: system:tinygodriver-dynamodb upstream_requests
  what: UpdateTimeToLive first, then UpdateTable for adding a secondary index
  reason: one driver change unlocks the framework's session backend, the TTL half of its migration, and requirement:dynamo-ttl-attribute here
framework_owns:
  request_reproduction:
    reason: dynamobind builds an item and hands it to the driver, so it is not in the request path
    available: a captured HTTP body is already the exact CLI input, with no placeholder to reconstruct
  offline_doctor_checks:
    available: AnalyzeDynamoItemsWithOptions reports every bound type and tag error without a network, and GenerateArtifacts returns artifacts without writing a file
    framework_side: endpoint reachability and the deployed-versus-generated diff, because the table name prefix is the framework's
  seed_and_assert:
    path: decode fixture data with jsonbind, then EncodeItem; assert by scanning and comparing decoded values
    note: the fixture-to-item direction composes out of the two codecs, so no SQL-shaped fixture runner is needed
  paging_cursor:
    available: a dynamodb.Key round trips through encoding/json without loss, measured over a 38-digit number, high-byte binary and a NUL-bearing multi-byte string
    caution: a cursor is a table position rather than an authorization, so a signature must cover whatever scopes the query
key_conditions_are_a_closed_grammar:
  distinction: partition key equality with at most one sort key predicate, over attributes already typed by the key tags
  effect: requirement:dynamo-typed-queries does not reopen the update and condition expressions api:dynamobind-operations defers, whose grammar is open
related:
  - requirement:dynamobind-product-goals
  - api:dynamobind-operations
  - system:tinygodriver-dynamodb
  - decision:dynamo-single-table-scope
  - rule:dynamo-tag-options
```
