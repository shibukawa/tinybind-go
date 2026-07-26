---
id: requirement:godoc-config-descriptions
type: requirement
title: Godoc Descriptions for Config Structs
---
Generator reads godoc comments on config struct types and fields, backfills missing help tags into source, and emits the text into TOML scaffolds, .env scaffolds, and CLI usage.

```yaml
priority: should
intent: keep one human description per config field without writing it twice
problem: help tag is optional; untagged fields render bare keys with no comment
sources:
  - doc comment above a Bind or SubCommand struct type declaration
  - doc comment above a struct field
  - trailing line comment on a struct field
precedence: decision:godoc-help-precedence
normalization: rule:godoc-comment-normalization
backfill: rule:help-tag-backfill
consumers:
  - TOML scaffold comment above the [prefix] table from struct doc
  - TOML scaffold comment above each key from field help
  - env scaffold comment above each variable from field help
  - CLI usage help from field help via data:cli-flag-def
  - subcommand usage description from struct doc when the help argument is empty
mechanism:
  - generator correlates go/types fields with AST fields to recover comments
  - generator writes missing help tags back to the declaring source file
  - codegen reads only tags afterward; emitted code carries no comment lookup
  - configbind.Definition gains a Doc field carrying the struct doc text
  - data:config-scaffold-fragment carries Doc plus per-field Help
constraints:
  - go/types *types.Struct exposes no comments; AST correlation is required
  - only same-package struct declarations are readable; imported config structs keep tag-only help
  - generated files are never scanned for comments and never backfilled
  - env scaffold sorts globally by variable name, so struct doc has no stable position and is omitted
  - runtime stays reflection-free per decision:configbind-codegen-no-reflect
related:
  - requirement:struct-field-metadata
  - requirement:scaffold-generation
  - requirement:cli-option-codegen
  - requirement:cli-subcommands
  - decision:struct-field-tags
  - decision:godoc-help-precedence
  - decision:configbind-codegen-no-reflect
  - rule:godoc-comment-normalization
  - rule:help-tag-backfill
  - data:cli-flag-def
  - data:config-scaffold-fragment
  - concept:scaffold-templates
  - flow:configbind-codegen
  - api:config-scaffold-output
  - system:configbind
acceptance:
  - field with help tag keeps that text and its godoc is ignored
  - field without help tag and with a doc comment gains help:"..." in the source file
  - field without help tag and without any comment stays untagged and renders no comment
  - regenerating after backfill produces no further source change
  - struct doc renders as # comment lines directly above [prefix] in scaffold TOML
  - nested struct doc renders above its nested table or is omitted for dotted keys
  - backfilled help appears in generated CLI usage without a second generator run
  - subcommand with empty help argument falls back to its struct doc
  - env scaffold shows field help comments and no struct doc
  - disabling the help-backfill feature leaves sources untouched and still seeds help
```

