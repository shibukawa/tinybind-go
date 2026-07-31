---
id: decision:dynamobind-json-transport-deferred
type: decision
title: Removing encoding/json Is Deferred
---
Build dynamobind on the driver attribute map now; recovering the encoding/json and reflect bytes is a separate project that does not change this API.

```yaml
status: accepted as a deferral
measured_tinygo_0_41_1:
  https_and_netdev_without_json: 1,217,856 bytes
  same_plus_encoding_json: 1,369,088 bytes
  cost: 151,232 bytes
  always_linked: the driver marshals its request bodies with encoding/json, so json and reflect link whether or not any reflection-based mapping is used
jsonbind_does_not_recover_it:
  decode: jsonbind.RawJSONMap calls json.Unmarshal into map[string]json.RawMessage
  encode: json.NewEncoder(w).Encode over a map[string]any
  measured: the tinygo-json-smoke binary is 480,848 bytes with 287 encoding/json symbols against 474,256 for the equivalent plain encoding/json program, so 6,592 bytes larger
  what_it_does_remove: reflection over the user structs, which is a different and still worthwhile goal
what_recovery_would_need:
  - a byte-level JSON reader and writer in jsonbind
  - a driver-side seam so an item decodes from raw bytes without an intermediate map[string]AttributeValue
  scope: both are separate projects, and the second one lives in tinygodriver
consequence: api:dynamobind-operations is unchanged by that work, which is the reason to build on the attribute map today rather than wait
related:
  - api:dynamobind-operations
  - requirement:dynamobind-verification
  - system:tinygodriver-dynamodb
  - concept:standalone-json-codec
  - decision:reflection-free
```
