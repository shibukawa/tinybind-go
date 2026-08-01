---
id: rule:template-name-casing
type: rule
title: Template Name and Keyword Casing
---
Case and word form identify language, user, HTML, and SQL symbol classes and are validated at compile time.

```yaml
source:
  - concept:typed-template-language
  - user design discussion 2026-07-20
classes:
  sql_keywords:
    form: UPPERCASE
    examples: [SELECT, FROM, LEFT JOIN, WHERE, IS NULL, TRUE, FALSE, NULL]
  dsl_keywords:
    form: lowercase
    examples: [export, component, statement, if, else, for, where, subquery, predicates]
  user_symbols:
    form: PascalCase
    includes: [types, enums, enum members, components, statements, external functions]
    examples: [UserRow, UserStatus, Active, UserCard, FindUser, NormalizeName]
  dsl_values:
    form: lowerCamelCase
    includes: [parameters, fields, local and loop variables]
    examples: [tenantID, minimumAge, displayName]
  sql_identifiers:
    form: lower_snake_case
    includes: [schemas, tables, columns, aliases]
    examples: [public, user_accounts, created_at, active_users]
  html_builtin_names:
    form: lowercase or kebab-case
    includes: [elements, attributes]
    examples: [div, aria-label, data-user-id]
  framework_builtin_elements:
    form: kebab-case, whitelisted at generate time per proposed decision:builtin-element-syntax
    includes: [requirement:builtin-element-registration elements]
    examples: [csrf-token]
    note: the hyphenated space is closed; an undeclared hyphenated name is a generation error, and a Web Component is declared as a passthrough entry
  sql_builtin_names:
    form: lowercase unless classified as a dialect keyword
    includes: [functions, type names]
    examples: [count, coalesce, lower, integer]
  builtin_output_types:
    form: lowercase
    examples: [html, sql.exec, sql.many, sql.relation]
diagnostics:
  - recognize a SQL keyword written with wrong case and report required uppercase spelling
  - do not reinterpret wrong-case SQL keywords as identifiers
  - do not silently normalize user-defined symbols or format identifiers
  - user symbol resolution is case-sensitive and requires exact spelling
postgresql_v1:
  identifiers: lowercase unquoted only
  quoted_mixed_case_identifiers: deferred
```
