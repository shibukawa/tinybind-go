---
id: rule:template-context-safety
type: rule
title: Template Context Safety
---
Every dynamic contribution is typed by its structural output position; it never becomes unclassified output text.

```yaml
source: concept:typed-template-language
parser: decision:template-parser-delegation
model: structural lists contain items, conditionals, loops, and compatible component references
html_lists:
  - child nodes
  - static attribute values
  - script raw-text content
  - style raw-text content
sql_lists:
  - predicates
  - joins
  - assignments
  - order items
raw_text_gate: rule:raw-text-insertion-gate narrows which braces open an insertion inside script and style content, where the body is authored in another language
rules:
  - format parser attaches one namespaced context ID to every shared expression and control node
  - HTML strings use context-specific escaping
  - explicit raw output and embedded JSON follow requirement:explicit-output-control
  - trusted output types are distinct and cannot cross HTML, CSS, or JavaScript contexts
  - SQL values become bound parameters
  - SQL identifiers and result shapes remain static in v1
  - if and for are valid only where the active format accepts structural list items
```
