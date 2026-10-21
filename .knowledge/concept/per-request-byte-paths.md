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
  htmlbind_render_against_quicktemplate_2026_09_12:
    harness: the benchfixture five-row user page written once as a .qtpl and once as page.tb.html, both rendering to a discarding writer, outputs byte-equal
    before: quicktemplate 342 ns 0 allocs; htmlbind 1016 ns 712 B 7 allocs
    allocations_removed: RenderChain takes its Renderer and renderOptions from a sync.Pool, which the synchronous path can hand back once composed returns, and the renderer's conversion buffer comes back with it; Bind allocates one bound[P] holding the plan and its parameters and answering as render, prepare and check, where three closures over the same values were three allocations and the parameters boxed once more; a plan with nothing to prepare renders through that same object
    after: htmlbind 3 allocs 152 B, of which one is the harness writer's conversion buffer; about 950 ns against quicktemplate's 390 ns under the same outside load
    what_is_left: the time is the op interpreter -- an interface call and a closure call per instruction, and per row for a loop -- where quicktemplate emits straight-line writes; that is the price of the plan being data a boundary, a cache and a live delivery can walk, and closing it would mean emitting Go per component the way the CBOR and JSON codecs are emitted
    fixture_fixed: the generated probe in templates/htmlbind/builtin_test.go shared two package variables between tests marked t.Parallel, and one test's token was read by another's render; the probes run one at a time now
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
second_survey_2026_09_12:
  asked: whether the same fixes apply to the other conversion paths, config and CLI excluded
  form_body:
    was: net/http ParseForm built url.Values and a copy into map[string]string; the generated binder indexed the map; 1037 ns 21 allocs on the benchfixture form row
    now: bindcore.FormValues holds an urlencoded body as the same key=value spans the query string is split into, unescaped only at a lookup, and a multipart body as the map its parser built; ReadFormBody on both transports returns it, generated binders call FormLookup and Present, and the rest maps iterate the pairs first-wins; fasthttp splits ctx.PostBody through the same bindcore.ParseQuery rather than the driver's PostArgs, so the two transports admit the same pairs
    bound: bindcore.MaxFormBodyBytes, the 10 MiB net/http applied, refused as a 400 form_parse
    measured: benchfixture HandlerGeneratedFormBind 1037 ns 21 allocs 2361 B to 571 ns 10 allocs 1128 B, best of ten; the JSON handler row also fell from 8 to 6 allocations through the string cache
    not_done: a single walk over the pairs with a key tree; the per-field lookup is a handful of string compares on a short pair list and was measured as not worth the restructuring of the query arm
  cachekeybind:
    was: KeyString and friends returned strings joined with +, one allocation per field and one more for the join
    now: AppendKey* helpers frame onto a caller-owned buffer, the generated CacheKey builds the whole key in one buffer sized by a per-type cachekeybind.SizeHint and converts once; the string helpers stay and cachekeybind/append_test.go pins that both spell every kind identically
    measured: a three-field key with a two-element slice, old shape against new in one process, 167 ns 6 allocs to 71 ns 2 allocs; the two left are the buffer and the string it becomes
  cborbind:
    key_tree: the map-shape decoder settles a key by a switch on its length and then the byte positions that tell the names apart, through generator/emit.go emitKeyTree in its exact-match mode; a name outside printable ASCII falls to a string switch on a miss
    already_had: a stack-resident reader (cbor.ReaderOver by value) and header-sized slices, so nothing to take from requirement:stack-resident-json-parser
    measured: a four-member map with two slices, 348 ns to about 190 ns through the generic entry, allocations unchanged; the machine carried outside load, so read the ratio not the figure
    size_hint_not_applied: cborbind imports nothing by contract and its entry points take the caller's buffer, which the game loop reuses; a hint would need an import or a copy
    finding: DecodeCBORInArrayFrom and DecodeCBORInMapFrom reach the generated method through a pointer-constrained type parameter, which the compiler cannot devirtualise, so the result is boxed: three allocations where calling the method on a local costs two, measured 2026-09-12; the fix that removed the same allocation in jsonbind was a registry, which the import-free contract rules out here; left for the maintainer
  htmlbind_jsonfloat:
    was: strconv 'g', so a million rendered as 1e+06
    now: encoding/json's spelling, plain notation with an exponent only below 1e-6 or from 1e21, exponent leading zero trimmed; htmlbind/jsonfloat_test.go compares with encoding/json; decided by the maintainer 2026-09-12 when asked which format to keep
  query_binding: unchanged; see form_body.not_done
