---
id: decision:cborbind-runtime-package
type: decision
title: cborbind Runtime Package And Dependency Direction
---
Add a CBOR binding runtime as its own transport-neutral leaf that imports the driver's codec, dispatches statically, and registers nothing.

```yaml
status: implemented 2026-08-19
extends: decision:runtime-package-boundaries
as_built:
  runtime: cborbind/doc.go and cborbind/declare.go; six annotations, one zero-size Declaration type, and no interface at all
  driver_bump: go.mod moved from tinygodriver v1.2.3 to v1.2.5 the moment the package existed, as this decision said it would
  generator_files: generator/cborbind.go, cborbind_types.go, cborbind_schema.go, cborbind_emit.go, cborbind_decode.go, cborbind_generate.go, mirroring the DynamoDB set
  analysis_shape: a go/types collector like dynamobind's, not the AST fieldTypeKind path, because interface satisfaction, underlying kinds and slice elements all have to be read from types rather than from syntax
  registration: none emitted, and no init function; the generated file declares two package-level profile vars and nothing else
  wired_into_the_run: generator/execution.go, after cachekeybind, writing cborbind_gen.go
  verified: full generator suite green 2026-08-19, and the generated codec builds and runs under TinyGo 0.41.1 for wasm and wasip1
the_combined_cli_run_refused_a_sized_integer:
  found: 2026-08-19, generating the TinyGo smoke fixture
  what: tinybind-gen generate runs the JSON mapping pass first, and AnalyzePackage walks every package-level struct whether or not anything uses it; fieldTypeKind maps string, int, int64, bool and float64, so a uint32 field failed the whole run before the CBOR pass was reached
  not_caused_by_this_mode: a package holding a uint32 field already failed that way with no CBOR anywhere, verified against a bare probe package
  why_it_bit_here_hardest: a sized integer per field is what the wire profile is for, so the mode most in need of the CLI was the one that could not reach it
  the_two_modes_never_collided_in_output: Emit is entirely usage-gated and emits nothing for a zero-usage type, and the CBOR pass writes its own file from its own analysis; only the JSON walk's refusal was in the way
  fixed: 2026-08-19, in generator/plan.go checkUnmappable
  how: the walk holds the refusal instead of raising it, and reports it once usage is assigned -- for a struct a configured call named, for one a used struct holds as a field, or under generate-all, which is asking for everything
  which_half_was_chosen: the mapping pass stopped failing on a struct nothing uses; fieldTypeKind still maps what it always mapped, so nothing changed about what a JSON codec does with a sized integer
  diagnostic_unchanged: the same message, at the same place, for every case that still needs it; verified by test over a call site, a nested field and generate-all
  the_other_half_is_still_open: teaching fieldTypeKind the sized integers would let those structs have a JSON codec too, which is a decision about the JSON wire format rather than about CBOR
source:
  - downstream game framework CBOR requirements 2026-08-19, its Phase 0
  - the framework decides to extend this generator rather than write its own codegen, and shares that base with the other downstream framework
package:
  name: cborbind, matching jsonbind, sqlbind, dynamobind, firestorebind and htmlbind
  path: github.com/shibukawa/tinybind-go/cborbind
owns:
  - the generation declarations of requirement:declared-cbor-codec
  - nothing else; the codec interfaces are the driver's, per rule:cbor-codec-interface-upstream
imports:
  - github.com/shibukawa/tinygodriver/encoding/cbor
driver_version:
  minimum: v1.2.5, the release that introduced encoding/cbor
  effect: the module's tinygodriver requirement moves from v1.2.3 to v1.2.5 the moment cborbind exists
excludes:
  - net/http
  - database/sql
  - any fixed-point math package; decision:cbor-scale-lives-in-the-type leaves the conversion in the author's own type, so cborbind names none
dependency_direction:
  - user package -> cborbind -> system:tinygodriver-cbor
  - user package -> tinygodriver/encoding/cbor, because generated code names driver types
  - the same edge dynamobind and fasthttpbind already have, so this is the third instance of a settled shape rather than a new one
forbidden:
  - tinygodriver importing tinybind-go, in any package, example or test
  - a cborbind example living in tinygodriver
no_registry:
  what: generated code emits methods and registers nothing, as decision:dynamobind-static-dispatch already establishes for a mapping mode
  why_it_is_not_a_preference_here: a registry lookup is per message, at tick rate per player, against the 9.2 ns encode the driver measured; the lookup would be a visible fraction of the whole codec
  consequence: no generated init function, and a type with no generated code fails to compile rather than at run time
  dispatch_at_depth: a nested or foreign field resolves at generation and the emitted call names one path, with no runtime type switch, which is what the driver's own reference codec already writes by hand
generated_code_placement:
  location: the user package
  may_import: cborbind and the driver
  declares: only methods of its own declared types, per decision:generated-runtime-in-module and rule:generated-source-self-contained
name_question:
  raised: 2026-08-19, in the requirements
  still_open_after_building: yes; one package carries both profiles today and nothing about the build argued against it
  what: one package name covers two profiles that differ in kind, a frozen realtime format and an evolvable document format
  assumed: one package with two declaration forms, per requirement:declared-cbor-codec
  worth_settling_before: the API is published, since a later split moves an import path
related:
  - system:tinygodriver-cbor
  - requirement:declared-cbor-codec
  - decision:dynamobind-static-dispatch
  - decision:generated-runtime-in-module
  - requirement:tinygo-wasm
  - system:tinybind
```
