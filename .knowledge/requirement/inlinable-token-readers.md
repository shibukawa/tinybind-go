---
id: requirement:inlinable-token-readers
type: requirement
title: Token Readers The Generated Decoder Can Inline
---
Give the generated decoder an inlinable fast half for every reader it calls between tokens, with the full reader called only on a miss, so a compact document costs a byte compare per token where it cost a function call.

```yaml
priority: should
source:
  - the gap analysis of 2026-09-12 in system:odjson, after the first two rounds: partial documents showed odjson and jsonbind level per string and per number and apart per token, with about twice the non-inlined calls per member on the jsonbind side
  - go build -gcflags=-m=2 over jsonbind: ObjectStart 101, ObjectKeyRest 145, keyColon 81, ArrayStart 101, ArrayNext 134 against a budget of 80, the call to the slow half alone costing most of it
review_gate: proposed
status: implemented 2026-09-12; see as_built
why_a_method_cannot_do_it_alone: a method that checks the fast case and then calls its own slow half does not inline, because the inliner charges the call at nearly the whole budget; the check has to sit in the generated code and the call be taken only when the check declines, which is the shape odjson's SkipSpace and AfterKey have
as_built:
  runtime: jsonbind/parser.go ObjectStartFast, ArrayStartFast, KeyFast, Rest, Skip, ColonFast, Colon, ArrayNextFast; each verified inlinable; the full readers keep the same fast case at their head for callers outside generated code
  numbers: Int64, Uint64 and Float64 read a plain integer or a short decimal in one pass off the input and hand everything else to the grammar check and the checked conversion, one call where there were two
  strings: the cache hit is written into Parser.String, so a recurring value costs a hash and a compare and no call
  emitted: generator/emit.go emitMemberWalk, emitArrayOpen, emitArrayNext, and the map open, on both the codec and the binder walks
  measured: with requirement:stack-resident-json-parser's in-place decoding landed in the same pass, benchfixture decode 534 ns to 428 ns best of ten; against odjson the rotating Order decode went from 1.35x to 1.12x and Book from 1.47x to 1.16x
related:
  - requirement:stack-resident-json-parser
  - requirement:decoder-key-decision-tree
```
