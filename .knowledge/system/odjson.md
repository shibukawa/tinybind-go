---
id: system:odjson
type: system
title: odjson, An encoding/json/v2 Accelerator Generator
---
External Go code generator (github.com/mazrean/odjson) that emits per-type MarshalJSONTo / UnmarshalJSONFrom for encoding/json/v2; measured against jsonbind on 2026-09-12 as the reference for what a pure-Go generated codec can reach, and as the source of the decoder techniques proposed for this module.

```yaml
upstream: https://github.com/mazrean/odjson, commit 569737b (2026-09-11), Apache-2.0
shape:
  what_it_emits: MarshalJSONTo, UnmarshalJSONFrom, MarshalJSON, UnmarshalJSON on every named type; call sites keep calling json.Marshal / json.Unmarshal
  runtime: github.com/mazrean/odjson/odjsonrt, imports encoding/json/jsontext and unsafe
  go_coupling: the fast path is enabled only on a verified Go minor (go1.27); any other version silently falls back to the public jsontext API
  contrast_with_jsonbind: jsonbind is its own API and its own parser with no encoding/json dependency (concept:standalone-json-codec, decision:reflection-free); odjson is a drop-in accelerator for a stdlib API this module keeps out of TinyGo builds (decision:jsonv2-bench-stays-opt-in, requirement:tinygo-wasm)
measured_2026_09_12:
  where: one process, Apple M3, Go 1.27.0, darwin/arm64, medians of 5-10 runs; harness was a scratch module with replace directives for both repos, identical type files per package, outputs checked byte-equal on Order and Book
  payloads: this module's 312 B Order, odjson's 365 B Book, odjson's 616 KiB Twitter
  decode_rotating_distinct_documents:
    order: odjson 437 ns 14 allocs; jsonbind 798 ns 15 allocs; json/v2 reflect 1.7 us 18 allocs
    book: odjson 443 ns 10 allocs; jsonbind 992 ns 18 allocs; json/v2 reflect 2.1 us 15 allocs
  decode_twitter_same_document: odjson 412 us; jsonbind 812 us; json/v2 reflect 1047 us
  encode:
    order: odjson 234 ns; jsonbind 261 ns into a reused buffer; json/v2 reflect 850 ns
    book: odjson 227 ns; jsonbind 229 ns
    twitter: odjson 148 us in HTML-escaping mode, 98 us in its json/v2 direct mode; jsonbind 130 us writing 210 KB where odjson writes 255 KB
  reading: decode is 1.8x-2.2x behind; encode is level under equal escaping rules, and odjson's 98 us Twitter row is its json/v2 mode, which skips HTML escaping and writes into the coder's buffer directly -- neither is a technique jsonbind can take as it stands
  encode_techniques_and_where_they_went:
    swar_escape_scan: only in the non-HTML-escaping modes; odjson's own HTML-escaping encoder keeps a table loop because the scan measured no faster there; requirement:word-at-a-time-json-scanning encode_caveat
    short_float_formatting: requirement:json-scalar-fast-paths
    size_hint: requirement:encode-size-hint
    pooled_buffer_object: odjson pools a *Buffer to avoid boxing a slice header; jsonbind already pools *[]byte, nothing to take
    direct_buffer_path: not portable, below
  why_rotate: json/v2 and odjson both keep a string cache across calls, so looping one document allocates no strings after the first pass and overstates both; 4096 documents with distinct string values were cycled for the decode rows
remeasured_2026_09_12_after_adoption:
  what_landed: requirement:word-at-a-time-json-scanning decode half, requirement:stack-resident-json-parser parser half, requirement:decoder-key-decision-tree; the three generator gaps
  decode_rotating_distinct_documents:
    order: odjson 434 ns 14 allocs; jsonbind 648 ns 14 allocs, was 798 ns 15 allocs
    book: odjson 464 ns 10 allocs; jsonbind 826 ns 17 allocs, was 992 ns 18 allocs
  decode_twitter_same_document: odjson 376 us; jsonbind 655 us, was 812 us, now reading the interface{} fields it used to drop
  benchfixture_readme_row: 777 ns to 634 ns
  reading: the gap went from 1.8x-2.2x to 1.5x-1.8x; what remains is the string allocations both pay, the per-token whitespace and depth bookkeeping of requirement:json-scalar-fast-paths, and odjson's string cache on the repetitive document
remeasured_2026_09_12_after_the_second_round:
  what_landed: requirement:json-scalar-fast-paths, requirement:decode-string-intern-cache, requirement:encode-size-hint, and the registry-first precedence of requirement:json-codec-interface
  decode_rotating_distinct_documents:
    order: odjson 466 ns 14 allocs; jsonbind 630 ns 13 allocs, from 798 ns before the first round
    book: odjson 460 ns 10 allocs; jsonbind 675 ns 9 allocs, from 992 ns
  decode_twitter_same_document: odjson 385 us 2466 allocs; jsonbind 635 us 2682 allocs, from 812 us; the cache took the any-field decode from 4349 allocations to 2682
  encode_small: level; order 212 ns reused buffer against odjson 236 ns, book 189 ns against 225 ns
  append_to_a_nil_buffer: AppendJSONTo(nil) 251 ns 1 alloc 352 B, where the raw append function into nil costs 353 ns 7 allocs 1016 B; the size hint is what separates them
  benchfixture_readme_rows: decode 634 ns 15 allocs to 534 ns 4 allocs, encode 283 ns to 227 ns; the four allocations are the two slices, the body buffer and the reader, the strings being cache hits on a repeated document
  reading: the gap is 1.35x-1.5x on the small documents and 1.65x on the large one, from 1.8x-2.2x at the start; the machine carried intermittent outside load during these runs, so medians of five with the run repeated until the reflection rows matched earlier idle runs
  not_taken_from_here: the direct buffer path and the non-HTML-escaping mode, per not_portable and requirement:word-at-a-time-json-scanning encode_caveat
techniques_by_portability:
  portable_pure_go:
    - requirement:decoder-key-decision-tree
    - requirement:word-at-a-time-json-scanning
    - requirement:stack-resident-json-parser
    - requirement:json-scalar-fast-paths
    - requirement:decode-string-intern-cache
    - requirement:encode-size-hint
  not_portable:
    direct_buffer_path: writes into and reads out of the jsontext coder's private buffer through reflect-computed offsets and unsafe, gated on a verified Go minor; needs jsontext, which costs 3.5x stripped wasm size here, and a per-release re-verification this project does not want
    string_modes: ModeStream / ModeV2 skip HTML escaping and UTF-8 validation because jsontext redoes both; jsonbind's bytes go straight to the wire, so it cannot drop either
    jsontext_token_driven_decoder: the large-document decoder drives a jsontext.Decoder token by token; irrelevant without jsontext
found_while_measuring:
  decode_json_bytes_not_discovered: api:decode-json's DecodeJSONBytes is not a generator discovery pattern; a package calling only it gets encoders and no decoder, failing at runtime; GenerateCodec[T]() was the workaround (requirement:declared-json-codec)
  interface_fields_dropped_silently: an interface{} struct field is omitted from both codecs with no diagnostic; the Twitter output lost 17% of its bytes this way
  all_unsupported_struct_breaks_compile: a struct whose every field is unsupported gets no codec, yet the parent's generated code still calls appendXJSON and decodeXJSON, so the DO NOT EDIT file does not compile; a hand-written stub was the workaround
  filed_as: requirement:decode-json-bytes-discovery, requirement:any-field-kind, requirement:empty-struct-codec
other_parsers: concept:per-request-byte-paths surveys where the techniques apply outside jsonbind; decision:startup-parsers-stay-simple keeps them out of the config and CLI parsers
```
