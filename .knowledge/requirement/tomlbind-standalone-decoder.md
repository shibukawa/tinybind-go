---
id: requirement:tomlbind-standalone-decoder
type: requirement
title: Standalone TOML Decoder Package
---
Give TOML the entry point JSON and CBOR already have: a tomlbind package that decodes one restricted-TOML document into a struct through generated code, importing minitoml and nothing of configbind.

```yaml
priority: should
status: implemented 2026-09-20
review_gate: proposed
source:
  - maintainer 2026-09-20, noticing configbind exists but no tomlbind, and that a package usable on its own would be worth having
  - maintainer 2026-09-20, closing the open questions by precedent -- follow jsonbind and its siblings where a call is unclear
as_built:
  runtime: tomlbind/tomlbind.go; Decodable, DecodeTOML and DecodeTOMLBytes constrained on a pointer per decision:tomlbind-dispatch-shape, and GenerateDecoder returning a zero-size Declaration on the requirement:declared-json-codec spelling; imports io only
  emitter: minitoml/codegen gained FieldFloat, Value.AsFloat, and EmitApply with ApplyOptions{FuncName, ErrorPrefix, AllowEmptyPrefix}; Generate is unchanged and the pinned fixture still matches byte for byte, so configbind's output did not move
  generator: generator/tomlbind.go; OperationTOMLDecode over the three entry points through canonicalTOMLBindCalls, FeatureDecodeTOML as decode-toml, tomlBindSources walking the struct with the configbind collector under a floats switch only this pass sets, generateTOMLBind writing tomlbind_gen.go and tomlBindArtifacts one artifact per naming source file
  one_operation: a call and the declaration are one operation, since there is one direction and the method is always published; jsonbind's two exist for the method bit, which tomlbind has no use for
  generated: decode<T>TOML(dst *T, doc minitoml.Document) error with the root as prefix and errors opened tomlbind, and DecodeTOMLFrom parsing, resetting *v as jsonbind's DecodeJSONFrom does, and applying; nested structs and element structs are inlined, so only a named type gets a method
  foreign_type: a declaration naming another package's type is a generation error carrying the remedy, per requirement:declared-json-codec a_foreign_type_is_refused; an ordinary call naming one is left to the compiler, per requirement:cbor-codec-generation same_package_only
  tests: generator/tomlbind_test.go over discovery of a call and a declaration, a package calling nothing, the foreign declaration, the feature switch, and a compiled probe decoding a document, defaults, a replaced value, parse errors with line numbers, and a wrong kind or width named by key; minitoml/codegen tests over floats, the options and AsFloat; tomlbind's own test over a stand-in method
  tinygo: testdata/cmd/tinygo-toml-smoke runs natively, builds for wasm and runs under wasip1 on TinyGo 0.42.0, and scripts/tinygo-check.sh refuses its graph if a config, HTTP, SQL or root runtime appears
  docs: docs/tomlbind.md and its Japanese twin, a README row and guide link in both languages, and a tip at the head of the configbind guides
  empty_mapping_file: the CLI also writes an empty tinybind_gen.go beside tomlbind_gen.go for a package with no mapping targets, which is what it already does for testdata/configmodulea; not changed here
why:
  a_file_is_not_a_config_stack: configbind answers "load my service's settings" and links cliparser, configpath, system:configdir and go-envparse to do it; a program that wants one TOML file read into a struct pays all of that for the TOML layer alone
  the_family_has_a_gap: concept:standalone-json-codec and requirement:cbor-codec-generation give JSON and CBOR a typed Decode entry; TOML has only minitoml.Document and a hand-walked Get per key
  the_emitter_already_exists: minitoml/codegen Generate emits Apply<Type>(dst *T, doc minitoml.Document) error today, exercised only by minitoml/codegen/fixture; configbind/codegen borrows its Field, FieldKind and IntFieldSpec and emits its own Overlay-based apply, so the Document-based emitter has no public door and no generator wiring
what_exists_today:
  parser: minitoml, byte-at-a-time per decision:startup-parsers-stay-simple, restricted per decision:toml-shape-constraints, duplicate keys refused
  document: flat key path to Value, KindTableArray holding one Document per [[element]]
  emitter: minitoml/codegen, FieldString FieldBool FieldInt FieldDuration FieldStringSlice FieldStruct FieldStructSlice, sized integers per requirement:sized-integer-field-kinds, default literals, an error text prefixed minitoml
  writer: configbind/scaffold.go quotes and lays out TOML for the scaffold, with help comments; not a general encoder
  not_wired: tinybind-gen never calls minitoml/codegen.Generate; generator/execution.go reaches configbind/codegen only
scope:
  runtime: package tomlbind importing minitoml only; no cliparser, configpath, configdir, go-envparse, net/http
  entry_points:
    decode: 'DecodeTOML[T](r io.Reader) (T, error) and DecodeTOMLBytes[T](data []byte) (T, error), spelled after api:decode-json'
    shape: one shape only; TOML has no container axis the way decision:cbor-shape-is-the-only-axis names for CBOR, so the name carries the format and nothing else
    dispatch: decision:tomlbind-dispatch-shape
  generated: a per-type decoder built from minitoml.Parse plus the minitoml/codegen apply body with an empty prefix, its error text prefixed tomlbind and its function names owned by the tomlbind pass
  discovery: rule:usage-directed-generation; a call names the type, a package calling nothing regenerates byte for byte, feature key gated per rule:generator-feature-disable
  tags:
    key: the TOML key when the snake-cased field name is not it, the same spelling configbind reads
    default: the value when the key is absent, per decision:default-tag-form
    not_carried: opt, env, help, enum, falsy, dependon, summary, secret; those are configbind's layering and reporting, not the file
  types: decision:configbind-supported-types as the floor, plus float64 and float32 fields, since minitoml already parses KindFloat and the gap is a missing FieldFloat kind in the emitter
  toml_subset: decision:toml-shape-constraints unchanged; tomlbind documents the same allowed and forbidden list and does not widen the parser
  errors: a parse error carries minitoml's line and column; an apply error names the full key path; both are ordinary errors since decoding runs once per document, not per request
  no_interpolation: ${NAME} expansion stays in configbind per decision:env-interpolation-layer; tomlbind returns values verbatim
  tinygo: the emitted decoder links under requirement:tinygo-wasm; a smoke fixture on the cborbind pattern
  docs: docs/tomlbind.md and its Japanese twin, a README row beside jsonbind and cborbind, a system:tinybind runtime_packages line
what_stays_out:
  - an encoder; the scaffold writer is the seed if one is wanted later, and generalizing it is a second requirement
  - layering with env or CLI; that is configbind, and configbind keeps its own Overlay apply rather than being rebuilt on tomlbind
  - widening the TOML subset: quoted keys, inline tables, nested arrays, datetimes
  - a Codec noun or a registry; see decision:tomlbind-dispatch-shape
  - float in configbind; adding FieldFloat to the shared emitter makes it available, and whether configbind's Overlay apply admits it is decision:configbind-supported-types's call
acceptance:
  - a package calling DecodeTOML[T] gets T's decoder generated and nothing else; one calling nothing regenerates byte for byte
  - a decoded struct matches what configbind would bind from the same file under the same struct with only key and default tags, minus interpolation
  - a float64 field reads a TOML float and an integer literal; an int field refuses a float
  - a missing key with a default takes the default; without one it keeps the zero value
  - a duplicate key, a quoted key and an inline table are parse errors carrying line and column
  - a package importing tomlbind and calling nothing links no cliparser, configpath or configdir symbol
  - the emitted decoder builds for wasm and wasip1 under TinyGo and runs under wasmtime
  - the generator tests run with -count=1 and finish inside the package's own timeout
related:
  - decision:tomlbind-dispatch-shape
  - concept:standalone-json-codec
  - requirement:cbor-codec-generation
  - concept:reusable-source-parsers
  - decision:toml-shape-constraints
  - decision:configbind-supported-types
  - decision:startup-parsers-stay-simple
  - decision:env-interpolation-layer
  - decision:runtime-package-boundaries
  - decision:reflection-free
  - rule:usage-directed-generation
  - rule:generator-feature-disable
  - rule:same-package-convention
  - requirement:tinygo-wasm
  - system:configbind
  - system:tinybind
resolved_by_precedent:
  emitter: one emitter with options, as requirement:cbor-codec-generation shares its value emitters across the HTTP mode and both shapes so they cannot drift
  declaration: shipped in the first cut, as jsonbind's; it is not optional here, since the method must land in the type's package and a config struct is commonly declared away from the main that reads it
  returns_a_value: as jsonbind and cborbind do
  no_path_helper: jsonbind takes an io.Reader and nothing else; opening the file is the caller's line
