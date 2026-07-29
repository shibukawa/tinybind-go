---
id: rule:sql-statement-access-mode
type: rule
title: Statement Access Mode Derivation
---
A statement is read-only only when its top-level verb provably reads; everything else is a write.

```yaml
priority: must
detection: rule:sql-top-level-keyword-scan
applies_to: every sql.exec, sql.one, sql.optional, and sql.many declaration
consumers:
  - requirement:sql-read-only-executor
read_only_requires:
  - top-level verb SELECT, VALUES, or TABLE
  - or WITH whose CTE bodies contain no INSERT, UPDATE, DELETE, or MERGE and whose tail verb is SELECT, VALUES, or TABLE
  - and no top-level row-locking clause: FOR UPDATE, FOR NO KEY UPDATE, FOR SHARE, FOR KEY SHARE
write_otherwise:
  - any other leading verb, including INSERT, UPDATE, DELETE, MERGE, CREATE, DROP, TRUNCATE, COPY, CALL, and GRANT
  - an unrecognized leading token
  - a body the scanner cannot resolve
rationale: misclassification can only waste a reader connection, never send a write to a read-only executor
leading_token:
  - skip leading whitespace, line comments, and block comments before reading the verb
  - a literal cannot precede the verb, so quote handling matters only for the WITH tail and the lock-clause scan
branch_handling:
  - a body whose first content node is not literal text is a write, because the leading verb would depend on a runtime branch
  - the lock-clause scan reads every branch, so a locking clause emitted on any path makes the statement a write
  - branch text is joined with a separator so adjacent fragments cannot fuse into one token
acceptance:
  - 'select id from t is read-only'
  - 'delete from t where id = {id} returning id declared sql.one is a write'
  - 'select id from t for update is a write'
  - 'with x as (select 1) select * from x is read-only'
  - 'with x as (insert into t values (1) returning id) select * from x is a write'
  - 'a leading line comment naming update does not make the statement a write'
  - 'a leading block comment naming delete does not make the statement a write'
  - "select 'update t' from x is read-only"
  - 'a backtick or double-quoted identifier named update does not make the statement a write'
  - 'a for-update inside a subquery is not a top-level lock clause'
  - 'a body opening with an if node is a write on every branch'
  - 'an unterminated literal, identifier, or block comment is a write'
```
