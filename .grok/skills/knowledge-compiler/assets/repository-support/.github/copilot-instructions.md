# Knowledge Compiler Instructions

When working with `.knowledge` catalogs, prefer scripts before model reasoning.

Use:

```bash
python3 .agents/skills/knowledge-compiler/scripts/concept.py compile --project .
python3 .agents/skills/knowledge-compiler/scripts/concept.py search --project . <query>
python3 .agents/skills/knowledge-compiler/scripts/concept.py show --project . <id> --related
python3 .agents/skills/knowledge-compiler/scripts/concept.py export --project . <id> --profile review
```

Source rules:

- Keep one concept per file.
- Keep frontmatter to `id`, `type`, `title`.
- Use token-efficient English source text.
- Use YAML fences for structured content.
- Do not use Markdown tables in `.knowledge` source.
- Treat `.knowledge/.cache/` as generated output.

Use the main agent for schema changes, multi-file semantic changes, contradiction resolution, and final approval.
