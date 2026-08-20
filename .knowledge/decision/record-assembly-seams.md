---
id: decision:record-assembly-seams
type: decision
title: Record Assembly Seams
---
Accept the downstream request to export htmlbind's JSON string escaper in append form, and keep it in htmlbind rather than routing the caller to jsonbind's, because the two escapers are not interchangeable and the htmlbind one is the policy generated HTML already writes against.

```yaml
source:
  - downstream framework live-delivery allocation report 2026-08-21, against v0.5.20
  - decision:framework-integration-seams
  - decision:runtime-package-boundaries
review_gate: accepted 2026-08-21, implemented the same day
round:
  when: 2026-08-21, against the shipped live runtime rather than against the plan
  reporter: the downstream framework, which ships the live record writers and had already absorbed every cost it could
  size: one item, which is why it carries a proposal rather than a ranking
  precedent: decision:live-integration-seams and decision:partial-transfer-seams, the earlier rounds from the same reporter against the same runtime
verification:
  method: every claim checked against htmlbind and against the reporter's own source before acceptance, not read off the report
  three_copies: confirmed by AllocsPerRun, three allocations per escaped field at v0.5.20
  string_constraint: confirmed; JSONString is ~string and the fragment field is []byte
  scratch_reuse: confirmed in the reporter's source; the record writers take scratch and return it grown, and the comment states the intent the escaper defeats
  cannot_reuse_content_appendjson: confirmed; the reporter's record carries a kind tag and a validator digest, so the module's own record shape does not cover it
  generic_core: confirmed by compiling it — the bulk copy compiles for both members of the type set and the windowed rune decode adds no allocation, so the reporter's implementation note holds and the two-wrapper fallback it offers is not needed
  omissions_in_the_report: three further call sites in the reporter's own writers take the same shape and are not listed; recorded in requirement:append-style-json-escaper call_sites
two_escapers:
  finding: jsonbind.AppendString is already exported, already append-style, and already escapes for a script context, so the obvious answer is to point the caller at it
  why_that_answer_is_wrong:
    divergence: on invalid UTF-8, htmlbind emits the three raw UTF-8 bytes of U+FFFD and jsonbind emits the six ASCII characters of a backslash-u-fffd escape; every other case agrees byte-for-byte
    consequence: the two produce different bytes for the same input, so a caller mixing them writes records that differ by which function filled a field
    still_a_copy: jsonbind.AppendString takes a plain string, so the []byte conversion the report is about survives the switch
  boundary: decision:runtime-package-boundaries makes htmlbind a transport-neutral leaf owning the JSON formatters generated HTML uses, and does not list jsonbind among what it may import; routing this through jsonbind would add the module's first htmlbind-to-jsonbind edge to save a duplicated loop
  decided: export from htmlbind, leave jsonbind alone, and record the divergence rather than discovering it later from a wire diff
  not_decided: whether the divergence should be closed at all; both decode to the same string, so it is a byte-identity question and not a correctness one, and closing it changes bytes on a shipped wire
accepted:
  what: requirement:append-style-json-escaper
  value: high; it is a cost the reporter cannot relocate, and the module pays a share of it internally at Content.AppendJSON
  cost: low; one exported generic wrapping a loop that already exists, with no signature change and no output change
  breaking: none; JSONString keeps its name, constraint, and bytes, so decision:generated-runtime-in-module _tinybindJSONQuote and every generated encoder are untouched
  status: implemented 2026-08-21, with the as_built findings in requirement:append-style-json-escaper
principle:
  applies: the decision:framework-integration-seams rule, widen a seam whose default output stays identical and whose contract stays the caller's
  fits: this is the seam Content.AppendJSON already declares open — framing is the caller's — with the escaping the module keeps for itself
  reading: a module that tells a caller to own the framing owes it the primitives the framing needs, or the seam is open in the documentation and closed in the API
  same_reading_as_08_08: decision:partial-transfer-seams accepted that a caller assembling its own transfer needs what the module already computes; this is the same argument one layer down, about a byte-level helper rather than a render output
severity:
  reading: not a defect; the output is correct today and only its cost is wrong
  bound: the waste is proportional to escaped bytes per delivery per client, so it grows with subscriptions rather than with requests
  worst_case: a live dashboard fanning large fragments to many clients allocates three times per field where the design intended zero
observation_for_the_reporter:
  what: WriteLiveClose appends its reason between quotes with no escaping at all, unlike every other string field in the same file
  status: not a defect today, because every call site passes a package constant
  why_stated: it is the one field that would not be fixed by adopting this API, so adopting it would leave a single unescaped field behind in a file that otherwise escapes everything
related:
  - concept:live-boundary-updates
  - requirement:live-signal-emission
  - rule:signal-payload-trust
  - decision:caller-owned-wire-versioning
```
