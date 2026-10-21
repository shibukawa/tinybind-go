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
status: implemented 2026-10-21, after the jsonbind encode half measured in its favour; see as_built
as_built:
  where: htmlbind/escape_scan.go holds the word load, the two masks and their byte forms, htmlRun and jsonRun, htmlEntity and multibyteRun; Escape and escapeNeeded in ops.go, Renderer.WriteEscaped in plan.go and AppendJSONString in values.go run over them
  html_mask: ampersand and quote (0x26, 0x22) differ in one bit, as do the angle brackets (0x3c, 0x3e), so the five entity characters are three has-zero tests, and the high bit is one AND; no control-byte test, since HTML passes those
  json_mask: jsonbind's swarEscapeStop with the backslash in place of the apostrophe, over either member of the type set through one generic load the compiler fuses into a MOVD for both shapes
  prefilter: escapeNeeded is the entity scan alone, consulting utf8.ValidString only when a word carried a byte outside ASCII, so Escape still returns an unchanged value without allocating; WriteEscaped has no prefilter any more, one pass writes the whole value when nothing stops it
  multibyte: a run of valid runes stays with the rune decoder rather than returning to the word scan at every rune, so CJK text pays no more than before and less where the old ContainsAny byte loop ran over it
  oracles: the retired prefilter-and-rune-loop Escape and the retired AppendJSONString byte loop stay in escape_scan_test.go, with a corpus placing every stop at every offset of a word and a fuzz target over each; FuzzEscapeMatchesWriteEscaped still pins the pair
  measured_in_one_process_against_the_oracles:
    escape: 3-byte 15.2 to 5.4 ns, 12-byte name 16.8 to 6.7, 15-byte email 19.4 to 9.1, 35-byte sentence 25.2 to 10.6, 136-byte run 64.6 to 21.3, 136 bytes with one ampersand 300 to 77, 19-rune Japanese 48.6 to 34.8
    write_escaped_against_the_committed_tree: 3-byte 18.0 to 8.2, name 20.9 to 9.7, sentence 28.6 to 14.6, 136-byte run 66.8 to 27.1, with one ampersand 242 to 35, Japanese 52.7 to 50.1
    append_json_string: 5-byte 8.3 to 9.5, 15-byte 18.9 to 11.2, 35-byte 44.1 to 12.8, 136-byte 162 to 26.6, URL with three escapes 51 to 29, Japanese 82 to 76, an 864-byte markup fragment as bytes 1454 to 1245
    benchfixture: HTMLGenerated 1008 to 789 ns interleaved eight rounds against the committed tree under a load average of thirteen, the stdlib row flat
  cost: a nanosecond on a JSON string under eight bytes, as on the jsonbind side; the three-byte HTML value is three times faster because strings.ContainsAny over a value of eight bytes or fewer ran a rune loop with an IndexRune per byte
  prior_overturned: expected says nothing on short attribute values; the short values gained most, because the prefilter's fixed cost was theirs
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
