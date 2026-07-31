---
id: decision:godoc-help-precedence
type: decision
title: Godoc Help Precedence and Backfill
---
The help tag is the single source of truth for descriptions; godoc comments only seed it, and the generator writes the seeded value into the source struct tag.

```yaml
status: accepted
precedence:
  - help tag value when present, even when empty-looking
  - field doc comment above the field
  - field trailing line comment
  - no description
struct_doc:
  source: doc comment above the struct type declaration
  target: Definition.Doc
  no_tag_equivalent: struct-level text has no tag form and is not backfilled
backfill:
  chosen: generator rewrites the declaring source file to add help:"..."
  rejected_alternative: resolve godoc at generate time only and leave source untouched
  rationale:
    - tag stays the single readable source of truth after one run
    - reviewers see the exact CLI and scaffold text in the struct
    - imported or vendored config structs behave the same as local ones once tagged
    - codegen keeps a tag-only input contract
  consequences:
    - generator writes hand-written source, not only *_gen.go files
    - doc comment and tag can drift later; the tag wins by precedence
    - backfill must be idempotent and formatting-safe per rule:help-tag-backfill
    - a wrong generated tag is fixed by editing the tag, not the comment
opt_out: generator Feature "help-backfill" disables the source rewrite; godoc still seeds help in generated output
scope:
  applies_to:
    - api:configbind-bind option fields
    - api:configbind-subcommand option and positional fields
  excluded:
    - unexported fields
    - fields in generated files
    - non-config structs
related:
  - requirement:godoc-config-descriptions
  - decision:struct-field-tags
  - decision:single-source-of-truth
  - rule:help-tag-backfill
  - rule:godoc-comment-normalization
  - flow:configbind-codegen
  - system:configbind
```
