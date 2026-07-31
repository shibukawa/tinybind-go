# Authoring Reference

## Purpose

`.knowledge` is an AI-first source format. Humans normally interact through an agent and generated exports, not by browsing source files directly.

Optimize source for:

- low token cost,
- token-efficient English,
- stable retrieval,
- incremental edits,
- deterministic compilation,
- LLM-readable structure.

## OKF Alignment

Keep the catalog compatible with the spirit of OKF:

- Markdown files with YAML frontmatter.
- Parseable UTF-8 text.
- Frontmatter includes a required `type`.
- Consumers tolerate future custom types and unknown schema values.
- Directory layout remains portable and git-diffable.

Intentional `.knowledge` constraints beyond OKF:

- Add required `id` because `type:name` references are cheaper and more stable than path links.
- Keep frontmatter to only `id`, `type`, and `title`.
- Move category, role, and enum metadata to `.knowledge/_schema/`.
- Infer relations from `type:name` body references instead of writing relation lists.
- Prefer YAML fences over Markdown tables in source concepts.

## Granularity

Prefer one concept per file. A concept should describe exactly one primary idea.

Good concept units:

- one API,
- one flow,
- one data model,
- one policy,
- one permission,
- one requirement,
- one decision,
- one domain term,
- one UI surface.

Split a concept when it starts defining multiple independently reusable concepts. Many small files are cheaper than one large source file because search/show/export can reconstruct wider context from cache.

## Source Style

Use token-efficient English for `.knowledge` source content.

Rationale:

- English is usually cheaper and more stable for LLM technical context.
- Technical identifiers, API names, data fields, and policy terms drift less in English.
- Localized prose belongs in generated exports, not source concepts.

Use short prose only to define meaning, intent, or constraints. Put structured details in YAML fences.

Avoid:

- Markdown tables,
- decorative Markdown,
- long explanatory prose,
- localized prose as source of truth,
- duplicated facts,
- generated review documents inside `.knowledge`.

Use stable references such as `api:access-check`, `data:access-decision`, and `policy:access-check` in prose or YAML values. The compiler derives relations from those references.

## Frontmatter

Only these keys are allowed:

```yaml
---
id: api:access-check
type: api
title: Access Check API
---
```

Rules:

- `id` uses `type:name`.
- `type` is lowercase.
- `id` prefix equals `type`.
- `title` is short and human-readable.
- Do not write `role`, `category`, `format`, `editor`, `tags`, `status`, or relations in frontmatter.

## Review Outputs

Human review artifacts are generated output. They may use:

- Markdown tables,
- Mermaid,
- checklists,
- localized Japanese,
- longer explanations.

Do not copy generated review output back into `.knowledge` source.
