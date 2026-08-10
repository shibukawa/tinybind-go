---
id: rule:usage-directed-generation
type: rule
title: Usage-Directed Mapping Generation
---
Generate each model mapping path only when its configured generic call is present.

```yaml
mapping:
  Bind: binder plus required JSON/form helpers
  Write: HTTP writer plus required encoder helpers
  WriteStatus: encoder helpers
  DecodeJSON: document decoder helpers
  EncodeJSON: document encoder helpers
  ScanRows: SQL scanner plus nested grouping helpers
  Store / StoreAll: requirement:dynamobind-generated-item-codec EncodeItem plus nested item encoders
  Load / LoadAll / Query / QueryPage / Scan: DecodeItem plus nested item decoders
  Remove / Update: ItemKey
  StoreReturning: EncodeItem plus DecodeItem
  RemoveReturning: ItemKey plus DecodeItem
  item_key_exception: a type with a partitionkey tag gets ItemKey and its table definition without a discovered call, per requirement:dynamobind-generated-item-codec
  both_client_forms: every store name above stands for its Context entry and for the Handle-taking On twin alike, per requirement:parameter-api-call-discovery; the type is still discovered from a call, and only the argument holding it moves
closure: nested model helpers inherit only operations required by the parent
registration: only directly used root models register public dispatch entries
imports: derive from emitted paths; JSON-only output must not import net/http
runtime_imports:
  DecodeJSON / EncodeJSON only: jsonbind
  Bind / Write: httpbind
  ScanRows only: sqlbind
  DynamoDB item operations only: the driver, for dynamodb.Item and dynamodb.Key; dynamobind is imported by the call site, not by the emitted methods
boundary: decision:runtime-package-boundaries
unused_models: emit no mapping functions
compatibility: explicit generate-all option may emit every supported path
discovery: requirement:configurable-generator-discovery
disablement: rule:generator-feature-disable
related:
  - concept:code-generation
  - flow:code-generation
  - concept:standalone-json-codec
  - api:scan-rows
  - rule:generator-feature-disable
  - decision:runtime-package-boundaries
  - requirement:dynamobind-generated-item-codec
```
