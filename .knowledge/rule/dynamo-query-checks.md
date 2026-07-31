---
id: rule:dynamo-query-checks
type: rule
title: DynamoDB Query Declaration Checks
---
Every attribute a query declaration names is checked against the bound type's tags, and each clause accepts only the attributes DynamoDB allows there.

```yaml
status: required, implemented 2026-07-31 in generator/dynamoquery_plan.go
applies_to: requirement:dynamo-typed-queries
name_validation:
  rule: an attribute named in a declaration must exist on the bound type, per rule:dynamo-tag-options
  failure: a generation error naming the declaration, the attribute and the type
  effect: this is what closes the drift; a declaration alone is still text, and the check is what ties it to the tag
  possible_here: the tags are the schema, so the check the SQL template cannot make is available
clause_sets:
  key:
    accepts: the partition key, required and equality only, and the sort key, at most one predicate
    sort_predicates: "=, <, <=, >, >=, BETWEEN, begins_with; begins_with only on a string sort key"
    rejects: any other attribute, with a message naming the clause it belongs in
  filter:
    accepts: non-key attributes only
    rejects: a key attribute, which DynamoDB itself rejects, with a message naming the key clause
  disjoint: the two sets never overlap, so both directions are checkable
never_merge_the_clauses:
  rule: a declaration has separate key and filter clauses, never one where
  cost: a filter reads and discards, so capacity is spent on what it drops; moving a predicate between the clauses decides a cost and latency question that belongs to the author
  no_planner: DynamoDB has no query planner and the split is visible in the request; a generator cannot infer it, having no view of the data distribution
index_selection:
  rule: infer the index when exactly one keys on the named attributes; require an explicit index when several could, or when the condition spans two
  visibility: the chosen index is written into the generated code, so an inference is never silent
  deferred_until: secondary index tags exist, per rule:dynamo-tag-options; until then a declared query runs against the table's own keys
parameter_types:
  rule: a declared parameter type is checked against how the tag stores the attribute, S or N
  reason: DynamoDB has no date or time type, so a parameter type is a claim about the stored form rather than about the Go type alone
reserved_words:
  rule: alias every attribute name unconditionally, as "#n0" and its siblings
  reason: DynamoDB reserves 573 words, including status, name, size, type, data, year, count and timestamp, and an expression naming one literally fails with ValidationException
  cost: none, because the alias and its name map are constants fixed at generation time
  no_list: aliasing everything means no reserved-word list is carried or kept current, and no per-name branch exists to get wrong
related:
  - requirement:dynamo-typed-queries
  - rule:dynamo-tag-options
  - api:dynamobind-operations
  - requirement:analysis-diagnostics
```
