# Knowledge Compiler Delegation

Use this file when the runtime does not have native subagent configuration.

Prefer deterministic commands:

```bash
python3 .agents/skills/knowledge-compiler/scripts/concept.py compile --project .
python3 .agents/skills/knowledge-compiler/scripts/concept.py search --project . <query>
python3 .agents/skills/knowledge-compiler/scripts/concept.py show --project . <id> --related
python3 .agents/skills/knowledge-compiler/scripts/concept.py export --project . <id> --profile review
```

Delegate only narrow work:

- read-only search,
- one-concept review,
- one-concept edit,
- first draft of one small concept.

Keep main-agent ownership of:

- schema changes,
- concept splitting across files,
- resolving contradictions,
- accepting generated concepts,
- final review.
