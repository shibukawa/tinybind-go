---
id: rule:sql-template-layout
type: rule
title: SQL Template Body Layout
---
Lay out a statement body from the token stream the generator already scans, so CTEs, clauses, joins, and subqueries each get their own line and depth.

```yaml
status: implemented 2026-08-02
applies_to: requirement:template-source-formatting
bounded_by: rule:template-format-fidelity sql_token_identity and sql_regions
substrate:
  scanner: the rule:sql-top-level-keyword-scan lexer already carries parenthesis depth across text nodes, skips literals, quoted identifiers, and comments, and records byte offsets
  extension: a layout mode emits those skipped regions as opaque tokens instead of dropping them, so a comment or a literal has a position to be placed at and is never reflowed internally
  control_nodes: an embedded expression, if, or for node is one more opaque token in the same stream, so a clause spanning a control boundary is still one clause
  why_no_sql_parser: laying out clauses needs keyword positions and nesting depth, which the scanner already yields; a real SQL grammar would have to be dialect-specific and is not owed by an indentation pass
line_starts:
  keywords: [WITH, SELECT, FROM, every JOIN form, WHERE, GROUP BY, HAVING, WINDOW, ORDER BY, LIMIT, OFFSET, FOR UPDATE, RETURNING, INSERT INTO, VALUES, UPDATE, SET, DELETE FROM, ON CONFLICT, UNION, INTERSECT, EXCEPT]
  condition: recognized only at the nesting depth of the statement being laid out, which is exactly what makes a subquery's WHERE indent under its own SELECT rather than align with the outer one
  reuse: the same depth-zero test the mutation and access-mode checks already run, so a keyword inside a literal or a comment starts no line
indentation:
  unit: two spaces, one level per parenthesis depth the layout opened
  clause_items: the items of a clause indent one level below their keyword
  cte:
    form: 'WITH name AS ( opens a level, the body is laid out as its own statement, and the closing paren returns to the WITH level'
    chain: a following comma stays with the closing paren so the next CTE name starts a line
  subquery:
    laid_out: a parenthesized SELECT becomes its own indented statement
    left_alone: a parenthesized value list, function argument list, or IN list is data, not a statement, and stays inline until the width forces a break
  join: each JOIN form starts a line at the clause level, and its ON continues on the next line one level in
  boolean: a top-level AND or OR inside WHERE, HAVING, or ON starts a line at the clause body level, with the operator leading so the conditions align under each other
  select_list: one item per line when the list exceeds the width, one line otherwise
comments:
  line_comment: ends its line; whatever followed it in the source moves to the next line, because joining would comment that content out
  block_comment: stays on the line it begins and is never reflowed internally
  attachment: a comment stays immediately before the token it preceded
control_flow:
  inline: an if or for that opens inside a clause stays where it sits, since its branches are fragments of one clause rather than clauses of their own
  block: an if whose branches each begin with a clause keyword at statement depth puts its branches on their own lines, one level in, with else and the closing marker at the opening's indentation
not_done:
  keyword_case: unchanged; the scanner knows a keyword position but this module owns no dialect keyword list, and case is a choice the author may have made deliberately
  literal_contents: untouched, so a multi-line dollar-quoted string keeps its own newlines and defeats the line width for its whole extent
  dialect_rewrites: no placeholder, cast, alias, or function spelling is changed
related:
  - requirement:template-source-formatting
  - rule:template-format-fidelity
  - rule:sql-top-level-keyword-scan
  - decision:template-formatter-architecture
  - requirement:sql-template-v1
```
