# Knowledge Compiler Skill

This skill bundles a self-contained Python CLI for compiling and querying AI-first `.knowledge` catalogs.

## Starter Contents

`assets/starter/.knowledge/` contains schema files only. It intentionally does not include sample concepts, so new projects start with an empty catalog and search results are not polluted by unrelated example knowledge.

Use `assets/templates/concept.md` as a copy source when drafting a new concept.

## Vendored Python Packages

The packages under `vendor/python/` are bundled so the CLI can run without installing dependencies or using network access at runtime.

| Package | Version | Author | License | Bundled path |
| --- | --- | --- | --- | --- |
| `fastjsonschema` | 2.21.2 | Michal Horejsek | BSD-3-Clause | `vendor/python/fastjsonschema/` |
| `pyyaml-pure` | 0.1.0 | Ali Fadel | MIT | `vendor/python/yaml/` |

## License Files

Full license and attribution files are preserved under `vendor/licenses/`:

- `fastjsonschema-LICENSE`
- `fastjsonschema-AUTHORS`
- `pyyaml-pure-LICENSE`

## Runtime Policy

`scripts/concept.py` prepends `vendor/python/` to `sys.path` and imports:

- `yaml` from `pyyaml-pure`
- `fastjsonschema`

If either package is missing, the CLI exits with a clear dependency error.

## Optional Repository Agent Support

When this skill is installed in a project at `.agents/skills/knowledge-compiler`, install optional repository support files with:

```bash
python3 .agents/skills/knowledge-compiler/scripts/install_agent_support.py --project .
```

This copies project-local templates for Claude Code agents, GitHub Copilot instructions, and generic Codex-style delegation prompts from `assets/repository-support/`.
