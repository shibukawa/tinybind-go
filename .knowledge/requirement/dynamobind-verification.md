---
id: requirement:dynamobind-verification
type: requirement
title: dynamobind Verification And Size Budget
---
Prove the generated codec by golden output, a real round trip, and a binary that grows no faster than the hand-written map path.

```yaml
status: required
golden:
  scope: one case per data:dynamodb-attribute-mapping row, plus each generation error in rule:dynamo-tag-options
  form: generated output compared byte for byte
runtime_behavior:
  - an iterator threads WithExclusiveStartKey across pages and stops early without a further request
  - a batch larger than the limit is chunked, and unprocessed entries come back unretried
  - StoreReturning and RemoveReturning report false when no previous item existed
  - every driver sentinel survives errors.Is through each helper
round_trip:
  implemented: an in-process fake speaking the same JSON wire shapes over httptest, so the driver's request building, signing, paging and response decoding all run in the normal test suite with no daemon
  not_implemented: DynamoDB Local; it remains the check worth adding for service semantics this fake does not model, and it needs "docker run -p 8000:8000 amazon/dynamodb-local -jar DynamoDBLocal.jar -inMemory -sharedDb", where -sharedDb is required or the server partitions data by access key and region
  fake_scope: storage, paging, batch limits and ALL_OLD only; no conditions, no capacity, no consistency, and nothing here should test DynamoDB semantics
  cases_that_broke_the_driver_codec:
    - multi-byte strings
    - the empty string
    - binary with high bytes
    - a nested map
    - a 38-digit number
no_new_reflect:
  source_check: the emitted file contains no reflect reference; asserted on the committed fixture codec and on generated output in the analyzer tests
  runtime: dynamobind resolves nothing by reflection; AsError walks the chain by type assertion because errors.As needs it
  note: reflect is linked regardless through the driver's encoding/json request path, per decision:dynamobind-json-transport-deferred, so a symbol count over the whole binary measures the driver rather than this
size_measured_2026_08_01:
  toolchain: tinygo 0.41.1, target wasip1
  program: store and read one four-field item, every row doing the same work including attribute-level error reporting
  raw_driver_hand_built_map_no_errors: 3,541,365
  driver_reflection_marshaler: 3,586,193
  hand_written_codec_driver_direct: 3,586,568
  hand_written_codec_through_dynamobind: 3,625,798
  generated_codec_through_dynamobind: 3,626,010
  codec_cost: +212 bytes against the same codec written by hand, which is the budget that matters and which holds
  context_client_cost:
    measured: +37,971 bytes, by building the same program against the previous commit where the client was a parameter
    not_ours_to_fix: a bare context.WithValue plus one type assertion, with no dynamobind linked, costs 48,409 bytes in the same program; the assertion pulls in type-descriptor machinery TinyGo otherwise drops
    consequence: decision:dynamo-context-client-api buys a call-site property at a fixed per-binary price, and the price is the largest single number here
  against_reflection:
    result: the generated path through dynamobind is 39,817 bytes LARGER than the driver reflection mapper, reversing the 2026-07-31 measurement
    why: a typed codec calling the driver directly is a wash against reflection at +375 bytes; the whole difference is the API surface, most of it the Context
    escape: the generated EncodeItem, DecodeItem and ItemKey are ordinary methods, so a size-critical program calls the driver directly and links none of this package
  drift_argument_unaffected: generating the codec is about names that cannot disagree, which holds at any size
  earlier_measurement:
    date: 2026-07-31
    void: taken against the client-parameter API, and its hand-written baseline reported no attribute errors, so neither its rows nor its conclusions carry over
build_paths:
  - "go test ./..."
  - "go test -tags force_tinygo_logic ./..."
  - "tinygo test ./internal/dynamofixture, wired into scripts/tinygo-check.sh"
  reason: the driver transport differs between the first two, and the third proves the codec runs on the target it exists for
fixture:
  package: internal/dynamofixture
  codec_tests: no net/http, so they run under TinyGo as well as the host
  client_tests: tagged !tinygo; they drive the driver over an httptest server, which TinyGo has no server for
  committed_output: a test regenerates and compares, so a stale codec fails the build rather than storing something wrong
environment:
  driver: system:tinygodriver-dynamodb at tinygodriver v1.1.3
  caveat: re-measure when the driver version moves or the runtime API changes; the budget compares paths within one build, not across releases, and the 2026-08-01 round exists because an API change invalidated the 2026-07-31 one
related:
  - requirement:dynamobind-product-goals
  - requirement:dynamobind-generated-item-codec
  - decision:dynamobind-static-dispatch
  - decision:dynamobind-json-transport-deferred
  - requirement:tinygo-wasm
  - decision:reflection-free
```
