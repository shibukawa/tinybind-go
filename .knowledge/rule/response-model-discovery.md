---
id: rule:response-model-discovery
type: rule
title: Response Model Discovery
---
Response models are discovered from the generic type argument of httpbind.Write[T](w, r, value), including Stream[T].

```yaml
detection_calls:
  - "httpbind.Write[T](w, r, value)"
  - "httpbind.WriteStatus[T](w, r, status, value)"
ordinary_example: "httpbind.Write[CreateUserResponse](w, r, output)"
status_example: "httpbind.WriteStatus[CreateUserResponse](w, r, http.StatusCreated, output)"
streaming_example: |
  httpbind.WriteStream(w, r, func(s *httpbind.Stream[ChatEvent]) error { ... })
  _ = stream.Write(ChatEvent{...})
model_source: generic type argument T
streaming_type: "httpbind.Stream[EventType] via WriteStream[EventType]"
symbol_identity: rule:go-types-symbol-identity
must_be:
  - github.com/shibukawa/tinybind-go.Write
  - github.com/shibukawa/tinybind-go.WriteStatus
reject: same-named Write/WriteStream from other packages
alias_ok: true
openapi_status: rule:openapi-success-status
related:
  - api:write
  - api:write-status
  - api:new-stream
  - concept:response-binding
  - concept:streaming
  - concept:handler-discovery
  - concept:openapi-generation
  - rule:go-types-symbol-identity
  - rule:openapi-success-status
```


