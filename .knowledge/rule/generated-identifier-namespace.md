---
id: rule:generated-identifier-namespace
type: rule
title: Generated Identifier Namespace
---
Every identifier a generator introduces into emitted Go begins with an underscore, and a template declaration may not, so a parameter can never collide with a variable the emitter owns.

```yaml
priority: must
problem:
  found: 2026-08-17 while verifying requirement:sql-conditional-predicate-composition
  symptom: 'a template parameter named b makes the generated builder unbuildable: "b redeclared in this block"'
  cause: goLocalName escapes Go keywords only, so every other emitter-owned name is unprotected
  surface: the emitter owns b, err, statement, rows, result, value, executor, and yield as locals, plus ctx and db as parameters of the public signature
  severity:
    hard_error: b, ctx, and db collide outright
    silent_risk: err, statement, rows, result, value, and executor shadow, so a body can compile against the wrong variable
  why_it_was_not_caught: no fixture or test declares a parameter with one of these names, and the collision is invisible in the template
status: implemented 2026-08-17 in templates/sqlbind
rules:
  - an emitter-introduced local is spelled with a leading underscore
  - a template parameter, value binding, or any other author-chosen name reaching emitted Go may not begin with an underscore
  - ctx and db stay unprefixed, and are refused as author-chosen names instead
renamed: _b, _err, _statement, _rows, _result, _value, _executor, and _yield
underscore_prohibition_was_already_enforced:
  found: rule:template-name-casing already refuses a leading underscore, because it is not lowerCamelCase
  consequence: the namespace was reserved before this rule asked for it, so the new check is a second gate stating the reason rather than a new restriction
  kept_anyway: it names the emitter as the owner of the namespace, which the casing diagnostic does not, and it does not depend on the casing rule staying as it is
public_signature_exception:
  which: ctx and db, the leading parameters of a generated executor API
  why_not_prefixed: they are the documented public surface, so _ctx and _db would put the emitter's bookkeeping in every godoc signature; requirement:sql-generated-api-layers owns how that surface reads
  consequence: two reserved words rather than two renamed ones, refused with a diagnostic that names them
  precedent: the emitter already owns the _tinybind prefix for generated functions, so reserving the underscore namespace widens a line the module already draws
non_goals:
  - renaming an author's parameter to avoid a collision, which would make the generated signature disagree with the template
  - renaming ctx or db only when a template happens to collide, which makes two statements in one package read differently
scope:
  done: templates/sqlbind
  not_checked: htmlbind, dynamobind, firestorebind, configbind, and the server-action emitter each write their own locals beside author-chosen names and were not audited; the same class of collision is likely present
  how_to_find_it: an emitted string literal declaring an unprefixed local, which is what the sqlbind audit enumerated
test_expectations_this_moved:
  which: check_test, generate_test, binding_test, and namecase_test in templates/sqlbind, plus generator/sqlerror_test
  why: they assert on generated Go text, so a renamed local is a real expectation change rather than a weakened test
  note: the sibling emitters' tests were untouched and still pass, which is the evidence that the rename stayed inside sqlbind
acceptance:
  - 'a parameter named b, err, statement, rows, result, value, executor, or yield generates and runs'
  - 'a parameter named ctx or db is a generation diagnostic naming it reserved'
  - 'a parameter named _x is a generation diagnostic'
  - 'a value binding named _x is a generation diagnostic'
  - no generated file declares an unprefixed emitter-owned local
related:
  - requirement:analysis-diagnostics
  - requirement:sql-generated-api-layers
  - concept:code-generation
  - rule:template-name-casing
  - requirement:sql-template-v1
```
