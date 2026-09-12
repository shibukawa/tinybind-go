---
id: requirement:json-scalar-fast-paths
type: requirement
title: Scalar Fast Paths For Numbers And Whitespace
---
Give the parser and the appender the small fast paths odjson carries for the values that dominate typed documents: a plain decimal integer read without strconv, a simple float read for short decimals, a float written as the short decimal it was parsed from, and whitespace skipped with a first-byte test that inlines.

```yaml
priority: could
source:
  - system:odjson odjsonrt/decode.go ParseDecimal and ParseSimpleFloat, ftoa.go appendShortFloat, scan.go SkipSpace, read 2026-09-12
  - CPU profile 2026-09-12: ws was the second flat user-code frame of a Book decode; strconv.readFloat appeared for the three floats
review_gate: proposed
status: implemented 2026-09-12; see as_built
as_built:
  whitespace: Parser.ws returns after one compare when the next byte is above the space, which is every byte of a compact document
  depth: the nesting bound is read once per Reset into Parser.maxDepth, not once per container through the atomic
  float_in: parseFloatFast in jsonbind/parser.go takes a span with no exponent and at most nineteen digits whose integer is below 2^53 and whose fraction has at most 22 digits, and divides by an exact power of ten; anything else goes to strconv; jsonbind/fastpath_test.go compares it with strconv over 200000 random decimals
  float_out: appendShortFloat in jsonbind/append.go writes an integer below 2^53, or a decimal of at most three fraction digits below 1e9 found by trying the lengths in order and keeping the first whose division returns exactly the value; zero, anything larger and every other value stay with strconv; fastpath_test.go compares it with encoding/json over 200000 random values including every bit pattern shape
  decimal_int: parseInt64 already read digits in place; left as it was
paths:
  whitespace: 'SkipSpace checks data[p] > '' '' first and returns, so the common no-space case is one compare inlined at every call site; jsonbind ws() is a loop with a switch, called before every token'
  decimal_int: parse digits in place with overflow detection and only fall to the general path on a sign, exponent or fraction; jsonbind parseInt64 already does most of this
  simple_float: a mantissa of up to 19 digits with a short fraction and no exponent converts exactly by table, and only the rest goes to strconv.ParseFloat
  short_float_out: a float that was a short decimal prints as that decimal, proven with exact integer arithmetic, and only otherwise pays strconv shortest formatting; odjson measured 16 ns against 26 ns per float
must_hold:
  - the bytes written stay identical to encoding/json's, which the differential fuzz in the repo already checks
  - the number grammar numberSpan enforces stays enforced; a fast path reads what the span validated
expected: small on the small documents, a few percent; listed because it is cheap and independent of the other proposals
related:
  - requirement:word-at-a-time-json-scanning
```
