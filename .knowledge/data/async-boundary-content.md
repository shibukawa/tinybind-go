---
id: data:async-boundary-content
type: data
title: Async Boundary Content
---
One resolved boundary result yielded by an async generated component after its initial document write.

```yaml
source:
  - requirement:suspense-html-streaming
  - user signature decision 2026-07-25
go_shape: struct with the generated boundary ID and its rendered replacement HTML
fields:
  id: generated boundary identifier matching the placeholder written during the initial pass
  html: context-checked rendered fragment for that boundary
carries:
  success: resolved decision:async-boundary-syntax await subtree output
  failure: recover subtree output rendered from data:async-render-error
excluded:
  - initial document bytes; those are written to the io.Writer argument
  - raw Go error values and diagnostics; error travels as the iterator error value
  - transport framing; the caller wraps id and html into template element and update record
gap:
  liveness: the record is the same shape for a settled await boundary and for one delivery of a live boundary, so a live-mode consumer cannot tell them apart; requirement:live-boundary-liveness-signal proposes the field
constraints:
  - id is unique per request, opaque, and safe in HTML and script contexts
  - html is already escaped and context-validated; consumers must not re-escape it
  - value is valid only until the next iteration step; consumers copy what they retain
  - no item is yielded for expected cancellation or superseded boundary revision
```
