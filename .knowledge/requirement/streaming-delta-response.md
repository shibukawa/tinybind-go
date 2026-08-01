---
id: requirement:streaming-delta-response
type: requirement
title: Streaming Delta Response
---
Emit delta operations as an ordered record stream so each boundary applies as soon as it is rendered and compared.

```yaml
source:
  - requirement:component-delta-rendering
  - user async-sequence discussion 2026-07-26
review_gate: proposed protocol surface requires user approval
status:
  delivered: the record framing, per-record manifest entries, the terminator, incremental application, and truncation handling
  pending: the asynchronous producer, because requirement:suspense-html-streaming is not in this branch
  independence: the transport does not depend on the producer; a synchronous delta drives it today and an async sequence replaces the source later
  seam: an open stream exposes writing one settled boundary, restating an unchanged one, reporting a late failure in band, and terminating; an async producer ranges its sequence and calls the first of those per completion
  framing: one JSON record per line, matching the module's existing streamed-value format
motivation: a delta computed from an async chain finishes boundary by boundary; buffering the whole response would discard the requirement:suspense-html-streaming benefit
driver: the merged decision:async-component-signature sequence of requirement:chain-render-pipeline, extended to emit delta records instead of document bytes
records:
  head: served mode, server protocol version, and document-level directives; written first and commits the response
  head_sync: requirement:delta-head-sync operations, before any content record that depends on them
  operation: one data:component-delta-response operation plus the data:component-update-manifest entry it produces
  async_completion: an await boundary inside a delta fragment yields the same record shape, so one client runtime consumes both
  directive: navigate, reload, or in-band error after commit
  end: explicit terminator meaning the server finished the requested render
incremental_manifest:
  rule: no trailing whole-document manifest; each record carries its own manifest entry and the client merges
  reason: a trailing manifest cannot be written before the operations it describes
  absence: an instance never mentioned keeps its previous validator, which is why the terminator is mandatory
  truncation: a stream ending without the terminator leaves manifest state unknown; mark it stale and force a complete document on the next request
ordering:
  structural_first: an insert or replace of an ancestor precedes operations addressing its descendants
  independent: unrelated boundaries emit in completion order
  per_instance: at most one content operation per instance per response, excluding that instance's own async boundary completions
  fence: every record carries the instance revision it produces, per rule:delta-consistency-model
commit_consequences:
  before_head: validation, authorization, and requirement:render-mode-negotiation downgrade are still possible
  after_head: status is fixed; failure becomes an in-band error record and the client decides between partial state and full navigation
  partial_apply: applied operations are not rolled back; the client marks the manifest stale
transport:
  framing: length-delimited records over one chunked response
  flushing: the api:render-html-chain writer Flush assertion, after the head record and after each subsequent record
  compression: flush the encoder per record, so a compressing writer cannot withhold a completed boundary
  fragments: HTML stays a safe fragment per rule:template-context-safety; operation metadata is never interpolated into script source
cancellation:
  client: aborting the request stops server-side boundary work
  superseded: a newer navigation or boundary revision cancels the older stream and its records are discarded unapplied
  no_recover_noise: an expected cancellation emits no error record
limits:
  - configured maximum records and bytes per response
  - configured per-request boundary concurrency shared with requirement:chain-render-pipeline
acceptance:
  - a slow boundary does not delay applying an already-rendered changed boundary
  - a boundary that finishes after an error record still applies or is discarded by one documented rule
  - a client disconnect mid-stream stops remaining render work
  - an interrupted stream never leaves the client believing unmentioned boundaries are current
resolved:
  framing: newline-delimited JSON records
  unchanged_boundaries: reported explicitly with a validator and no markup, so the client can rebuild its whole manifest from what it received
open_questions:
  - whether the framing is shared with the requirement:suspense-html-streaming update records once those exist
  - back-pressure policy when the client applies slower than the server renders
  - whether boundary-mode responses need streaming in the first milestone
```
