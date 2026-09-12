---
id: requirement:word-at-a-time-json-scanning
type: requirement
title: Scan Strings Eight Bytes At A Time
---
Replace the byte-at-a-time loops in Parser.stringSpan and AppendString with a word-at-a-time (SWAR) scan that checks eight bytes per iteration for a quote, a backslash, a control byte, or a byte that needs escaping, and falls back to the byte loop only at a hit.

```yaml
priority: should
source:
  - system:odjson odjsonrt/swar.go and mode.go, read 2026-09-12
  - CPU profile 2026-09-12: stringSpan was the top flat user-code frame of a Book decode; on the Twitter encode odjson's SWAR escape scan was the whole of its 1.6x per-byte lead
review_gate: proposed
status: decode half implemented 2026-09-12; the encode half waits on the non-escaping mode named in encode_caveat
as_built:
  where: jsonbind/parser.go stringSpan, with swarStringStop, swarIndex and swarBelow beside it; both fuzz targets in jsonbind pass against encoding/json
  measured: internal/benchfixture decode 780 ns to 683 ns together with requirement:stack-resident-json-parser, best of 10; the two were not measured apart
mechanics:
  load: binary.LittleEndian.Uint64 over the slice, two independent words per iteration so the loads overlap
  classify: the classic has-zero idiom, (w - 0x0101..*c) &^ w & 0x8080.., fused for quote, backslash and below-0x20 in one expression
  index: bits.TrailingZeros64(mask) / 8 gives the offset of the first hit; a borrow can report a lane early but never late, and the byte loop settles it
  non_ascii: in the decode scan bytes >= 0x80 never match and are validated afterwards; in the encode scan the high-bit mask stops the word loop and a validator settles the run three bytes at a time for the CJK lead bytes, which is what keeps the Twitter encode from a second pass over every string
encode_caveat:
  odjson_measured: odjsonrt/encode.go appendQuoted, its HTML-escaping encoder, keeps a byte-at-a-time table loop and says why -- a word scan was tried there and measured no faster, because adding <, > and & widens the escape set until the mask costs about what eight table lookups do, and the lookups pipeline well
  where_the_98us_came_from: odjson's fast Twitter encode ran its json/v2 mode, which does not escape HTML at all; its HTML-escaping mode measured 148 us against jsonbind's 130 us for less output, that is level per byte
  therefore: the encode half of this requirement pays only for an output mode without HTML escaping; jsonbind always escapes HTML like encoding/json (concept:standalone-json-codec), so whether to offer a non-escaping mode is a separate decision, and until it exists the encode side is expected to gain nothing
  decode_unaffected: stringSpan stops on quote, backslash and control bytes only, the narrow set where the mask wins
stays_the_same:
  - encoding/json's escaping set on encode, including <, >, & and U+2028/2029, per concept:standalone-json-codec
  - invalid UTF-8 written as � on encode and coerced on decode
  - the escape validation stringSpan performs today, which moves to the byte-loop fallback
tinygo:
  pure: encoding/binary and math/bits only; no unsafe and no assembly, so it runs on every target this project builds for
  check: measure on wasm too, since a 64-bit word scan on a 32-bit wasm target may not pay the same
where:
  decode: jsonbind/parser.go stringSpan, which both ObjectKey and String use
  encode: jsonbind/append.go AppendString; requirement:append-style-json-escaper's htmlbind escaper is a second copy of the same loop
related:
  - requirement:decoder-key-decision-tree
  - requirement:append-style-json-escaper
```
