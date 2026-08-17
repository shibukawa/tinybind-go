---
id: requirement:sql-conditional-predicate-composition
type: requirement
title: Conditional SQL Predicate Composition
---
A conditional predicate's source reads as the fully-populated SQL with conditions punched out of it; the generator withholds the joiner, grouping paren, and clause keyword that a vanished condition would leave dangling.

```yaml
origin:
  source: downstream framework change request 2026-08-17, against v0.5.14, Popcorn Wave at github.com/shibukawa/popcornwave
  disposition: decision:sql-boundary-joiner-inference
status: implemented 2026-08-17 for boolean clauses WHERE, HAVING, QUALIFY, and join ON; comma clauses not done
refines: requirement:sql-template-v1 structured_lists, which already promises 'AND children by default' and 'omit when empty for SELECT'
gap:
  implemented: flat text; emitNodes at templates/sqlbind/generate.go:511 emits a TextNode verbatim inside a plain Go if, with no separator logic anywhere
  consequence: the operator joining two conditions is SQL bytes the author owns and cannot get right on every branch
  scale: a two-and-three-condition predicate is wrong in 13 of its 16 branch combinations
target_property:
  statement: read the template with every condition true and the if wrappers deleted; that is the SQL it must render
  corollary: every operator sits between its two operands, in the enclosing text, belonging to the pair rather than to either side
  canonical_form: 'WHERE {if a}name LIKE {name}{/if} AND {if b}city = {city}{/if} AND ({if c}age >= {min}{/if} OR {if d}role = ''staff''{/if})'
acceptance:
  - all conditions true renders byte-identical to today, which is the whole compatibility story
  - 'one middle condition only renders WHERE city = $1, not WHERE AND city = $1 AND ( OR )'
  - no condition true renders no WHERE clause at all for a SELECT
  - a nested group that stays empty takes its parentheses and the joiner that attached it
  - placeholder numbering and Args are unchanged in every branch combination
constraints:
  no_new_syntax: no marker, no block form, no clause construct, no node kind, no scope name
  no_generated_punctuation: the builder withholds a token the author wrote; it never invents a grouping paren
  no_post_render_pass: nothing scans, trims, or rewrites the assembled string at generation or run time, per the requirement:sql-template-v1 ban on a generated runtime check for a generation-time condition
  mutation_safety_unrelaxed: rule:sql-static-mutation-safety keeps its full strength; reject this request before weakening it
mechanism: rule:sql-predicate-group-elision
rationale: decision:sql-boundary-joiner-inference
scope:
  phase_one: boolean clauses WHERE, HAVING, QUALIFY, and join ON
  phase_two: comma clauses, with SET last because it additionally carries the mutation proof
  excluded: conditional result columns and a general for in a clause stay forbidden for their existing reasons
  dialect: a boolean joiner and a paren are not what a dialect differs about, so the frame protocol is identical everywhere
related:
  - requirement:sql-template-v1
  - rule:sql-predicate-group-elision
  - decision:sql-boundary-joiner-inference
  - rule:sql-static-mutation-safety
  - rule:sql-top-level-keyword-scan
  - rule:sql-placeholder-emission
  - concept:typed-template-language
```
