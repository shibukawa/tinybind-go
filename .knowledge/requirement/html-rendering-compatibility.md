---
id: requirement:html-rendering-compatibility
type: requirement
title: HTML Rendering Compatibility
---
Add runtime extensions without changing the observable default behavior of existing HTML templates.

```yaml
source: concept:html-render-runtime-extensions
baseline: requirement:html-template-v1
must_preserve:
  - existing template syntax unless an extension is explicitly used
  - the requirement:html-component-api signature shape for unchanged templates
  - direct component composition and children semantics
  - streaming writes, context-aware escaping, and trusted output restrictions
  - rendered body bytes for unchanged templates
  - generation-time parsing and type checking with no runtime template parsing
activation:
  nested_layout: only for an explicit or convention-generated data:html-render-route-plan
  filesystem_routing: only below an explicitly configured route root
  cache: only on explicitly cache-enabled components
  async: only on explicitly async external declarations inside a suspense boundary
  partial_update: only on explicitly update-enabled component boundaries and update requests
acceptance:
  - unchanged templates generate compatible public APIs and equivalent bytes
  - response headers, negotiation, and compression stay outside generated code
  - extension runtime failures do not silently fall back to unsafe raw output
  - ordinary navigation and clients without update runtime receive complete HTML
  - existing flat direct-package template discovery does not recurse or reinterpret reserved route filenames
```
