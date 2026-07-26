---
id: rule:godoc-comment-normalization
type: rule
title: Godoc Comment Normalization
---
Convert a Go doc comment into one single-line description deterministically before it becomes a help tag or a scaffold comment.

```yaml
input:
  field: ast.Field.Doc, else ast.Field.Comment
  type: ast.TypeSpec.Doc, else ast.GenDecl.Doc
steps:
  - strip // and /* */ markers from each line
  - drop directive lines matching //go:, //nolint, //lint:, //revive:, //nosec
  - drop lines that are empty after trimming
  - stop at the first blank line; keep only the first paragraph
  - join remaining lines with a single space
  - collapse runs of whitespace to one space and trim
  - drop one trailing period
  - result is empty when nothing remains
name_prefix:
  decision: keep the leading Go field or type name
  example_in: '// Port is the HTTP listen port.'
  example_out: 'Port is the HTTP listen port'
  rejected: strip "Name is " to get "the HTTP listen port"
  rationale:
    - stripping is lossy and produces sentence fragments in --help
    - keeping the name is reversible and never mangles hand-written text
    - authors who dislike it edit the backfilled tag once
tag_encoding:
  - value is escaped with Go string quoting before insertion into the struct tag
  - a description containing a backtick would break the raw tag literal, so the field is skipped by rule:help-tag-backfill but still receives the help text in generated output
  - resulting tag stays on one line
multiline_scaffold:
  - scaffold comments still accept multi-line help from a hand-written tag
  - normalization output is single-line, so backfilled fields render one # line
applies_to:
  - rule:help-tag-backfill
  - requirement:godoc-config-descriptions
  - concept:scaffold-templates
related:
  - decision:godoc-help-precedence
  - decision:struct-field-tags
  - data:cli-flag-def
  - flow:configbind-codegen
```
