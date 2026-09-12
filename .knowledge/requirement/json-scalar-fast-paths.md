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
status: proposed 2026-09-12
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
