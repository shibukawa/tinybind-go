---
id: decision:tomlbind-dispatch-shape
type: decision
title: tomlbind Dispatches Through A Constrained Generic
---
tomlbind's entry point calls a generated method through a pointer constraint, as cborbind does, rather than a registry filled at init, as jsonbind does; a missing decoder is then a compile error and the package imports nothing but minitoml.

```yaml
status: accepted 2026-09-20, by the maintainer pointing at jsonbind and its siblings as the precedent; implemented the same day
review_gate: proposed
asked: requirement:tomlbind-standalone-decoder needs one of the two dispatch shapes the family already has
options:
  registry:
    precedent: jsonbind RegisterDecode at init, lookupDecoder at the call
    costs: an emitted init per package, a run-time missing-codec error, a map lookup and an interface value on every call
  constrained_generic:
    precedent: cborbind DecodeCBORInArrayFrom[T any, PT interface{*T; ArrayDecodable}], decision:dynamobind-static-dispatch
    costs: a first run over a package whose call does not yet type-check, which generator/load.go checkLoadedPackage already admits per requirement:cbor-codec-generation
chosen: constrained_generic
why:
  - a type with no decoder fails to build instead of failing at the first Load; the same trade decision:dynamobind-static-dispatch made
  - no emitted init, so a TOML-only package carries no registration code, which requirement:tinygo-wasm counts
  - the interface is spelled in byte slices and io.Reader, so tomlbind names minitoml only from the generated code's side and the runtime package stays a few lines
  - a hand-written DecodeTOMLFrom on a type wins and a generated one is refused, the rule:cbor-codec-interface-upstream shape without a driver to defer to
spelling:
  method: DecodeTOMLFrom(data []byte) error on the pointer receiver
  reader_entry: DecodeTOML reads r to the end, then calls the byte entry; a document is small and read once, so no streaming parser is owed
  type_argument: 'DecodeTOML[T](r) with PT inferred, so a call site writes one type argument as cborbind does'
not_chosen_because:
  registry: its one advantage, a foreign type reachable through a published method, is what requirement:json-codec-interface built for JSON's HTTP composition; TOML has no such composition to serve
related:
  - requirement:tomlbind-standalone-decoder
  - requirement:cbor-codec-generation
  - decision:dynamobind-static-dispatch
  - requirement:json-codec-interface
  - rule:cbor-codec-interface-upstream
  - decision:runtime-package-boundaries
  - requirement:tinygo-wasm
```
