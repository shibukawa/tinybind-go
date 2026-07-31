---
id: data:async-render-error
type: data
title: Async Render Error
---
Safe typed failure value exposed to an asynchronous boundary recover clause.

```yaml
source: requirement:async-external-functions
go_shape: exported runtime struct with only the public fields below, so a recover subtree cannot reach a raw Go error
template_type: the built-in `error` type; its fields are readable in a decision:async-boundary-syntax recover subtree
public_fields:
  code: stable application or generated classification
  message: optional presentation-safe text
  retryable: whether UI may offer a retry
  timeout: whether configured async deadline expired
server_only:
  - original Go error or panic value
  - stack and component call chain
  - request and boundary diagnostic context
normalization:
  classifier: an error carrying its own public projection supplies these fields directly
  default: any other error yields a generic internal code and empty message
  panic: recovered as an error, reported to the hook, and exposed as the generic internal code
  timeout: stable timeout code with the timeout field set
  cancellation: expected request cancellation produces no recover value at all
reporting:
  hook: the render error option receives the original Go error, which is where logging and metrics attach
  always: the hook fires even when a recover subtree renders, so a handled failure is still observable
constraints:
  - raw error text is not public by default
  - error value is immutable and usable only inside a recover subtree
  - failure results and recover HTML are not stored by requirement:component-output-cache
```
