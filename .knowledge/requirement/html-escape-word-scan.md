---
id: requirement:html-escape-word-scan
type: requirement
title: Word-At-A-Time Scan For The HTML And JSON Escapers In htmlbind
---
Apply the eight-bytes-per-step escape scan of requirement:word-at-a-time-json-scanning to htmlbind's AppendJSONString and to Renderer.WriteEscaped, whose byte loops are the same shape as jsonbind.AppendString and run once per rendered text value.

```yaml
priority: could
source:
  - system:odjson mode.go appendQuotedStream, read 2026-09-12
  - htmlbind/values.go AppendJSONString and htmlbind/plan.go WriteEscaped, read 2026-09-12
review_gate: proposed
status: proposed 2026-09-12; measure before adopting
where:
  json_side: AppendJSONString is a copy of jsonbind.AppendString's loop with the same escape set; one SWAR scanner can serve both, which requirement:append-style-json-escaper's split of the two already made awkward
  html_side: WriteEscaped runs a rune loop over every value that escapeNeeded admits; the classified set is & < > " ' plus invalid UTF-8, five ASCII bytes and the high bit, which is one SWAR mask
what_already_exists:
  escapeNeeded: strings.ContainsAny plus utf8.ValidString, chosen because a fused byte loop measured three to four times slower than the two library calls; the comment on it records that
  therefore: a SWAR scan competes with that fast path, not with a byte loop; on a value that needs no escape it may not win, and on a value that does the rune loop starts only after the prefilter
  odjson_own_result: odjson tried a word scan on its HTML-escaping encoder and kept the table loop, per requirement:word-at-a-time-json-scanning encode_caveat; the HTML escape set here is the same width, so the prior is that this does not pay
expected: real on long text values with a few escapes, such as user content with an ampersand; nothing on the short attribute values most pages are made of
verify: internal/benchfixture HTML rows and htmlbind's own escape benchmarks, both directions, before any change lands
related:
  - requirement:word-at-a-time-json-scanning
  - requirement:append-style-json-escaper
  - concept:per-request-byte-paths
```
