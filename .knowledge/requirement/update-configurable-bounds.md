---
id: requirement:update-configurable-bounds
type: requirement
title: Update Endpoint Configurable Bounds
---
Give every bound and protocol constant of the update endpoints the same form, so a deployment tunes them all one way instead of one per mechanism.

```yaml
priority: could
source:
  - downstream framework runtime ownership report 2026-08-01, against v0.3.0
review_gate: proposed
shipped_today:
  manifest_size:
    form: Options.MaxManifestBytes field with DefaultMaxManifestBytes
    configurable: yes
  redraw_query_size:
    form: 'const MaxQueryBytes = 4 << 10 in redraw.go'
    configurable: no
  stream_content_type:
    form: const StreamContentType in stream.go
    configurable: no
reading:
  same_kind_of_knob: all three are deployment-facing values a proxy, a CDN, or a policy may force
  different_shapes: one is an Options field with a default, two are package constants
  cost: low; the reporter files it as small and independent, not as a blocker
  why_it_is_worth_stating: requirement:component-redraw-endpoint already names 'a configured maximum query length' as a constraint, so the query bound is a field the design asked for and the code shipped as a constant
ask: the same treatment for all three, following the MaxManifestBytes shape
constraints:
  - a project configuring nothing behaves exactly as today, so each default equals the current constant
  - the stream content type stays protocol surface, so overriding it is a framing choice rather than a tuning knob and may deserve a narrower seam
  - an oversized manifest keeps being dropped rather than rejected, and an oversized query keeps being rejected, per requirement:render-mode-negotiation and requirement:component-redraw-endpoint
acceptance:
  - a deployment behind a proxy with a short URL limit lowers the redraw query bound without a fork
  - the exported constants remain readable, so a caller can still name the default
as_built:
  shipped: 2026-08-01
  fields: Options.MaxQueryBytes and Options.StreamContentType, matching MaxManifestBytes
  renamed_constants: MaxQueryBytes became DefaultMaxQueryBytes and StreamContentType became DefaultStreamContentType, so the default keeps a readable name beside the field that overrides it
  breaking: both renames break a caller naming the old constant, which is the cost of the consistency this asked for
  open_question_resolved: the stream content type got a field like the other two, with its doc comment stating that it names the wire format rather than a limit, so overriding it is a framing choice a client has to agree with
  validation: Options.Validate rejects a negative bound
related:
  - requirement:component-redraw-endpoint
  - requirement:streaming-delta-response
  - requirement:render-mode-negotiation
open_questions:
  - whether a client has any way to learn an overridden stream content type, since only the server side is configured
```
