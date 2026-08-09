---
id: api:new-stream
type: api
title: httpbind.NewStream, Removed
---
The held-stream entry point, removed 2026-08-10. It is kept as a record of what the shape was and why it went.

```yaml
status: removed 2026-08-10; api:write-stream is the entry
was: "func NewStream[T any](w http.ResponseWriter, r *http.Request) (*Stream[T], error)"
now: "func WriteStream[T any](w http.ResponseWriter, r *http.Request, fn func(*Stream[T]) error)"
why_removed:
  no_transcription: a stream the handler holds across statements cannot be written on fasthttp, where the body stream writer runs after the handler returned
  deprecating_was_not_enough: an entry that still compiles is a call site with no counterpart on the other backend, so the failure moves from the build to the deploy; deleting it puts the error where it can be fixed
  two_defects_it_allowed: a producer that forgot Close sent an unterminated JSON array document with a 200 on it, and a discarded write error was invisible
  downstream_precedent: the framework on top removed rather than deprecated its own held entry first, and made this argument
migration:
  mechanical: wrap the body in the callback, delete the defer, and return the error instead of discarding it
  behaviour_unchanged: the format negotiation, the framing, the headers and the status all happen where they happened before
what_moved_with_it:
  - the generator call pattern and the parser DefaultConfig entry, both of which named it
  - testdata/stream_newstream, the discovery fixture
related:
  - api:write-stream
  - decision:stream-callback-shape
  - rule:stream-content-negotiation
```
