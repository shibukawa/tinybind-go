---
id: decision:startup-parsers-stay-simple
type: decision
title: Startup-Time Parsers Are Sized For Binary Size, Not Throughput
---
minitoml, cliparser and configbind's env and interpolation readers run once at process start, so the word-at-a-time and decision-tree techniques in system:odjson are not applied there; a smaller binary is worth more than a faster startup parse.

```yaml
decided: 2026-09-12, by the maintainer, when the odjson techniques were surveyed against the other parsers in the repo
status: accepted
applies_to:
  minitoml: byte-at-a-time parser with line and column tracking, called from configbind load once per config file
  cliparser: argv walk at startup
  configbind: env file reading and ${} interpolation, once per load
  minitoml_codegen: the generated Apply functions look keys up in a map-backed Document, so a key decision tree has nothing to replace
why:
  frequency: each runs once per process, so the cost measured on jsonbind's per-request path is not paid here
  size: every fast path is code, and requirement:tinygo-wasm counts bytes; a config parser is on the TinyGo path and carries none of the SWAR helpers
  clarity: the line and column bookkeeping these parsers keep for their error messages is what a word scan would have to work around
not_covered: jsonbind, htmlbind's escapers and the per-request query and form readers, which run per request and are where concept:per-request-byte-paths looks
related:
  - system:odjson
  - requirement:tinygo-wasm
```
