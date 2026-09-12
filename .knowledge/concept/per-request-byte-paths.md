---
id: concept:per-request-byte-paths
type: concept
title: Where The odjson Techniques Apply Outside jsonbind
---
Survey of the repo's other hand-written byte-level code, run 2026-09-12 after system:odjson, recording for each per-request path which of the five proposed techniques applies and which do not; startup-time parsers are excluded by decision:startup-parsers-stay-simple.

```yaml
techniques:
  key_tree: requirement:decoder-key-decision-tree
  swar: requirement:word-at-a-time-json-scanning
  no_heap: requirement:stack-resident-json-parser
  scalar: requirement:json-scalar-fast-paths
  intern: requirement:decode-string-intern-cache
paths:
  htmlbind_escapers:
    what: AppendJSONString, JSONString, Renderer.WriteEscaped, Escape
    applies: swar, as requirement:html-escape-word-scan
    not: key_tree and intern, there are no keys; no_heap, the writer is already append-style
  cachekeybind:
    what: generated CacheKey concatenates one framed string per marked field; each KeyString call allocates its framing and the + chain allocates once more
    applies: no_heap in spirit, as an append-style KeyAppend beside KeyString so a generated key is one allocation, the same move requirement:append-style-json-escaper made for the escaper
    not: swar, values are short identifiers; key_tree, nothing is matched
    priority: could; keys are built per lookup, so it is per request, but the strings are short
  bindcore_query_and_form:
    what: ParseQuery cuts on & and =, Lookup walks pairs with MatchQueryPair decoding percent escapes on the fly
    applies: nothing; strings.Cut is IndexByte underneath and already vectorised, and the keys are the request's, so no tree can be generated for them
    note: the generated binder could match its known field names against the raw key with the same tree jsonbind would use, but the pair list is short and the lookup is not in any profile
  jsonbind_any:
    what: Parser.Any and AppendAny for rest maps and, after requirement:any-field-kind, any fields
    applies: swar through stringSpan automatically; intern, since the map keys of an any object repeat across documents
  cborbind:
    what: generated against tinygodriver's cbor package, which owns the byte loops
    applies: not here; a SWAR text scan for AppendText would be a tinygodriver catalog concept
  routetree_and_templates:
    what: parse Go source and templates at generation time
    applies: nothing; not a runtime path
excluded_at_startup: minitoml, cliparser, configbind env and interpolation, per decision:startup-parsers-stay-simple
reading: outside jsonbind the only clear candidate is the htmlbind escaper pair, and it needs a measurement against its existing prefilter first; everything else is either already vectorised by the standard library or not on a per-request path
related:
  - system:odjson
  - decision:startup-parsers-stay-simple
```
