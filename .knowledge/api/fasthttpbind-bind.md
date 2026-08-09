---
id: api:fasthttpbind-bind
type: api
title: fasthttpbind.Bind
---
Generic request binder that maps a fasthttp RequestCtx into a typed request struct using generated code.

```yaml
signature: "func Bind[T any](ctx *fasthttp.RequestCtx) (T, error)"
example: "input, err := fasthttpbind.Bind[CreateUserRequest](ctx)"
mirrors: api:bind
behavior:
  - bind query, payload, path, header, cookie per the same field tags
  - validate check tags then apply defaults, unchanged from api:bind
  - copy every value out of pooled memory per rule:fasthttpbind-requestctx-lifetime
  - return typed value or error
  - no runtime reflection, per decision:reflection-free
registry: typed marker keyed like the httpbind registry, holding func(*fasthttp.RequestCtx) (T, error)
source_access_mapping:
  query: ctx.QueryArgs, parsed once per request and looked up per field
  payload_json: ctx.PostBody, already a complete buffer, so no reader limit applies
  payload_form: ctx.PostArgs
  payload_multipart: ctx.MultipartForm
  path: supplied by the router, not by the transport; see decision:fasthttpbind-generator-backend-selection
  header: ctx.Request.Header.Peek
  cookie: ctx.Request.Header.Cookie
differences_from_api_bind:
  body_already_read: fasthttp has read the body before the handler runs, so the read-at-most-once bookkeeping of the httpbind binder collapses to a slice reference
  no_reader_limit: policy:json-read-limit describes a bounded reader that no longer exists here; see rule:fasthttpbind-body-limit-mapping
  path_values: no transport-level PathValue, so the router must publish path parameters into the ctx
error_path: api:fasthttpbind-write
related:
  - concept:request-binding
  - concept:code-generation
  - concept:fasthttp-handler
  - decision:fasthttpbind-runtime-package
  - requirement:bind-check-validation
```
