---
id: requirement:decode-json-bytes-discovery
type: requirement
title: Discover DecodeJSONBytes And DecodeJSONLimit Call Sites
---
Register jsonbind.DecodeJSONBytes and jsonbind.DecodeJSONLimit as decode discovery patterns, so a package that reads through either gets its decoder generated instead of failing at runtime with a missing codec.

```yaml
priority: must
source:
  - system:odjson harness 2026-09-12: a package whose only decode calls were DecodeJSONBytes[T] came out with encoders and no decoder
review_gate: proposed
status: implemented 2026-09-12; generator/options.go lists both entries as JSONDecodeCall, and generator/any_field_test.go asserts a package calling only either one gets its decoder
problem:
  where: generator/options.go canonicalRuntimeCalls lists JSONDecodeCall for DecodeJSON only; DecodeJSONBytes and DecodeJSONLimit are public entry points of api:decode-json with the same type parameter and are not listed
  effect: the call compiles, the runtime looks the type up in the registry, finds nothing, and returns a missing_codec jsonbind.Error on the first request
  why_it_hid: every test fixture in the repo reads through DecodeJSON or a GenerateCodec annotation, and requirement:declared-json-codec was the workaround the harness used
fix:
  patterns: 'JSONDecodeCall(Function(path, "DecodeJSONBytes"), GenericType("decode", 0)) and the same for DecodeJSONLimit, beside the DecodeJSON line'
  no_new_operation: both are the decode direction of the same codec, so the usage bit and the emitted code are unchanged
  test: a generator test whose fixture calls only DecodeJSONBytes and asserts the decoder is emitted; the fixture in generator/generate_test.go already covers the DecodeJSON spelling
rule_going_forward: any public generic entry point jsonbind or httpbind adds is a discovery pattern on the day it is added, or it is documented as annotation-only; data:generator-options is where the list is described
related:
  - api:decode-json
  - requirement:declared-json-codec
  - data:generator-options
```
