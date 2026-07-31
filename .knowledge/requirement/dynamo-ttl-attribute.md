---
id: requirement:dynamo-ttl-attribute
type: requirement
title: TTL Attribute Tag And Table Specification
---
A ttl tag names the attribute DynamoDB expires items by, and carries it into the generated table definition, so an expiring record is declared once rather than enabled by hand after every deploy.

```yaml
status: blocked upstream
source: decision:dynamo-framework-requests, the tinybind half of the framework's session and auth-state backend
tag: "dynamo:\"expires,ttl\""
field_type: time.Time, stored as epoch seconds, which is the only form DynamoDB reads for expiry
relation_to_unixtime:
  fact: ttl implies the unixtime encoding rather than sitting beside it
  reason: DynamoDB expires on an epoch-second number attribute and reads nothing else, so a ttl field encoded as RFC 3339 would simply never expire
  rule: a ttl tag with an explicit conflicting encoding is a generation error
generated:
  table_definition: the table gains a TTL specification naming the attribute, per decision:dynamobind-table-definition
  codec: unchanged beyond the unixtime encoding the tag implies
  checks: at most one ttl field per type
blocked_on:
  upstream: the driver declares no UpdateTimeToLive, and TTL is excluded by name in its own scope, per system:tinygodriver-dynamodb upstream_requests
  what_cannot_be_done_meanwhile: nothing here can enable expiry on a real table; a generated TableDefinition can carry the attribute name, but no call applies it
  partial_value_before_that: the tag and the encoding are still worth having, because they name the attribute one place and a migration tool can read it; the gap is the apply step, not the declaration
  documentation_duty: until the driver call exists, the guide must say that expiry is enabled by the operator, since a TTL attribute that nothing acts on looks like a working feature and silently retains every record
why_it_matters_downstream:
  session_backend: a session store without expiry grows without bound and pays for every record it will never read again
  migration_integrity: a framework whose migration claims to apply a table's desired state cannot claim it while TTL is outside the state it can apply
related:
  - decision:dynamobind-table-definition
  - rule:dynamo-tag-options
  - system:tinygodriver-dynamodb
  - decision:dynamo-framework-requests
```
