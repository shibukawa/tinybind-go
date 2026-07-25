---
id: rule:sql-top-level-keyword-scan
type: rule
title: Top-Level SQL Keyword Scanning
---
Static SQL analysis recognizes a clause keyword only at the analyzed statement's own nesting level, never inside a subquery, literal, or comment.

```yaml
priority: must
applies_to: every generation-time SQL inspection in templates/sqlbind
problem:
  - current analysis splits emitted text on whitespace and matches any occurrence
  - a subquery WHERE is counted as the outer statement's WHERE
  - a keyword inside a string literal or comment is counted as SQL syntax
  - a flattened string merges both branches of an if, hiding which path emitted what
rules:
  - tokenize emitted SQL text; do not split on whitespace
  - track parenthesis depth and accept a clause keyword only at depth zero of the analyzed statement
  - skip single-quoted literals, dollar-quoted literals, double-quoted identifiers, line comments, and block comments
  - match keywords case-insensitively and on whole tokens only
  - walk the template node tree, keeping if branches distinguishable, instead of analyzing one concatenated string
  - treat a CTE body as its own nesting level; the outer statement is the WITH tail
consumers:
  - rule:sql-static-mutation-safety
  - rule:sql-cardinality-body-agreement
  - result shape validation in requirement:sql-template-v1
acceptance:
  - 'DELETE FROM t USING (SELECT id FROM u WHERE u.flag) s WHERE false is reported as having a top-level WHERE'
  - 'DELETE FROM t USING (SELECT id FROM u WHERE u.flag) s is reported as having none'
  - 'UPDATE t SET c = (SELECT v FROM k WHERE k.id = 1) is reported as having none'
  - "SELECT '-- where' FROM t contains no WHERE"
  - 'SELECT a, (SELECT b FROM c) FROM t exposes two top-level result columns'
related:
  - requirement:analysis-diagnostics
  - requirement:sql-template-v1
```
