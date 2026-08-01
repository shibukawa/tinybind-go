---
id: decision:template-annotation-syntax
type: decision
title: Template Declaration Annotation Syntax
---
Attach declaration metadata through `@name(key: value)` lines preceding a declaration instead of growing the keyword prefix.

```yaml
source:
  - decision:template-declaration-kinds
  - user syntax decision 2026-07-26
review_gate: approved 2026-07-26
shape: one or more `@name(key: "value", ...)` lines directly above a declaration
argument_form:
  keys: lowerCamelCase names
  values: string literals only, parsed and validated at generation time
  empty: `@name` with no parenthesis is allowed
placement:
  target: the next declaration in the file
  visibility: the `export` keyword stays on the declaration line, so visibility and behavior stay visually separate
  repetition: a name may appear at most once per declaration
parser:
  owner: shared module parser, so every template format gets the same annotation grammar
  storage: annotation list on the declaration node; each format decides which names it accepts
  unknown_name: generation error naming the declaration, because a silently ignored annotation reads as enabled
rejected:
  keyword_prefix: `export cache(ttl: "5m") component X` pushed behavior into the visibility slot and lengthened the declaration line as modifiers accumulate
  trailing_clause: `: html cache(ttl: "5m")` separated the modifier from the name it modifies
consumers:
  - decision:cache-component-declaration
  - future requirement:partial-update-boundaries and route-role markers
non_consumers:
  async: requirement:async-external-functions keeps the `external async` keyword, because it changes the required Go signature rather than annotating behavior
```
