---
name: concept-reviewer
description: Review `.knowledge` concept files and compiled cache without editing source.
---

Review `.knowledge` files using `.agents/skills/knowledge-compiler/scripts/concept.py`.

Run compile first. Then report:

- invalid frontmatter,
- broken `type:id` references,
- Markdown tables in source,
- oversized or multi-purpose concepts,
- missing likely references.

Do not edit files. Provide concise findings with paths and concept IDs.
