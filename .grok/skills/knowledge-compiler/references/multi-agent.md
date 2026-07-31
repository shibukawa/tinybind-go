# Multi-Agent Offloading

## Principle

Keep expensive reasoning focused on high-level judgment. Use deterministic scripts first, then lightweight agents for narrow read-only or single-file work.

Order of preference:

1. Python scripts,
2. lightweight model or subagent,
3. main reasoning model.

## Good Offload Targets

- Search `.knowledge` without editing.
- Read one concept and summarize it.
- Check one concept against authoring rules.
- Edit one concept file.
- Draft one small concept.
- Convert one user note into one `.knowledge` concept.
- Check broken `type:id` references.
- Generate review Markdown from compiled cache.
- Compare two concept files.
- Propose missing references.
- Rewrite Japanese discussion into token-efficient English `.knowledge`.

## Do Not Offload

- Global architecture decisions.
- Concept boundary decisions across many files.
- Contradiction resolution.
- Multi-file semantic changes.
- Accepting or rejecting generated concepts.
- Schema/type design.
- Final review before commit.

## Suggested Agent Roles

`concept-searcher`: read-only, lightweight. Search compiled cache, inspect related IDs, list candidate concepts.

`concept-editor`: single-file edit, lightweight. Edit exactly one concept, preserve machine-first style, avoid Markdown tables, keep token-efficient English.

`concept-reviewer`: read-only, lightweight or medium. Check authoring rules, oversized concepts, broken or missing `type:id` references.

`concept-architect`: strong model or main agent. Split concepts, resolve ambiguity, design schema changes, approve multi-file changes.

## Repository Support Files

Install optional support files from `assets/repository-support/`:

```bash
python3 .agents/skills/knowledge-compiler/scripts/install_agent_support.py --project .
```

The support files include:

- `.claude/agents/concept-searcher.md`
- `.claude/agents/concept-editor.md`
- `.claude/agents/concept-reviewer.md`
- `.github/copilot-instructions.md`
- `.github/agents/concept-reviewer.md`
- `.agents/knowledge-compiler/delegation.md`
- `.agents/knowledge-compiler/prompts/*.md`

Treat these as project-local templates. Adjust model names and tool restrictions to the target agent runtime.
