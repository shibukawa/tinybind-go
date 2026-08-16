---
id: requirement:message-scope-declaration
type: requirement
title: Message Scope Declaration
---
Let a template file name one namespace its message references resolve against, and let a dotted id leave it.

```yaml
source: concept:template-message-surface, request items A and C
review_gate: approved 2026-08-16 by the owner
as_built:
  status: implemented 2026-08-16
  parser: templates/internal/syntax/module.go, one case beside package and import; MessagesDecl on Module
  resolution:
    where: the compiler, not the parser
    why: resolution needs the whole module, and a `messages` line may follow the declaration that uses it; doing it in the parser would have made the header positional
    verified: a declaration written after the component that uses it resolves, covered by TestMessageResolution
  missing_declaration: reported at the reference, naming it, per the no_derivation rule below
  tests: templates/htmlbind/message_test.go
priority: must; decision:message-reference-syntax has nothing to resolve against without it
form:
  written: "messages about"
  placed: the shared header, beside imports and external declarations from requirement:template-file-scope
  name: a dotted identifier, so `about` and `checkout.payment` are both legal
  arity: one per file; a second is an error
semantics:
  import_like: it names the namespace this file's references resolve against, and nothing more
  opaque: what the namespace contains is known only through the symbols requirement:message-symbol-resolution is given
  no_derivation: a file carrying a reference with no declaration is an error; nothing is inferred from the file path or a component name
  reason_for_no_default: a derived scope makes moving a file change which messages it resolves, silently
resolution:
  bare: `{t title}` under `messages about` resolves as about.title
  dotted: `{t common.save}` resolves as written and ignores the file's scope
  precedent: the same rule as a package-qualified name in Go, so it needs nothing beyond permitting dots in the id decision:message-reference-syntax accepts
  ambiguity: none; a dot is the marker, so a bare name is never also a qualified one
declaration_site:
  decided: 2026-08-16 by the owner; the header form, not the annotation
  reason_it_fits: the scope is a property of the file rather than of one declaration, and requirement:template-file-scope makes several components per file ordinary
  consequence: the shared header grammar gains one form, so requirement:template-source-formatting prints it and requirement:template-parse-introspection reports it from the header rather than from an annotation list
  not_taken: the annotation below, which stays recorded because it was a real substitute rather than a degraded one
declaration_site_was_open:
  header_form: what the request asks for, and what was chosen
  annotation_fallback:
    written: "@i18n(scope: \"about\")" on the component declaration
    grammar_cost: none; decision:template-annotation-syntax already parses `@name(key: \"value\")` and hands each format an annotation list
    why_it_is_the_fallback: an annotation attaches to one declaration, so a file holding several components repeats it, and requirement:template-file-scope makes several per file ordinary
    consumer_note: decision:template-annotation-syntax names its consumers explicitly and would gain one
  reporter_position: nothing downstream depends on which lands
  our_reading: the header form is right if a header addition is cheap, because the scope is a property of the file rather than of a declaration; the annotation is a real substitute rather than a degraded one, and the choice can be made on implementation cost alone
  what_would_change_that: a future need to give two components in one file different scopes, which neither the request nor the reporter's catalog policy asks for
downstream_policy_not_ours:
  - the declared name is the catalog's sharding key
  - the declared name is the id prefix
  - both are stated here only so a reader knows why the name is not free-form
acceptance:
  - a file declaring a scope resolves a bare reference against it
  - a dotted reference resolves without consulting the declaration
  - a second declaration in one file is an error at the second one
  - a reference with no declaration anywhere in the file is an error naming the reference
  - a file declaring a scope and using no reference is legal and generates unchanged output
```
