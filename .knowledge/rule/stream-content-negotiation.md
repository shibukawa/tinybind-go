---
id: rule:stream-content-negotiation
type: rule
title: Stream Content Negotiation
---
The stream entry selects SSE, NDJSON/JSONL, or JSON array from stream query, Accept, User-Agent, then default.

```yaml
formats:
  sse:
    media_type: text/event-stream
    framing: "data: <json>\\n\\n"
  ndjson:
    media_type: application/x-ndjson
    framing: one JSON object per line
    aliases: [JSONL, application/jsonl, application/ndjson]
    not: a JSON array document
  json_array:
    media_type: application/json
    framing: "[obj1,obj2,...]"
    close: writes trailing ] or empty []
    not: JSONL / NDJSON line stream
priority:
  - stream query parameter
  - Accept header
  - User-Agent
  - Default
stream_query:
  values:
    sse: [sse, event-stream, events, eventstream]
    ndjson: [ndjson, jsonl, nd, lines]
    json_array: [json, array, json-array, jsonarray]
accept:
  text/event-stream: sse
  application/x-ndjson: ndjson
  application/ndjson: ndjson
  application/jsonl: ndjson
  application/json: json_array
user_agent_hints:
  browser_like: sse
  curl_like: ndjson
typical:
  browser: Server-Sent Events
  curl: NDJSON (JSONL-style)
  Accept application/json: JSON array document
curl_example:
  command: "curl -N http://localhost/chat"
  note: works without extra Accept; defaults to NDJSON for curl UA when no Accept match
json_array_example:
  command: "curl -N -H 'Accept: application/json' http://localhost/chat"
  note: single JSON array body; distinct from ?stream=jsonl NDJSON lines
default: ndjson
related:
  - concept:streaming
  - api:new-stream
```
