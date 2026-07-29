---
id: rule:sql-placeholder-emission
type: rule
title: Generated SQL Placeholder Emission
---
Template value expressions create bound arguments; template authors never manage database placeholders.

```yaml
source: concept:typed-template-language
template_syntax: 'column = {value}'
output: data:sql-statement
rules:
  - only a parsed template value expression may create a bind placeholder
  - raw SQL input never creates or preserves a bind-placeholder node or token
  - append evaluated value to Args and emit its placeholder as one operation
  - preserve encounter order after runtime structural conditions are resolved
  - reject manual bind-parameter tokens outside SQL literals and comments
  - never interpolate an ordinary value into SQL text
  - expand requirement:sql-relation-composition AST calls before numbering placeholders
parser:
  expression: lower to one generated placeholder plus one Args append
  sql_text: reject dialect-recognized bind-placeholder tokens
  literals_and_comments: placeholder-like characters remain inert source text
initial_styles:
  dollar_numbered: '$1, $2, ...'
  question: '?, ?, ...'
configuration:
  phase: code generation
  relation: derived from data:sql-dialect, not selected independently
  selection: requirement:sql-dialect-selection; explicit, with no implicit default
dynamic_sql: generated runtime appender owns numbering when optional clauses change argument count
```
