---
id: requirement:encode-size-hint
type: requirement
title: Size A Fresh Encode Buffer From The Type's Last Output
---
Keep a per-type high-water mark of encoded length, so an encoder that has to hand its result to the caller rather than borrow a pooled buffer allocates once at the right size instead of growing through several reallocations.

```yaml
priority: could
source:
  - system:odjson odjsonrt/sizehint.go, read 2026-09-12; its MarshalJSON allocates through SizeHint.New and records the result
  - measured 2026-09-12: jsonbind's append into a nil buffer costs 7 allocations and 1016 B on the 312 B Order, against odjson's 1 allocation and 352 B
review_gate: proposed
status: proposed 2026-09-12
mechanics:
  hint: one atomic int64 per generated type, holding the largest length recorded so far plus an eighth of headroom, floored at 64 bytes
  grows_only: a single large value raises the hint for good, which trades some memory on later small values for never growing again
  record: after every successful encode, a compare-and-swap that stores the new length only when larger
where_it_pays:
  append_jsonto_with_nil: AppendJSONTo(nil) from requirement:json-codec-interface, which a caller assembling its own record calls; the interface cannot borrow a pooled buffer because the bytes leave with the caller
  not_encode_json: EncodeJSON and the httpbind writers already borrow from jsonbind.GetBuffer and return it, so growth is amortised after the first request and a hint would add an atomic load for nothing
  not_the_generated_encoder_body: the body appends into whatever it is handed; the hint belongs in the entry that allocates
tinygo: sync/atomic is present; on a single-threaded wasm target the CAS is a plain store
verify: AppendJSONTo(nil) on the Order fixture drops to one allocation; assertion under rule:allocation-assertion-toolchain-guard
related:
  - requirement:json-codec-interface
  - requirement:stack-resident-json-parser
```
