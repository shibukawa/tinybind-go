---
id: requirement:append-style-json-escaper
type: requirement
title: Append-Style JSON String Escaper
---
Export htmlbind's JSON string escaper in append form over both string and byte-slice input, so a caller assembling its own record into a reused buffer escapes straight into that buffer instead of through two intermediate allocations and a conversion.

```yaml
priority: should
source:
  - downstream framework live-delivery allocation report 2026-08-21, against v0.5.20
  - decision:record-assembly-seams
  - decision:partial-transfer-seams
review_gate: proposed
status: implemented 2026-08-21; see as_built
problem:
  the_seam_is_already_the_callers: Content.AppendJSON and Signal.AppendJSON both state that framing around a record is the caller's to choose, because the framing has to match the client that reads it; the reporter took that seam and writes its own record shape
  its_shape_is_not_the_modules: the reporter's record carries a kind tag and a validator digest that neither module record has, so reusing Content.AppendJSON whole is not available to it
  what_the_seam_withholds: the escaper filling those records is unexported, so a caller writing its own framing reaches the same escaping only through JSONString, which allocates its result rather than extending a buffer
  string_only_type_set: JSONString is constrained to ~string, so a []byte fragment pays a conversion before the escaper runs at all
measured:
  cost: three allocations per escaped field, confirmed by AllocsPerRun against v0.5.20
  which_three:
    - the string conversion the ~string constraint forces on a []byte fragment
    - the result buffer appendJSONString builds
    - the conversion of that buffer back to string that JSONString performs
  what_it_defeats: the reporter's record writers take a scratch []byte and return it grown so one live response reuses a single allocation across every delivery; the three above land inside that design and undo it per field
  largest_payload: boundary HTML is by far the biggest field in the record, and it is the one paying the conversion
occurrence:
  per_field: every escaped field of every record, not once per record
  per_delivery: every boundary of every delivery, per concept:live-boundary-updates
  per_client: a live response is per subscription, so the cost multiplies by connected clients rather than by requests
  contrast_with_the_08_08_round: decision:partial-transfer-seams measured this same escaper's cost in bytes on the wire; this measures its cost in allocations on the server, and the two are independent
call_sites:
  downstream_reported: the boundary HTML and the build and head strings
  downstream_found_here: the reporter's proposal omits the boundary id, the validator digest, and the signal name, which take the same shape and are in the same three writers
  module_internal: htmlbind itself pays the conversion at async.go Content.AppendJSON, which escapes string(c.HTML) for a []byte field, so the export is not for downstream alone
not_workable_around_downstream:
  reimplement: a caller can copy the escaper, and then two copies of an escaping policy exist where rule:template-context-safety and htmlbind.Escape hold the module's copy authoritative; a policy fork is worse than the allocations
  jsonbind_instead: jsonbind.AppendString is exported and append-style, but it is a different escaper — see decision:record-assembly-seams two_escapers — and it takes a plain string, so the []byte conversion is unchanged
  reading: the residual cost cannot be removed from outside without forking the escaping table
proposal:
  signature: AppendJSONString[T ~string | ~[]byte](dst []byte, value T) []byte
  semantics: byte-for-byte identical to JSONString, which then delegates to it
  covers: the quote, the backslash, control characters, '<', '>', '&', U+2028, U+2029, invalid UTF-8 replaced by U+FFFD, and clean runs copied in bulk
  unchanged: JSONString keeps its name, its ~string constraint, and its output, so decision:generated-runtime-in-module _tinybindJSONQuote and every generated encoder are untouched
feasibility:
  verified: 2026-08-21, by compiling and measuring the generic shape rather than by reading it
  bulk_copy: append(dst, value[start:i]...) compiles for both members of the type set, so the clean-run copy needs no specialization
  rune_decode: the one string-typed call, utf8.DecodeRuneInString, is reached through a window of at most utf8.UTFMax bytes; the conversion stays on the stack and adds no allocation
  result: zero allocations appending a multi-byte fragment into a pre-grown buffer, so one generic core is enough and two concrete implementations are not required
  caveat: the current pre-grow branch sizes dst against len(value)+2 and must keep doing so for a caller that passes a nil or short buffer
acceptance:
  - appending a clean fragment into a pre-grown buffer costs zero allocations
  - AppendJSONString and JSONString agree byte-for-byte over a corpus covering empty input, every escaped byte, U+2028 and U+2029, truncated UTF-8, and multi-kilobyte markup
  - []byte input and the same bytes as a string produce identical output
  - JSONString's own output is unchanged for every input in that corpus
  - Content.AppendJSON escapes its HTML field without a string conversion
as_built:
  shipped: 2026-08-21
  entry: AppendJSONString[T ~string | ~[]byte](dst []byte, value T) []byte, in htmlbind/values.go
  one_implementation: the unexported appendJSONString was renamed rather than wrapped, so there is no second copy of the escaping table to drift; every internal call site moved to the exported name
  generic_core_held: the feasibility above was confirmed in place — the bulk copy needed no specialization and the windowed decode added no allocation
  internal_beneficiary: Content.AppendJSON escapes its fragment as bytes, so the module's own record writer stopped paying the conversion it was reported for
  jsonstring_delegates: JSONString keeps its ~string constraint and now calls the exported entry, dropping one conversion of its own
  proof_of_no_movement: the test keeps a verbatim copy of the pre-export escaper and runs the corpus through both, because a caller already embedding this output in a script element cannot re-verify byte identity for itself
  corpus: empty input, every byte below 0x80 alone and embedded, both line separators, a genuine U+FFFD, truncated sequences of every width, and multi-kilobyte markup
  allocations: zero for a []byte fragment appended into a pre-grown buffer, asserted for both AppendJSONString and Content.AppendJSON; the test also asserts JSONString still allocates, so the corpus keeps measuring what the entry removes
  allocation_assertions_are_guarded: the negative control above failed under TinyGo, which counts no allocations at all, so both assertions skip when the toolchain cannot measure; rule:allocation-assertion-toolchain-guard records the finding, which this was the repository's first allocation assertion to hit
  docs: the framework-owner guide, both languages, at the point where it already tells a caller the framing is theirs
  instantiation_collapse: JSONString converts to a plain string before the call, which costs nothing and keeps every ~string caller on one instantiation instead of one apiece
  tinygo_cost: 86 bytes on a wasm binary against v0.5.20, measured with tinygo 0.41 on a fixture exercising seven named string types; the collapse above accounted for 126 of the bytes it would otherwise have been, and requirement:tinygo-wasm is unaffected at this scale
open_questions:
  - whether Content.AppendJSON and Signal.AppendJSON should also gain a caller-supplied field, given the reporter re-implements both records only to add two fields
  - whether the invalid-UTF-8 divergence from jsonbind.AppendString is worth closing while this API is being added, per decision:record-assembly-seams
```
