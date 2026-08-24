---
id: requirement:alias-transparent-type-analysis
type: requirement
title: Alias-Transparent Type Analysis
---
Generator analysis resolves Go type aliases before every named-type test, so an alias binds exactly like the type it names.

```yaml
priority: must
problem: >
  gotypesalias is the go/types default from Go 1.24 and mandatory from Go 1.27,
  so an alias reaches analysis as types.Alias and misses every *types.Named
  assertion
scope:
  packages: [configbind, httpbind, jsonbind, cborbind, sqlbind]
  sites:
    - config struct field type classification
    - time.Duration detection in concept:config-struct-mapping
    - struct slice element type
    - Bind type argument and wrapper role types
    - method receiver identity under rule:go-types-symbol-identity
    - same-package field-kind resolution (generator/plan.go resolveNamedKind), see same_package_field_kind_resolution below
rule: unalias before asserting the named type, at every such site, not one
failure_modes_today:
  loud: an alias of a named struct field type reports unsupported field type
  silent: >
    an alias of time.Duration skips duration detection, falls through to
    underlying int64, binds as an integer, and then rejects "5s"
  a_third_found_2026_08_24: same_package_field_kind_resolution below; unlike
    the other two, its message was not just unhelpful but self-referential --
    "type Mark is Mark underneath" -- because the type assertion failing left
    nothing recorded to quote
non_goals:
  - resolving a defined type such as 'type D time.Duration' to its source type; only aliases are transparent
  - relaxing the package boundaries of rule:same-package-convention
acceptance:
  - 'type PublicConfig = other.PublicAssetConfig used as a struct field generates the nested table'
  - 'type Timeout = time.Duration binds as a duration and accepts "5s" per rule:duration-value-parsing'
  - 'configbind.Bind[AliasOfConfig]("p") resolves the aliased config type'
  - an alias and the original spelling of one type generate byte-identical code
  - a defined type over a struct keeps its current behavior
  - 'a field aliasing a predeclared scalar (type Mark = uint) is admitted and generates the same code a bare uint field would'
  - 'a field aliasing a same-package defined type (type M = Mark) is admitted and converts under Mark''s own declared name'
  - 'a field aliasing a same-package struct (type M = LocalStruct) is admitted and calls decodeLocalStructJSON, not a function named for the alias'
  - 'a field aliasing a type declared in another package is refused, naming the field, the alias and the foreign package -- not planned as a nested struct nothing in this package''s run will generate'
  - a slice and a fixed-length array of an aliased scalar follow the same rules as the scalar itself

same_package_field_kind_resolution:
  status: implemented 2026-08-24
  found_by: maintainer, reporting type Mark = uint refused even as a plain scalar field, with the self-referential message above
  the_site: generator/plan.go resolveNamedKind, which classifies every same-package identifier a struct field or collection element names, for the one analysis pass jsonbind, httpbind, cborbind and sqlbind all read their FieldPlan from
  one_fix_four_downstream_generators: this site is shared, not per-format -- fixing it here fixed the alias case for the JSON codec, the CBOR codec, the HTTP binder and SQL row-filling at once, none of which has its own copy of this logic
  the_defect: resolveNamedKind asserted t.(*types.Named) directly; gotypesalias=1 (default since Go 1.24, mandatory in 1.27) makes an alias identifier arrive as *types.Alias, which fails that assertion, so no kind was ever recorded for it and the field was refused
  why_a_named_type_over_byte_was_the_tell: 'a *types.Named reached the old assertion fine, so type Mark uint worked while type Mark = uint did not -- the asymmetry the maintainer noticed and reported, the same shape rule:named-type-field-kind''s own collections_are_generated defect had two days earlier'
  four_shapes_worked_through:
    predeclared_scalar:
      example: 'type Mark = uint'
      after_unalias: a bare *types.Basic, no *types.Named at all
      resolution: 'the scalar kind, with NO declared name carried -- the field reads and writes as the predeclared type with no conversion, which is what makes it byte-identical to writing uint directly, per the acceptance line above'
      why_this_needed_a_new_signal: the old NamedKind had no way to say "carries a kind but no name"; Kind alone could not distinguish it from a plain defined type, which does need its name carried for the conversion Go's type system requires
    defined_type:
      example: 'type M = Mark, with type Mark uint'
      after_unalias: 'types.Unalias follows the WHOLE alias chain, not one layer, so this lands on Mark''s own *types.Named directly, identically to what a field written Mark would resolve to'
      resolution: 'the scalar kind, with Mark''s own declared name -- not M, the identifier the field was actually written with'
      why_not_the_field_s_own_identifier: 'M is not a real declaration generated code could reason about on its own terms; using Mark''s name is what makes this byte-identical to declaring the field Mark directly, the stronger reading of the acceptance line, and it was free once the struct case below needed the same lookup'
      compiles_either_way: 'named as M(v) it would have compiled too, since M and Mark are the same type by definition -- this was not a correctness bug like the struct case below, only a byte-identical one'
    local_struct:
      example: 'type M = LocalStruct'
      after_unalias: LocalStruct's own *types.Named, same package
      the_danger: fieldTypeKind used to return the field's own written identifier (M) as the struct name unconditionally, which is safe for a direct field (the identifier names the real declaration) but not here -- planning a struct named M, which nothing declares, emits a call to decodeMJSON inside a file headed DO NOT EDIT, compiling nothing
      resolution: LocalStruct's own declared name, exactly as the defined-type case above -- one mechanism serves both, since both are "the type this alias actually names, not the identifier it was reached through"
      same_lesson_as: rule:generated-source-not-discovered and rule:named-type-field-kind the_defect, each a diagnostic or a generated call naming something the generator itself never defined
    foreign_type:
      example: 'type M = other.Config, or type M = other.Meters even when Meters is itself a plain scalar'
      after_unalias: a *types.Named whose Obj().Pkg() does not match the package being analyzed
      resolution: refused, naming the field, the alias and the foreign import path -- not planned as a nested struct, and not silently reinterpreted down to a bare scalar either
      why_not_transparent_even_for_a_foreign_scalar: 'a directly-qualified foreign type (other.Meters spelled out) is only ever admitted through the foreign-codec path (requirement:json-codec-interface) or silently dropped when it carries no codec -- inventing a THIRD behavior, transparently reading through to the foreign type''s predeclared kind, for the alias spelling alone would make an alias behave unlike its own original spelling, the opposite of what this requirement asks for'
      not_wired_to_the_foreign_codec_path: 'the foreign-codec detector (codecCapableTypeNames) matches package-qualified selector expressions (other.Config) syntactically; an alias is a bare identifier (M) at every use site, so routing it into that detector would need its own plumbing -- left as the refusal, which is what case 4 already had before this fix, now with a clear reason instead of the self-referential one'
  mechanism:
    unalias_once: 'types.Unalias(t) at the top resolves the entire alias chain in one call, per go/types semantics -- there is no need to loop'
    the_new_fields_on_NamedKind: 'Declared (the name generated code should spell, "" for a transparent predeclared alias) and ForeignPackage (set only when the resolved *types.Named is declared somewhere else, which refuses rather than plans)'
    same_package_check: 'named.Obj().Pkg().Path() compared against the package being analyzed, threaded down through namedFieldKinds and collectNamedKinds from plan.PackagePath -- the same value discoverGenericTypeArgs already receives for an unrelated same-package check'
    the_diagnostic_hole_from_2026_08_24_earlier_that_day: 'filling Underlying from the type itself (rather than leaving it blank) is what made the alias case legible enough to report in the first place; it did not cause the bug, and this fix does not touch it beyond adding the ForeignPackage branch ahead of it'
  verification:
    generator/alias_field_test.go: 'all four shapes, plus a slice and a fixed-length array of the transparent case; one test asserts byte-identical output against the un-aliased spelling, one builds and runs a compiled fixture over both JSON and CBOR'
    existing_tests_unaffected: 'TestUnmappableTypeDiagnosticHasNoHole and TestNamedTypeWithAnUnsupportedUnderlyingIsRefused, from the same-day diagnostic-hole fix, pass unchanged -- traced by hand and confirmed by the suite, since both exercise the non-alias path this change was careful to leave identical'
    compatibility: 'verified 2026-08-24 by regenerating both examples; every generated body is byte for byte what it was, since every new branch is reached only by a shape that used to be refused'
  not_extended_to_configbind: 'configbind already unaliases at each of its own sites (unaliasPtr, types.Unalias, used directly in generator/configbind.go), predating this fix and untouched by it -- this entry is scoped to the one shared site that had never been updated'
related:
  - decision:configbind-supported-types
  - requirement:duration-config-fields
  - requirement:framework-wrapper-discovery
  - rule:duration-value-parsing
  - rule:go-types-symbol-identity
  - rule:same-package-convention
  - concept:config-struct-mapping
  - flow:configbind-codegen
  - flow:code-generation
  - system:configbind
  - rule:named-type-field-kind
  - requirement:sized-integer-field-kinds
  - requirement:json-codec-interface
  - requirement:cbor-codec-generation
```
