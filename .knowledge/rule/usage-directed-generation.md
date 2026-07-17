---
id: rule:usage-directed-generation
type: rule
title: Usage-Directed Mapping Generation
---
Generate each model mapping path only when its configured generic call is present.

```yaml
status: required
mapping:
  Bind: binder plus required JSON/form helpers
  Write: HTTP writer plus required encoder helpers
  WriteStatus: encoder helpers
  DecodeJSON: document decoder helpers
  EncodeJSON: document encoder helpers
  ScanRows: SQL scanner plus nested grouping helpers
closure: nested model helpers inherit only operations required by the parent
registration: only directly used root models register public dispatch entries
imports: derive from emitted paths; JSON-only output must not import net/http
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
```
