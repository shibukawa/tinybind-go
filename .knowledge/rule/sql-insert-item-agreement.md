---
id: rule:sql-insert-item-agreement
type: rule
title: INSERT Column and Value Agreement
---
An INSERT whose column count can disagree with its value count on some branch path is a generation error, because the counts are a property of the template rather than of runtime data.

```yaml
priority: must
status: implemented 2026-08-17
closes: the hazard rule:sql-predicate-group-elision opened when comma groups landed
problem:
  cause: a column list and a VALUES tuple are two independent comma groups, so nothing pairs a conditional column with its conditional value
  without_the_check: the mismatch reaches the database as a runtime error on the branch that unbalances it, which is the failure mode rule:sql-static-mutation-safety exists to prevent one class of
  why_it_is_decidable: the item counts on each path are fixed by the template; only which path runs is a runtime question
mechanism:
  state: one integer per path, the columns counted so far minus the values counted so far, plus a flag for an item that has content and no separator yet
  item: a maximal run of content between commas at the list's own depth; a run with no content counts as no item, which is what makes an elided item vanish from the count
  content: every token the lexer emits, literals and quoted identifiers included, per the content-consumer setting in rule:sql-top-level-keyword-scan; an item spelled only 'bid' or only "id" is an item like any other
  content_is_not_optional: reading the keyword scan's token stream instead makes a literal-only item scan as empty, so a matched INSERT is reported as a disagreement; that was the second implementation's defect
  accept: every reachable path ends at zero
  scope: a statement whose top-level verb is INSERT, per rule:sql-top-level-keyword-scan
condition_correlation:
  why_it_is_required: the same condition guarding a column and its value is one runtime question, not two; a plain union over two independent merges reports the recommended shape as a disagreement, which was the first implementation's defect
  how: each path records which way every condition it passed was resolved, and re-entering a condition follows the branch that path already committed to
  identity: syntax.ExprString, which reinserts parentheses from precedence rather than remembering them, so two spellings of one condition compare equal
  consequence: correlated conditions are exact and genuinely independent ones still multiply into a product of paths
unresolvable_rather_than_reported:
  - a body the scanner cannot resolve
  - a second VALUES tuple, which requirement:sql-template-v1 already excludes with 'no bulk insert'; summing tuples is not what the check means
  - a form carrying only one of the two lists, such as INSERT INTO t VALUES or INSERT INTO t (cols) SELECT
  - a sql.predicate call inside a list whose body is not provably non-empty, which makes an item's content a runtime question
  - a path set past a bound, so a pathological template is left unchecked rather than reported on partial information
  stance: each of these declines to decide rather than guessing, on the same ground rule:sql-static-mutation-safety refuses an unproven clause
acceptance:
  - 'a static INSERT generates'
  - 'one condition guarding a column and its value generates'
  - 'two independent conditions each guarding a matched pair generates'
  - 'an if/else choosing a column and the same if/else choosing its value generates'
  - 'a conditional column with an unconditional value is reported'
  - 'a conditional value with an unconditional column is reported'
  - 'a column guarded by one condition and a value guarded by another is reported'
  - 'an if/else on the column side against a bare if on the value side is reported'
  - 'a function call inside an item is one item, not two'
  - "a literal item, first, middle, or last, counts as one item: VALUES ({id}, 'bid', {n}) against three columns generates"
  - 'a dollar-quoted literal counts as one item'
  - 'a quoted identifier in the column list counts as one column'
  - 'one condition guarding a column and its literal value generates'
  - "VALUES ({id}, 'bid') against three columns is still reported"
  - 'a condition guarding a literal value but not its column is still reported'
related:
  - rule:sql-predicate-group-elision
  - requirement:sql-conditional-predicate-composition
  - rule:sql-static-mutation-safety
  - rule:sql-top-level-keyword-scan
  - requirement:sql-template-v1
  - requirement:analysis-diagnostics
```
