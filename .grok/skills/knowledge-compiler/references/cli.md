# CLI Reference

Run from any project with a `.knowledge` directory:

```bash
python3 <skill-dir>/scripts/concept.py <command> --project .
```

## compile

```bash
python3 <skill-dir>/scripts/concept.py compile --project .
```

Behavior:

- Load `.knowledge/_schema/types.yaml`, `categories.yaml`, and `enums.yaml`.
- Parse `.knowledge/**/*.md`, excluding `_schema` and `.cache`.
- Validate required frontmatter keys: `id`, `type`, `title`.
- Reject extra frontmatter keys.
- Validate `id` format and `id` prefix.
- Validate type against loaded schema.
- Extract known `type:name` references from body text.
- Report missing references and invalid source records.
- Emit cache files under `.knowledge/.cache/`.

Cache files:

- `concepts.jsonl`: normalized concept metadata with schema-derived fields.
- `relations.jsonl`: derived source-to-target references.
- `index.jsonl`: denormalized searchable records with body text.

Generated cache may be much larger than source. That is expected: source optimizes LLM context cost; cache optimizes tool execution.

## search

```bash
python3 <skill-dir>/scripts/concept.py search --project . access
python3 <skill-dir>/scripts/concept.py search --project . --type api --limit 5 access
```

Reads compiled cache. If cache is missing, run `compile` first or pass `--auto-compile`.

Useful filters:

- `--type TYPE`
- `--linked-to ID`
- `--depends-on ID`
- `--orphan`
- `--limit N`
- `--format brief|full|json`

## show

```bash
python3 <skill-dir>/scripts/concept.py show --project . api:access-check --related --reverse
```

Reads compiled cache and prints the target concept plus selected nearby concepts.

Prefer supporting concepts below the target, such as `data`, `policy`, `permission`, `rule`, `event`, and `decision`. Avoid blind graph-depth expansion; use `--max-related` to keep output bounded.

Useful options:

- `--related`
- `--reverse`
- `--max-related N`
- `--format human|ai|json`

## export

```bash
python3 <skill-dir>/scripts/concept.py export --project . requirement:access-check --profile review --output review.md
```

Generates a human-facing artifact from compiled concepts. Markdown export may use tables, diagrams, and checklists because it is generated output, not source.

Useful options:

- `--profile review|ticket|ai`
- `--scope N`
- `--include TYPE`
- `--exclude TYPE`
- `--max-concepts N`
- `--format markdown|json|yaml`
- `--output FILE`
- `--compact`
