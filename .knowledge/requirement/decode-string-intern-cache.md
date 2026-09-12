---
id: requirement:decode-string-intern-cache
type: requirement
title: Intern Repeated Strings Through A Pooled Cache
---
Optionally intern decoded strings through a small pooled hash table, so a value that recurs in one document, or across the documents one process decodes, is allocated once; measured as real on repetitive documents and as a benchmark artifact on a single looped one.

```yaml
priority: could
source:
  - system:odjson odjsonrt/jsontext.go StringCache, read 2026-09-12
  - the same-document versus rotating-document decode rows in system:odjson
review_gate: proposed
status: proposed 2026-09-12; needs a decision on whether cross-call sharing is wanted
mechanics:
  table: 256 string slots in a sync.Pool object, hashed from the first and last eight bytes and the length, strings of 2 to 256 bytes only
  hit: an equal string already in the slot is returned; a miss allocates and overwrites the slot
  validity_bit: a hit remembers that the entry was UTF-8 validated, so a repeated non-ASCII value skips the validation pass too
  lifetime: taken at the start of a decode and returned at the end, so entries survive across calls on the same P
what_it_bought_odjson:
  same_document_loop: the Order decode fell from 14 to 3 allocations, because every string was a hit after the first iteration
  distinct_documents: nothing on Order and Book, whose values do not repeat; the Twitter document repeats language codes and colour strings across a hundred statuses and does gain
caution:
  benchmarks: a decode benchmark over one document will show this as a large win that a server with distinct values never sees; measure with rotating inputs, as system:odjson's rows were
  retention: a pooled table keeps up to 256 strings alive between requests; acceptable for values, worth a thought for anything sensitive
  tinygo: sync.Pool exists there but is a plain allocation on the single-threaded targets, so the table would be per-decode; still correct
related:
  - requirement:stack-resident-json-parser
```
