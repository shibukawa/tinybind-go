---
id: rule:call-operation-usage
type: rule
title: Call Operation Usage Names the Registry Its Runtime Reads
---
A call operation's usage must name every registry the runtime entry point actually looks in, because emitting a codec body is not registering it.

```yaml
status: implemented 2026-08-09
two_registries:
  writer: httpbind.RegisterWrite and fasthttpbind.RegisterWrite; read by api:write through lookupWriter
  encoder: jsonbind.RegisterEncode; read by jsonbind.EncodeJSON
  why_two: a writer owns its status and its headers, an encoder owns bytes alone; api:write-status supplies the status itself and so reaches past the writer
mapping:
  request_bind: UsageBind
  response_write: UsageWrite
  response_write_status: UsageWrite | UsageEncodeJSON
  stream_create: UsageWrite | UsageEncodeJSON
  json_encode: UsageEncodeJSON
  json_decode: UsageDecodeJSON
  rows_scan: UsageScanRows
why_write_status_carries_both:
  writer: a generated writer is still emitted and registered, so api:write on the same type keeps working
  encoder: WriteStatus serializes through jsonbind.EncodeJSON, which reads the encoder registry and nothing else
the_trap: |
  emitEncode runs for a written type, so encode<T> appears in the generated file
  whether or not the operation asked for it. Only the init entry is missing, and
  a reader scanning for the function finds it. The failure surfaces at runtime as
  missing_codec, at a call site that names no registry.
transport_independence:
  keyed_on: the operation, never the transport
  emitted_text: one registration block serves both backends; the fasthttp mapping imports fasthttpbind under the httpbind qualifier, and jsonbind is shared unchanged
  consequence: a usage entry that is right for net/http is right for fasthttp, and one that is wrong is wrong on both
verification:
  read_the_init: a test must assert the generated init, not only the OpenAPI document; the document is derived from the field plan and is correct while the registration is absent
  history: rule:openapi-success-status held for api:write-status while its encoder went unregistered, which is exactly this gap
related:
  - data:generator-call-pattern
  - api:write
  - api:write-status
  - api:fasthttpbind-write
  - concept:code-generation
  - rule:openapi-success-status
```
