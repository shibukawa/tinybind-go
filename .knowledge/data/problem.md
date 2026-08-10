---
id: data:problem
type: data
title: Problem
---
Application error payload carried by status helpers; includes machine code and human message.

```yaml
fields:
  - name: Code
    type: string
    purpose: machine-readable error code
  - name: Message
    type: string
    purpose: human-readable message
example: |
  Problem{
      Code:    "invalid_email",
      Message: "email is invalid",
  }
used_by:
  - concept:error-helpers
  - policy:problem-details
declared_in: internal/bindcore, aliased by both runtime surfaces, so the transports carry one type rather than two that agree
a_framework_declares_its_own:
  reported: downstream framework survey 2026-08-10, explicitly not as an ask
  fact: this shape is the body the constructors take; a framework on top needs status, title, fields and cause, so it declares a Problem of its own and there are two types under one name
  reporter_resolution: theirs lives in a leaf both their runtimes alias, which is the move made here for FieldError and then for the update types of decision:update-core-shared-leaf
  why_recorded: the next framework built on this module meets the same fork, and the answer is a shared leaf on its side rather than a wider type on this one
related:
  - api:write-error
```
