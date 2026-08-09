---
id: requirement:template-comment-retention
type: requirement
title: Template Comments Survive Parsing
---
The template AST keeps the comments the source wrote, because a formatter that drops them is a formatter nobody may run.

```yaml
status: implemented 2026-08-02; the tail acceptance below only held from 2026-08-09
priority: must, as a precondition for requirement:template-source-formatting
problem_that_was:
  shared_parser: templates/internal/syntax discarded every comment through skipSpaceAndComments, so no declaration, type, or parameter comment reached the AST
  dynamo: its lexer skipped a line comment during tokenization, so a header comment on a .tb.dynamo file was unrecoverable
  html: a markup comment already survived as an html:comment body node and needed no change
  sql: a comment inside a statement body already survived as raw SQL text and needed no change
  gap: the header grammar, which is exactly the part all three formats share
  closed_by: skipSpaceAndComments records what it skips instead of discarding it
retention_model:
  form: Module.Comments holds every declaration-part comment in source order; a printer walks its declarations in order and flushes what stands above each one
  why_not_per_node: attaching to nodes would touch every declaration type for no extra information, because ordered positions already answer the only question a printer asks
  keep: the comment text including its delimiters, whether it was a block comment, whether a blank line preceded it, and whether it trailed code on its own line
  attachment_rule: a comment on the line directly above a declaration is its documentation and no blank line is inserted between them; anything further above is detached and keeps its blank line
  backtracking: the parser re-scans the same bytes while peeking, so a comment is recorded by start offset and never twice
  dynamo: decision:dynamo-template-shared-parser keeps its own lexer, which drops comments, so its formatter recovers them with a second scan that records the line each one sits on
blank_line_visibility:
  measured_as: line breaks skipped since the previous content, so a blank line is two
  was_unreachable_after_a_line_comment: |
    the line-comment branch consumed its own terminating newline before
    continuing, which restarted the count, so a comment following a line comment
    was recorded with one break whether or not a blank line stood there. The two
    spellings parsed identically and no printer could tell them apart.
  fixed_by: leaving that newline for the whitespace scan, which is what the block-comment branch always did and why a block comment round tripped correctly throughout
  three_scanners: |
    the shared parser, modulePrinter, and declcomment each decide this
    separately. declcomment counts blank runs directly and was correct; it is the
    reference the other two were brought to, not a fourth opinion.
  printer_consequence: |
    a tail path that ignores the recorded blank re-spaces every run, and one that
    trusts a blank the parser cannot see deletes the spacing instead. Both halves
    are needed, and fixing only the printer trades one infidelity for the other.
non_goals:
  - interpreting a comment as documentation for generated Go, which api:generator-artifacts already sources from godoc on the Go side
  - a doc-comment convention that changes generated output; a comment stays a comment
acceptance:
  - parsing and printing a source with a comment before the package line, before a declaration, on a parameter line, and after the last declaration returns all of them in place
  - a blank line between two comments survives in both positions, and its absence survives too; a fixed point, not merely idempotent, since the wrong spacing settled as readily as the right one
  - existing compiler stages are unaffected, because they ignore the new nodes
  - a comment never becomes part of a generated artifact by accident, per rule:template-format-fidelity
related:
  - requirement:template-source-formatting
  - decision:template-formatter-architecture
  - decision:template-parser-delegation
  - decision:dynamo-template-shared-parser
  - rule:template-format-fidelity
```
