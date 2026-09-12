---
id: requirement:decoder-key-decision-tree
type: requirement
title: Match Object Keys With A Generated Decision Tree
---
Emit a byte-position decision tree over the struct's member names so the decoder settles a key with one or two byte switches and one fixed-length compare against the input in place, instead of scanning the key out as a span and then running switch string(key).

```yaml
priority: should
source:
  - system:odjson, generated odjsonParse, observed 2026-09-12
  - CPU profile of decodeBookJSON 2026-09-12: ObjectKey, objectKeySpan and stringSpan were the top user-code frames
review_gate: proposed
status: implemented 2026-09-12; see as_built
as_built:
  parser: jsonbind ObjectKey split into ObjectKeyRest, KeyMatched and ObjectKeyName; ObjectKey itself is unchanged for callers
  emit: generator/emit.go emitMemberWalk, emitKeyTree and emitKeyLeaf, shared by the codec decoder and the binder's JSON walk; a name outside printable ASCII or holding a quote or backslash is matched by the slow route only
  slow_route: reads the name the ordinary way and sets the same index through a string switch, so an escaped spelling of a declared name still binds
  measured: internal/benchfixture decode 683 ns to 634 ns on top of requirement:stack-resident-json-parser, best of 10
problem:
  today: generated decoders call Parser.ObjectKey, which walks the key byte by byte validating escapes and UTF-8, returns a span, and the emitted switch string(key) compares it against every candidate by length then bytes
  cost: every key pays a full scan plus a runtime string comparison, on the hottest loop of the decoder; a User with a dozen names starting profile_ compares them in turn
shape:
  select: the generator picks byte offsets that split the member-name set, emits nested switch statements on rest[k], and at each leaf compares the quoted literal including both quotes with one string(rest[:n]) == "\"name\"" test, which the compiler turns into a length check and memequal without allocating
  fallthrough: a miss at any level leaves the index at -1 and takes the existing slow path, which is where the quote and escape validation and the skip of an unknown member happen
  quoted_literal_in_the_tree: matching the closing quote inside the literal proves the key ended, so a member named "id" cannot match "identity"
keep:
  - exact-match semantics; no case folding, as concept:standalone-json-codec states
  - the existing error text for a malformed key, produced by the slow path
  - duplicate members still decoded on every occurrence
expected: a large share of the 1.8x-2.2x decode gap measured in system:odjson on the small documents, since key handling is per member and the small documents are mostly members
verify: benchmark internal/benchfixture decode before and after; the Order document has eleven members across three struct types
related:
  - requirement:stack-resident-json-parser
  - requirement:word-at-a-time-json-scanning
```
