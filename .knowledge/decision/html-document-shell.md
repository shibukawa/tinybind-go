---
id: decision:html-document-shell
type: decision
title: HTML Document Shell
---
Keep the root page template as the `/` page and give full-document bootstrap ownership to a distinct root shell.

```yaml
source:
  - concept:filesystem-html-routing
  - user bootstrap discussion 2026-07-23
review_gate: proposed document convention requires user approval
roles:
  page.tb.html: ordinary page for the exact route-root URL, named by decision:html-route-file-conventions
  layout.tb.html: reusable UI wrapper inside the document body
  document.tb.html: optional route-root-only owner of html, head, body, and generated bootstrap insertion points
fallback: generator supplies a minimal safe document shell when document.tb.html is absent
composition:
  - document body interior is an unnamed requirement:html-slot-syntax slot; a handler-invoked template renders only that interior
  - requirement:cross-template-components supplies the fill when the chain is not statically discovered
  - document wraps outermost layout and page result for complete HTML responses
  - partial navigation retains the existing document shell and updates layout or page boundaries
  - document is not an api:client-component-update target
injection:
  head: requirement:head-merging merged contributions, generated metadata, optional data:html-client-bootstrap metadata, and application-provided head nodes
  body_end: requirement:html-runtime-bootstrap script when route capabilities require it
  validation:
    - exactly one html, head, and body in an explicit document shell
    - exactly one generated bootstrap per complete document
    - injection uses parsed HTML positions rather than textual closing-tag replacement
rationale:
  - every route needs the shell, while the root page represents only one URL
  - separating roles avoids bootstrap duplication and keeps root-page behavior ordinary
```
