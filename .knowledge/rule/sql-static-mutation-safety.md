---
id: rule:sql-static-mutation-safety
type: rule
title: Static Mutation WHERE Safety
---
UPDATE and DELETE WHERE safety is proven at generation time from the template tree; no mutation guard is emitted into generated code.

```yaml
priority: must
supersedes: the generated _tinybindSafeMutation runtime guard
reason:
  - whether a WHERE clause can be empty is a property of the template, not of runtime data
  - the runtime guard re-parsed its own output with whitespace splitting, so a subquery WHERE satisfied it while the outer statement had none
  - the existing static check ran only for sql.exec, leaving a mutation with RETURNING unguarded
  - a check that can be decided at generation time must fail the build, not a request
scope: every statement whose top-level verb is UPDATE or DELETE, in every cardinality
detection: rule:sql-top-level-keyword-scan
rules:
  - a mutation must have a top-level WHERE clause
  - the WHERE clause must be provably non-empty on every branch path
  - the clause ends at the next top-level keyword, so content belonging to a later clause is not proof
  - clause_terminators: RETURNING, ORDER, LIMIT, OFFSET, FETCH, GROUP, HAVING, WINDOW, FOR, UNION, INTERSECT, EXCEPT
  - a predicate emitted only inside an if without an else that also emits one is not proof
  - an if/else where both branches emit a predicate is proof
  - a sql.predicate call is proof only when that predicate is itself provably non-empty on every path
  - a mutation failing the proof is a generation diagnostic at the declaration position, per requirement:analysis-diagnostics
  - generated Build<Component> contains no mutation-safety branch and cannot return that error
set_clause:
  - the same proof applies to a dynamic SET list
  - an UPDATE whose SET items are all conditional is a generation error, replacing the pre-execution empty check in requirement:sql-template-v1
full_table_mutation: still needs the future explicit opt-in named in requirement:sql-template-v1; without it an unconditional whole-table mutation stays a generation error
acceptance:
  - 'delete from t where {if flag} id = {id} {end} returning id generates a diagnostic'
  - 'delete from t where id = {id} generates'
  - 'delete from t generates a diagnostic'
  - 'delete from t {if flag} where id = {id} {end} generates a diagnostic'
  - 'delete from t where {if flag} a = {a} {else} b = {b} {end} generates'
  - 'delete from t using (select 1 from u where u.f) s generates a diagnostic'
  - 'update t set c = {c} where id = {id} returning id declared sql.one is checked the same way'
  - 'no generated file contains a mutation-safety helper or its error string'
group_elision_interaction:
  unrelaxed: rule:sql-predicate-group-elision may not weaken this proof; dropping an empty WHERE under a mutation turns one false branch into a full-table mutation
  two_changes_only:
    - proveClause runs against the group rather than against the keyword
    - a token recognized as a joiner counts as filling nothing
  cuts_both_ways: a conditional-only mutation predicate still fails, because the proof is about the group being provably non-empty rather than about whether the text renders
related:
  - requirement:sql-template-v1
  - data:sql-statement
  - rule:sql-cardinality-body-agreement
  - decision:generated-runtime-in-module
  - rule:sql-predicate-group-elision
  - requirement:sql-conditional-predicate-composition
```
