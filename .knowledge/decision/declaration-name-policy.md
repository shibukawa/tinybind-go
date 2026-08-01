---
id: decision:declaration-name-policy
type: decision
title: Per-Format Declaration Name Policy
---
Let each format state what it needs from a declaration name, because the three formats need three different things and only one of them needs PascalCase.

```yaml
status: implemented 2026-08-02
source:
  - user design decision 2026-08-02
  - decision:template-declaration-kinds
  - decision:template-parser-delegation
problem:
  was: the shared parser required PascalCase of every declaration name, for every format
  cost: it read as one language rule but was really one format's constraint generalized; it is also what kept .tb.dynamo out of the shared parser, per decision:dynamo-template-shared-parser
  observation: a name constraint is never about taste here; each one exists because some downstream spelling is derived from the name, and what is derived differs per format
why_each_format_differs:
  html:
    needs: PascalCase, always, exported or not
    reason: in markup an element whose tag name starts uppercase is the component-call syntax; a lowercase component could never be called and would collide with a standard element
    private_form: the generated Go identifier is prefixed, so visibility never depends on the template name's own case
  sql:
    needs: the name's case to agree with the export modifier, for an executable statement
    reason: the generated execution function is named exactly as the declaration is, in both directions, per requirement:private-statement-go-api
    exception: a sql.predicate or sql.relation reaches no Go identifier of its own, because it is embedded into a caller's builder through the prefixed fragment builder; its case is unconstrained
    where_checked: in the compiler rather than the parser, because the constraint depends on the cardinality and that is where the cardinality is resolved
  dynamo:
    needs: the name's case to agree with the export modifier
    reason: the generated function takes the declaration name verbatim, so an exported declaration with a lowercase name would emit an unexported Go function, and Go's own rule is the one that has to hold
policy:
  shape: three independent flags on the root declaration registration, each stating a fact about what the format's emitter does with the name
  why_facts_not_modes: a mode would be a label to keep in sync by hand; a fact about the emitted identifier is checkable against the emitter, and the constraint follows from it
  flags:
    pascalCase:
      states: the name is the call syntax, so it must start uppercase whether or not it is exported
      set_by: [html]
    exportedNameIsGo:
      states: the public generated identifier is the name itself, so an exported declaration needs an exported name
      set_by: [html, sql, dynamo]
    privateNameIsGo:
      states: the private generated identifier is also the name itself, so an unexported declaration needs an unexported name
      set_by: [sql, dynamo]
      not_set_by:
        html: its private component is emitted as render<Name>
  zero_value: constrains nothing, which is what a format registers when it composes every identifier it emits
registrations:
  html: pascalCase and exportedNameIsGo, on the parser registration
  sql: both name flags in effect, applied by the compiler because the cardinality decides whether they apply at all
  dynamo: exportedNameIsGo and privateNameIsGo, which is the rule its own parser already enforced
migration:
  html: unchanged
  dynamo: unchanged; its parser already enforced this rule
  sql: a private executable statement must now be named in lowerCamelCase; a private sql.predicate or sql.relation is unaffected
unchanged:
  - types, enums, enum members, and external functions keep the rule:template-name-casing PascalCase form; this decision covers root declaration names only
  - the export modifier keeps its meaning in every format: it selects the public generated API
  - a lowercase name never becomes a public Go identifier in any format
consequence:
  dynamo_migration: the difference that decision:dynamo-template-shared-parser recorded as a blocker becomes a registration flag, so moving .tb.dynamo onto the shared parser is no longer a language change
related:
  - decision:template-declaration-kinds
  - decision:template-parser-delegation
  - decision:dynamo-template-shared-parser
  - rule:template-name-casing
  - requirement:template-file-scope
```
