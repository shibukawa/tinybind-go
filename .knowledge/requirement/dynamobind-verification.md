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
size_measured_2026_07_31:
  toolchain: tinygo 0.41.1, target wasip1
  program: store and read one four-field item
  raw_driver_hand_built_map: 3,543,805
  hand_written_codec_through_dynamobind: 3,568,434
  generated_codec_through_dynamobind: 3,568,604
  driver_reflection_marshaler: 3,588,094
  codec_cost: +170 bytes against the same codec written by hand, which is the budget that matters
  api_cost: the 24,629 bytes between the first two rows are the dynamobind helpers, not the codec; a program can call the generated methods directly and skip them
  against_reflection: the generated path is 19,490 bytes smaller than the driver's reflection mapper
  superseded: the earlier 1.45 MB darwin/arm64 baselines, which no build here reproduces; tinygo 0.41.1 cannot link a native binary on this host
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
  caveat: re-measure when the driver version moves; the budget compares paths within one driver build, not across driver releases
related:
  - requirement:dynamobind-product-goals
  - requirement:dynamobind-generated-item-codec
  - decision:dynamobind-static-dispatch
  - decision:dynamobind-json-transport-deferred
  - requirement:tinygo-wasm
  - decision:reflection-free
```
