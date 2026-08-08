---
id: policy:problem-details
type: policy
title: RFC 9457 Problem Details Errors
---
Default error format is RFC 9457 Problem Details with field-level locations for validation failures.

```yaml
standard: RFC 9457
default_format: application/problem+json
example:
  type: "..."
  title: Validation failed
  status: 400
  errors:
    - field: email
      location: payload
      message: must be a valid email
error_locations:
  - input
  - payload
  - query
  - path
  - header
  - cookie
  - method
constructors: concept:error-helpers
problem_type: data:problem
write_path: api:write-error
write_error_behavior:
  - resolve HTTP status
  - convert error to RFC 9457 response
  - log wrapped internal cause
  - hide internal implementation details from clients
mappings: rule:standard-error-mapping
openapi_schema: generated automatically for error responses
openapi_media_type: application/problem+json
update_endpoints:
  deviated: htmlupdate wrote plain text through http.Error, so the update paths were the only ones in the module not producing this format
  settled_2026_08_08: they converge here; requirement:update-error-hook default_form carries the decision and what the media type distinguishes
  one_exception: a failure after the response committed cannot carry a status, so requirement:streaming-delta-response keeps its in-band terminal record
related:
  - system:tinybind
  - requirement:update-error-hook
  - concept:response-binding
  - rule:error-cause-preservation
  - concept:openapi-generation
  - rule:openapi-error-statuses
```
