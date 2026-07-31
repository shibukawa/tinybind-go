---
name: knowledge-compiler
description: Create, maintain, compile, search, show, and export AI-first `.knowledge` catalogs that minimize source-token and context cost by using token-efficient English source concepts. Use when Codex needs to initialize or edit machine-first knowledge concepts, enforce OKF-compatible Markdown/frontmatter constraints, compile `.knowledge` caches, answer questions from compiled concept context, generate review Markdown from concepts, or work with YAML-encoded flow, DFD, or UI sketch structures.
---

# Knowledge Compiler

## Overview

Use this skill to manage `.knowledge` catalogs: many small, machine-first Markdown concept files with tiny frontmatter and YAML-heavy bodies. Write `.knowledge` source in token-efficient English to reduce LLM context cost and keep technical terminology stable. Let compiled cache files become verbose for deterministic search, relation traversal, and export.

The bundled CLI is self-contained and works without network access:

```bash
python3 <skill-dir>/scripts/concept.py compile --project .
python3 <skill-dir>/scripts/concept.py search --project . access
python3 <skill-dir>/scripts/concept.py show --project . api:access-check
python3 <skill-dir>/scripts/concept.py export --project . requirement:access-check --profile review
```

Use `python` only when `python3` is unavailable.

Vendored Python dependencies live under `vendor/python/` and are loaded by `scripts/concept.py` before system packages. See `README.md` for package authors and licenses.

## Project Layout

Use these locations unless the user has an existing `.knowledge` layout:

- `.knowledge/<type>/<name>.md`: source concept files.
- `.knowledge/_schema/types.yaml`: configurable concept type metadata.
- `.knowledge/_schema/categories.yaml`: configurable category metadata.
- `.knowledge/_schema/enums.yaml`: configurable enum metadata.
- `.knowledge/.cache/`: generated JSONL cache files. Treat as derived output.

For a new catalog, copy `assets/starter/.knowledge/` into the project root, then run `concept compile`. The starter intentionally contains schema files only, not sample concepts, so new projects do not get irrelevant search results.

Use `assets/templates/concept.md` as a copy source when creating a new concept file, then replace every placeholder before compiling.

## Core Workflow

1. Inspect `.knowledge/`, `.knowledge/_schema/`, and `.knowledge/.cache/` before editing.
2. Read the smallest relevant reference file only when needed:
   - `references/authoring.md` for source style, granularity, relations, and OKF alignment.
   - `references/cli.md` for command behavior and cache formats.
   - `references/yaml-structures.md` for flow, DFD, and UI sketch YAML conventions.
   - `references/multi-agent.md` for script-first offloading and lightweight-agent task boundaries.
3. Edit source concepts only under `.knowledge/`, excluding `_schema` unless changing catalog rules.
4. Preserve machine-first style: token-efficient English prose, tiny frontmatter, YAML fences for structured content, no Markdown tables.
5. Run compile after source or schema edits:
   ```bash
   python3 <skill-dir>/scripts/concept.py compile --project .
   ```
6. Use search/show/export to answer or prepare human-facing artifacts from compiled cache:
   ```bash
   python3 <skill-dir>/scripts/concept.py search --project . --type api access
   python3 <skill-dir>/scripts/concept.py show --project . api:access-check --related --reverse
   python3 <skill-dir>/scripts/concept.py export --project . api:access-check --profile review --output review.md
   ```

## Offloading Policy

Prefer this order for every task:

1. deterministic script,
2. lightweight read-only or single-file agent,
3. main reasoning agent.

Use `concept compile`, `concept search`, `concept show`, or `concept export` before model reasoning when those commands solve the task. Keep the main reasoning agent responsible for concept boundaries across many files, contradiction resolution, schema/type design, multi-file semantic changes, and final approval.

When installing this skill into a project at `.agents/skills/knowledge-compiler`, optional repository support files can be installed from `assets/repository-support/`:

```bash
python3 .agents/skills/knowledge-compiler/scripts/install_agent_support.py --project .
```

## Source Rules

- Keep one primary idea per concept; split when a file starts covering independent APIs, flows, data models, policies, permissions, decisions, or requirements.
- Use token-efficient English in `.knowledge` source content. Prefer stable technical terms, short sentences, and compact YAML over localized prose. Generate Japanese, localized, or human-readable documents through `concept export`.
- Frontmatter must contain only:
  ```yaml
  id: api:access-check
  type: api
  title: Access Check API
  ```
- `id` must use `type:name`; `type` must be lowercase; the `id` prefix must equal `type`.
- Do not write `role`, `category`, `format`, `editor`, or relations in frontmatter. Resolve them from `.knowledge/_schema/` and derived body references.
- Write references naturally as `type:name` in prose or YAML. `concept compile` derives relations by scanning body text.
- Do not use Markdown tables, decorative Markdown, or long narrative sections in source concepts.
- Use YAML code fences for fields, endpoints, rules, states, permissions, examples, flow steps, DFD-like structures, and UI sketches.
- Do not seed new projects with example concepts. Keep examples in references or templates only, never in `assets/starter/.knowledge/`.

## Command Rules

The public command surface is intentionally small:

- `concept compile`: parse source concepts, validate schema/frontmatter, derive references, and emit JSONL cache files.
- `concept search`: search compiled cache without reparsing source unless explicitly auto-compiled.
- `concept show`: show one concept plus semantically useful nearby concepts; avoid blind graph-depth expansion.
- `concept export`: generate human-readable Markdown/JSON/YAML review or ticket artifacts. Generated review Markdown may use tables, Mermaid, checklists, and Japanese if requested; never write it back as `.knowledge` source.

Do not add an extract command. Convert OpenAPI, DB schema, Mermaid, DFD, C4, or UI artifacts into `.knowledge` concepts conversationally and only after user review.

## Schema Policy

Load configurable metadata from `.knowledge/_schema/`. Do not hard-code concept types, categories, roles, enum values, or validation rules in project edits. The starter schema includes 16 default types, but projects may customize them later by editing `types.yaml`.

Default types:

`vision`, `requirement`, `metric`, `term`, `concept`, `actor`, `flow`, `rule`, `ui`, `api`, `event`, `data`, `permission`, `system`, `policy`, `decision`.

Default internal categories:

`objective`, `vocabulary`, `concept`, `behavior`, `interface`, `structure`, `constraint`, `external`.
