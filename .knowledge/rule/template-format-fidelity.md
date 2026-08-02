---
id: rule:template-format-fidelity
type: rule
title: Template Formatting Fidelity
---
A formatter may change bytes a reader sees and must not change bytes a program produces; where the two cannot be separated, the formatter leaves the region alone.

```yaml
status: implemented 2026-08-02
applies_to: requirement:template-source-formatting
invariants:
  generation_equality:
    rule: generating from a formatted source produces output byte-identical to generating from the original
    test: format every fixture, regenerate, and diff the artifacts; this is the only check that covers all three formats at once
    note: rule:generation-input-hash hashes the source, so a formatting run does force one regeneration; the artifact it writes is what must be unchanged
  idempotence:
    rule: formatting formatted output changes nothing
    reason: without it the tool cannot be run in CI, because a diff never settles
    enforced_at_runtime: api:template-formatter-library formats twice and refuses to return a result that differs, so a formatter bug reaches a caller as an error rather than as a file corrupted a little more on every save
    reason_for_the_guard: the failure it catches is unbounded, and the check costs one more pass over a small file
  parse_stability: reparsing formatted output yields the same AST as the original, comments included
  no_semantic_edits: the formatter never adds, removes, or reorders a declaration, attribute, field, parameter, or clause
  sql_token_identity:
    rule: rescanning the formatted SQL body yields the same token sequence, with literals, quoted identifiers, and comments compared as whole opaque tokens
    why_this_form: it is what permits layout at all; "byte for byte" would forbid indenting a clause, while token identity forbids only the edits that change meaning
    consequence: whitespace between tokens is the formatter's to choose, and whitespace inside a token is not
escape_round_trip:
  problem: the parsers decode as they go, so the AST is not the source
  html_braces:
    parse: '{{x}} in a component body becomes the text {x}'
    print: a literal brace run in ordinary template text must be re-emitted escaped, and a text node that already contains }} cannot be wrapped naively
    raw_text:
      rule: in a script or style body a brace is written back as it stands, because the parser already keeps it as text
      why: escaping it rewrites the authored CSS or JavaScript, and an escape the next read has to undo is where a formatter stops settling
      exception: a brace the parser would read as an insertion, per rule:raw-text-insertion-gate, is the one brace whose bare spelling the source cannot hold, so it keeps the escape
      test: the printer asks the parser's own insertion test rather than a second guess at it
    not_raw_text:
      which: pre, textarea, and a preserve-whitespace subtree
      rule: whitespace is preserved there, but the text is still template text, so its braces are escaped like any other
      why_it_matters: treating them as raw text emits a bare brace the parser then reads as syntax, which is a source that no longer parses
  attributes: a value is a part list of text and expressions; printing normalizes the surrounding quote to double quotes and must escape any quote the value itself contains
  sql: the SQL body keeps literals, quoted identifiers, comments, and dollar-quoted strings as raw text, so no decoding happened and none is undone
whitespace_boundaries:
  html_significant:
    untouched: every position rule:whitespace-preserving-contexts preserves verbatim, byte for byte
    reason: the compiler already refuses to rewrite them, and a formatter has strictly less information than the compiler
  html_droppable:
    allowed: adding and removing breaks freely in the positions rule:whitespace-preserving-contexts calls droppable, because the HTML parser discards a whitespace-only run there
    covers: head, table, thead, tbody, tfoot, tr, colgroup, and around the html element
    consequence: one tag per line inside head is provably invisible, not a judgment call
  html_run_reshaping:
    allowed: replacing an existing whitespace run between two nodes with a newline plus indentation
    why_neutral: decision:whitespace-collapse-policy collapses any run to one U+0020, so a break and a space emit the same byte
    forbidden_create: no run may be introduced where none exists; adjacent glued nodes stay on one line even past the line width
    forbidden_delete: a run is reshaped, never removed, so the collapsed space always survives
  preserve_option:
    condition: PreserveTemplateWhitespace in data:generator-options turns collapse off and keeps authoring whitespace byte for byte
    consequence: reshaping is neutral only because collapse follows it, so under that option reshaping is disabled and only the droppable positions are laid out
    surface: api:template-format-command carries the same flag, because the formatter cannot read the generator's configuration
  sql_regions:
    allowed: any whitespace between two scanned tokens, which is what makes clause, join, and subquery layout possible
    forbidden: any edit inside a literal, quoted identifier, or comment, and any break that would move content onto a line a `--` comment already ends
failure_mode:
  on_parse_error: leave the file byte for byte and report the diagnostic; a partial write is worse than no formatting
  on_unprintable_node: fail the run rather than emit a guess, so a node type added to a parser cannot silently lose content
related:
  - requirement:template-source-formatting
  - decision:template-formatter-architecture
  - rule:whitespace-preserving-contexts
  - decision:whitespace-collapse-policy
  - rule:sql-template-layout
  - rule:html-template-layout
  - rule:sql-top-level-keyword-scan
  - api:template-format-command
  - requirement:template-comment-retention
  - data:generator-options
  - rule:generation-input-hash
```
