---
id: rule:help-tag-backfill
type: rule
title: Help Tag Backfill
---
The generator inserts a help tag into a config struct field only when the tag is absent and a normalized comment exists, and the rewrite is idempotent.

```yaml
when:
  - field belongs to a struct reached from api:configbind-bind or api:configbind-subcommand
  - field is exported
  - field has no help key in its struct tag
  - rule:godoc-comment-normalization yields non-empty text
then: add help:"text" to that field tag in the declaring source file
never:
  - overwrite or reorder an existing help tag
  - modify a field with no usable comment
  - modify unexported fields
  - modify files ending in _gen.go or files carrying a generated-code header
  - modify files outside the analyzed package directory
  - delete or edit the doc comment itself
tag_placement:
  - append help after existing keys, preserving their order
  - create a backtick tag literal when the field has none
  - keep one space between tag keys
  - run gofmt on the rewritten file so field alignment is restored
idempotency:
  - a second run finds the help tag and makes no change
  - byte-identical output for unchanged input
ordering:
  - backfill runs before IR construction in flow:configbind-codegen
  - the same run emits code from the backfilled tags
  - a failed rewrite aborts generation before any file is emitted
failure:
  - unparsable source file: report and skip the file, do not partially write
  - description containing a backtick: report and skip the field
  - read-only file: report path and abort
reporting:
  - rewrites happen in place; the version-control diff is the review surface
  - no per-field log is emitted today; Options carries no writer for it
applies_to:
  - requirement:godoc-config-descriptions
  - decision:godoc-help-precedence
related:
  - rule:godoc-comment-normalization
  - decision:struct-field-tags
  - rule:generated-source-self-contained
  - flow:configbind-codegen
  - system:configbind
```
