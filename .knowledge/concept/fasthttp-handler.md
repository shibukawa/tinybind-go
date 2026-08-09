---
id: concept:fasthttp-handler
type: concept
title: fasthttp Handler
---
Handlers are plain fasthttp RequestHandlers; they bind, call service logic, then Write or WriteError.

```yaml
shape: "func(ctx *fasthttp.RequestCtx)"
counterpart: concept:net-http-handler
pattern: |
  func CreateUserHandler(ctx *fasthttp.RequestCtx) {
      input, err := fasthttpbind.Bind[CreateUserRequest](ctx)
      if err != nil {
          fasthttpbind.WriteError(ctx, err)
          return
      }
      output, err := createUser(ctx, input)
      if err != nil {
          fasthttpbind.WriteError(ctx, err)
          return
      }
      fasthttpbind.Write[CreateUserResponse](ctx, output)
  }
layers:
  handler: concept:fasthttp-handler
  service: concept:service-layer
  bind: api:fasthttpbind-bind
  write_and_error: api:fasthttpbind-write
context_argument:
  fact: RequestCtx satisfies context.Context, so the service call takes ctx directly where the net/http form writes r.Context()
  bound: valid for the handler call only, per rule:fasthttpbind-requestctx-lifetime; a service storing it for later work is a defect
discovery: the same same-package walk as concept:handler-discovery, over a different registration surface; forms mirror concept:handler-forms with RequestHandler in place of HandlerFunc
related:
  - system:tinygodriver-fasthttp
  - decision:fasthttpbind-no-transport-interface
  - decision:transport-neutral-handler
  - concept:handler-forms
```