third_survey_2026_10_21:
  asked: what the net/http handler path still allocated and spent after the second survey, read from an allocation profile of the benchfixture handler rows
  response_headers:
    was: WriteJSONBytes declared Content-Length as []string{strconv.Itoa(n)}, two allocations per response once the body passed 99 bytes, on a path that otherwise allocated nothing; WriteStatus, WriteError and WriteCBORBytes set Content-Type through Header.Set, one allocation each
    now: contentLength answers a body under 1024 bytes from a table of one atomic pointer per length, each entry built on first use and shared for the life of the process under the contract jsonContentType already carried; above the bound the two allocations are paid against a body at least that long; the three other writers store a shared Content-Type slice, and WriteError and WriteCBORBytes declare their length the same way
    bound: 1024 entries, eight kilobytes of pointers in BSS and nothing until a length is answered; TinyGo 0.42 runs the table in tinycheck
  content_type:
    was: every bind read Content-Type through Header.Get, which re-canonicalizes the constant key, then MediaType cut, trimmed and case-folded a value that needs none of it; strings.ToLower alone was the cost of a query parse in the profile, and the form path paid it twice
    now: canonicalHeader indexes the header map by its canonical spelling for the constant keys the runtime reads itself, the same map entry Header.Get reaches since net/http canonicalizes every key it parses; MediaType answers application/json, the urlencoded form type and the empty value as they stand; a name from a struct tag still goes through HeaderValue and Get
  form_body:
    was: readFormBody read through io.ReadAll over a LimitReader, three allocations before the string the pairs alias
    now: jsonbind.ReadLimitHint sized by Content-Length, as the JSON body already was, with a declared length over the bound refused before a read; the buffer and the string it becomes are the two allocations left
  encoder: AppendString scans plain ASCII a word at a time, per requirement:word-at-a-time-json-scanning as_built_encode_2026_10_21
  measured: benchfixture allocation rows, which do not move with load: HandlerGenerated 6 to 5, HandlerGeneratedQueryBind 5 to 4, HandlerGeneratedFormBind 10 to 8 and 1128 to 624 bytes; of what remains, the body buffer and the harness writer's header map are the JSON row entire, the query row adds the pair slice and the one escaped value, the form row the pair slice and the body string; timings interleaved eight rounds under a load average above twelve: JSONEncodeGenerated -18%, HandlerGenerated -14%, the other rows inside their noise
  not_done:
    query_pairs: the one slice ParseQuery allocates stays, per form_body.not_done above; an inline array would make QueryValues a 160-byte value copied into every lookup
    json_body_buffer: pooling the buffer ReadJSONBody fills would need the generated binder to hand it back after the parse, a generator change with a fixture regeneration behind it, for one allocation the body's own size
    htmlbind_escaper: AppendJSONString in htmlbind is still the byte loop; requirement:html-escape-word-scan's measurement now exists on the jsonbind side and favours the same fold
excluded_at_startup: minitoml, cliparser, configbind env and interpolation, per decision:startup-parsers-stay-simple
reading: outside jsonbind the only clear candidate is the htmlbind escaper pair, and it needs a measurement against its existing prefilter first; everything else is either already vectorised by the standard library or not on a per-request path
related:
  - system:odjson
  - decision:startup-parsers-stay-simple
```
