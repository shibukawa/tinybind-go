---
id: requirement:sized-integer-config-fields
type: requirement
title: Sized Integer Config Fields
---
Integer config fields keep their declared Go width and signedness through codegen, so any integer type binds without a narrowing conversion.

```yaml
priority: must
problem: >
  every integer width collapses into one FieldInt kind and generated apply
  assigns int(n), so an int64 field produces code that does not compile and a
  32-bit target truncates a value that parsed as 64-bit
supported_go_types: [int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64]
codegen:
  - field metadata carries the Go integer type, not the collapsed kind alone
  - parse at the field width per rule:integer-value-parsing
  - assign the parsed value without narrowing
  - validate a default tag at the field width during generation
surfaces: [toml file, env, cli flag, default tag]
non_goals:
  - float, complex, and uintptr fields, which stay out of decision:configbind-supported-types
  - byte-size suffixes such as 10MB; the value form stays a plain base-10 integer
  - changing how time.Duration is detected, which requirement:duration-config-fields owns
  - a defined type over an integer, such as 'type Level int'
defined_type_rejection:
  why: >
    generated code needs the declared name for the assignment and the
    underlying width for the parse, and a field carries one type name
  state: fails generation with the type named, instead of emitting an
    assignment that does not compile, which is what it did before
  note: an alias of an integer type is transparent per requirement:alias-transparent-type-analysis
acceptance:
  - an int64 field compiles and round-trips math.MaxInt64
  - a uint64 field accepts a value above math.MaxInt64
  - an int32 field rejects 3000000000 with a range error naming the term:config-key
  - an int field parses at strconv.IntSize, so a 32-bit target reports a range error instead of truncating
  - 'default:"..." outside the field range fails go generate'
  - the scaffold and provenance forms of an integer key are unchanged
related:
  - decision:configbind-supported-types
  - rule:integer-value-parsing
  - requirement:duration-config-fields
  - requirement:struct-field-metadata
  - decision:struct-field-tags
  - concept:config-struct-mapping
  - flow:configbind-codegen
  - system:configbind
```
