---
id: requirement:stack-resident-json-parser
type: requirement
title: Keep The Parser And The Result Off The Heap
---
Stop the two fixed allocations every generated decode pays before it reads a byte: the Parser moved to the heap because ParseSlice takes a func(*Parser) callback, and the result boxed by any(&out) in decodeThroughInterface.

```yaml
priority: should
source:
  - go build -gcflags=-m over a generated package 2026-09-12: moved to heap: p at every decodeXBytes
  - allocation profile of the Order decode 2026-09-12: 15 allocs, of which 11 are the strings the document holds and 4 are structure
  - system:odjson, whose parser state is an int index and whose slice loops are emitted inline
review_gate: proposed
status: parser half implemented 2026-09-12; the boxed result is left as it was, see as_built
as_built:
  loops: generator/emit.go emitStreamSlice, emitStreamArray and emitStreamMap write the loop into the generated code with emitElemRead; ParseSlice, ParseArray and ParseMap stay for callers outside generated code, and jsonbind.ArrayTooLong keeps the error text the two agree on
  verified: go build -gcflags=-m over internal/benchfixture reports no moved to heap for the Parser; the decode fell from 16 to 15 allocations
  in_place_2026_09_12_later: the generated decoders write into a destination the caller owns -- decodeXJSON(p, out *X), decodeXInto(data, out *X) -- so a nested struct is decoded into its field, a slice element into the slot append made, an array cell in place, and DecodeJSONFrom into its receiver; a value returned through every level was zeroed and copied once per level, measured at 24 ns on a 144-byte struct for an empty document and 260 ns on a 1296-byte one; decodeXBytes keeps the value-returning signature the registry needs, one copy at the boundary
  result_2026_09_12_later: the registry now wins when both exist, so decodeThroughInterface runs only for a type the registry does not hold and an annotated type decodes without the boxing; see requirement:json-codec-interface precedence.method_over_plan.reversed_2026_09_12
the_four_structural_allocations:
  parser: 'var p jsonbind.Parser in decodeXBytes escapes because &p flows into ParseSlice''s read func(*Parser) argument, a function value the compiler cannot see through'
  result: decodeThroughInterface does any(&out).(Decoder), which boxes the pointer and moves out to the heap; carriesDecoder already avoids that for the probe, the conversion after it does not
  slices: ParseSlice starts every populated slice at make([]T, 0, 8); odjson starts at 4 and appends, which is one allocation for the three-element arrays both fixtures hold either way
  keep: the slice allocations are the document's; the first two are the runtime's and are the target
shape:
  inline_slice_loops: emit the array loop into the generated decoder as odjson does, calling the element decoder directly, so no func value takes the parser's address; ParseSlice stays for callers outside generated code
  or_index_state: alternatively pass position as an int the way odjson does and hold the buffer in an argument, which leaves nothing to escape at all; larger change, same effect
  typed_decode_call: emit the Decoder route as a direct method call on a concrete *T when the type is known at generation time, and keep the interface conversion for the runtime-only route
expected: two allocations and their GC share per decode; the CPU profile of the small-document decode was over half runtime GC and scheduler frames, which is the cost of allocation rate rather than of parsing
verify:
  - gcflags=-m shows no moved to heap for p or out in the generated package
  - the Order benchmark drops from 15 to 13 allocs; the assertion, if written, follows rule:allocation-assertion-toolchain-guard
related:
  - requirement:decoder-key-decision-tree
  - requirement:json-codec-interface
```
