# YAML Structures

Use YAML fences for structured content that is hard to maintain as prose.

## Flow

```yaml
flow:
  trigger: actor:requester submits api:access-check
  steps:
    - id: receive
      actor: system:access-service
      action: validate request data:access-check-request
    - id: evaluate
      action: apply policy:access-check
    - id: respond
      output: data:access-decision
  failure:
    default: fail_closed
```

## DFD-Like Structure

No finished DFD editor is assumed. Represent data flow as compact YAML until a tool exists.

```yaml
dfd:
  boundary: system:access-service
  actors:
    - actor:requester
  stores:
    - data:user-affiliation-record
  flows:
    - from: actor:requester
      to: api:access-check
      data: data:access-check-request
    - from: api:access-check
      to: data:user-affiliation-record
      purpose: verify affiliation
    - from: api:access-check
      to: actor:requester
      data: data:access-decision
```

## UI Sketch

No finished UI sketch editor is assumed. In `ui` concepts, represent the current UI structure as YAML. Keep it semantic rather than pixel-perfect.

```yaml
ui:
  root:
    kind: browser
    id: screen.access-review
    title: Access Review
    children:
      - kind: table
        id: access-requests
        columns:
          - requester
          - affiliation
          - decision
      - kind: button
        label: Recheck
        action: api:access-check
```

Recommended component keys:

- `kind`
- `id`
- `label`
- `title`
- `children`
- `columns`
- `action`
- `target`
- `state`

Use `ui` YAML blocks to preserve intent for future editors. Do not depend on a renderer.
