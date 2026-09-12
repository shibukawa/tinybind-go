---
id: system:jsonbench
type: system
title: jsonbench, The Standard Library's JSON Benchmark Datasets
---
The go-json-experiment/jsonbench module: six real documents and the Go types they decode into, which the encoding/json/v2 work measured itself against; used here 2026-09-12 as the cross-library yardstick after the odjson comparison, and as the source of the shapes requirement:nested-collections-and-pointers added.

```yaml
upstream: https://github.com/go-json-experiment/jsonbench, commit 6ccc987 (2025-09-06), BSD-3
datasets:
  canada_geometry: 270 KB, one FeatureCollection, nothing but float pairs; compact
  citm_catalog: 1.7 MB, maps keyed by numeric strings, small objects, many any; indented four spaces
  golang_source: 1.9 MB, a tree of nodes through slices, integers and one float per node; compact
  string_unicode: 18 KB, sixty strings of non-ASCII text
  synthea_fhir: 2.0 MB, a FHIR bundle with a ninety-field resource, pointers, nested slices; indented two spaces
  twitter_status: 632 KB, the sonic twitter document with a recursive retweeted_status pointer; indented two spaces
how_the_types_were_adapted: anonymous structs named; integer map keys read as the strings they are on the wire; ",string" integers and timestamps read as strings; jsontext.Value read as any; identical across the three packages the harness compares, and every codec's output checked equal as values
measured_2026_09_12:
  where: a scratch module with replace directives for both repositories, one process, best of three; the machine carried outside load through the run, so ratios are within a run and the figures approximate
  decode_tinybind_over_odjson: canada 1.20, citm 1.31, golang 1.05, string_unicode 0.83, synthea 1.26, twitter 1.36
  encode_tinybind_over_odjson: canada 0.95, citm 0.90, golang 0.99, string_unicode 1.03, synthea 1.27, twitter 1.37
  against_json_v2: decode 2.1x to 2.5x faster on every dataset; encode 1.2x to 4.6x
  what_the_datasets_taught:
    indentation: on the indented three the decode was more than half whitespace walked a byte at a time; wsSlow now measures the run after a newline two words at a time in the shape odjson's skipSpaceSlow has, and the number readers consume whitespace before their fast path looks, which took citm from 3.2x to 1.3x
    slice_capacity: eight elements reserved per slice was a third more bytes than json/v2 on synthea; sized by the element now
    shapes: the five silent drops and the spaced tag, filed under requirement:nested-collections-and-pointers
  where_the_rest_is: twitter and synthea are pointer-heavy and string-heavy documents where odjson's per-member call count is still lower, and its string cache holds values up to 256 bytes against 64 here
related:
  - system:odjson
  - requirement:nested-collections-and-pointers
  - requirement:inlinable-token-readers
```
