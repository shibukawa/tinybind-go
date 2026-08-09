---
id: requirement:transform-diagnostics
type: requirement
title: Transform Refusal Diagnostics
---
A refused function stops the fasthttp build, so the diagnostic must name the occurrence that caused it and the chain from the handler that inherited it.

```yaml
status: proposed 2026-08-08
why_this_matters_more_than_usual:
  fact: decision:backend-build-tag-mode removed the fallback, so a refusal is fatal rather than a slower route
  consequence: the diagnostic is the entire user experience of the transform; a refusal a developer cannot act on is indistinguishable from the feature not working
required_content:
  position: file and line of the occurrence, not of the handler
  classification: which entry of the refused list in rule:transform-eligibility it matched, named in the message
  chain: when a handler is refused because a callee is, every hop from handler to occurrence, in order
  remedy: the applicable author remedy, since the classification usually determines it
chain_example: |
  createUserHandler is not transformable
    handlers.go:31  calls renderError(w, r, err)
    render.go:12    renderError passes w to log.Attach, a function outside this package
    remedy: move the logging call behind a function taking neither w nor r,
            or register log.Attach as a framework call pattern with its transport slots
batching:
  required: report every refusal in the package in one run
  reason: refusals cluster on shared helpers, so fixing one commonly clears several, and one-at-a-time reporting turns that into as many build cycles
non_fatal_mode:
  offer: a report-only run that lists what a fasthttp build would refuse, without generating
  why: adoption is all-or-nothing per decision:backend-build-tag-mode, so an application needs to see the whole cost before committing to the migration rather than after
  shape: the same diagnostics, exit zero, no files written
acceptance:
  - a refusal names an occurrence position rather than only a declaration
  - an inherited refusal prints its chain to the original occurrence
  - one run reports every refusal in the package
  - the report-only run on a fully transformable package reports nothing and writes nothing
related:
  - rule:transform-eligibility
  - decision:transport-source-transform
  - requirement:analysis-diagnostics
  - rule:analysis-diagnostics-check
```
