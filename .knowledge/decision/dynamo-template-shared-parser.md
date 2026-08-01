---
id: decision:dynamo-template-shared-parser
type: decision
title: .tb.dynamo Moves Packages, Not Grammars
---
Move the DynamoDB access-pattern parser into templates/dynamobind so it sits beside the other two formats, and leave its grammar alone, because the two grammars differ where it matters most.

```yaml
review_gate: implemented 2026-08-02; the grammar question stays open
source:
  - requirement:template-source-formatting
  - decision:template-parser-delegation
  - requirement:dynamo-typed-queries
what_moved:
  from: generator/dynamoquery.go
  to: templates/dynamobind, beside templates/htmlbind and templates/sqlbind, under decision:template-package-boundaries
  mechanism: the generator keeps its DynamoQueryDecl names as aliases, so the planning and emission stages are untouched
  gain: the format package may import templates/internal, which is what a printer needs and what a package under generator could never have
what_did_not_move:
  parser: the hand-written lexer and parser stay, and .tb.dynamo is still not parsed by templates/internal/syntax
  reason_found_during_implementation:
    rule: a dynamo declaration takes its Go visibility from the case of its name, and the shared parser takes it from an export keyword while requiring PascalCase
    evidence: generator/dynamoquery_test.go declares "statement readingsAround", which the shared parser rejects outright
    consequence: registering the format against the shared parser is a language change to every existing .tb.dynamo file, not a refactor
    conclusion: the change may still be right, but it is a decision about the language and has to be made as one
    resolved_by: decision:declaration-name-policy turns that difference into a registration flag, so the migration is no longer a language change and the blocker recorded here is gone
formatter_impact:
  none: the format is small and closed, so its printer is direct rather than tree-driven, and it shares nothing with the other two but syntax.Printer
  comments: requirement:template-comment-retention is met by a second scan over the source, because the token lexer drops comments and widening it would touch the parser this decision left alone
  cost: the declaration header of a .tb.dynamo file is printed by its own code rather than by the shared module printer, which is the duplication accepted here
revisit_when:
  - now: decision:declaration-name-policy settled the visibility rule, so the parser may move
  - .tb.dynamo needs type, enum, or external declarations, which the shared parser already has
related:
  - requirement:template-source-formatting
  - decision:template-parser-delegation
  - decision:template-package-boundaries
  - decision:template-declaration-kinds
  - requirement:dynamo-typed-queries
  - requirement:template-comment-retention
```
