---
id: rule:wrapper-allow-query-semicolons
type: rule
title: AllowQuerySemicolons Unwrap
---
Unwrap http.AllowQuerySemicolons and analyze the inner handler; nothing is recorded.

```yaml
wrapper: http.AllowQuerySemicolons(h)
example: |
  mux.Handle(
      "GET /search",
      http.AllowQuerySemicolons(
          http.HandlerFunc(searchHandler),
      ),
  )
parse:
  - unwrap h
  - analyze inner handler
openapi:
  schema_change: none
  metadata: none; an allow_query_semicolons field was recorded through 2026-08-14 but nothing ever consumed it, so it was removed
related:
  - concept:stdlib-wrapper-unwrap
  - concept:openapi-generation
```
