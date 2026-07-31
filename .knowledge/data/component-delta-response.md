---
id: data:component-delta-response
type: data
title: Component Delta Response
---
Return only changed update-boundary payloads plus state needed for the next request.

```yaml
source: requirement:component-delta-rendering
fields:
  render_version: server version for compatibility checks
  next_manifest: data:component-update-manifest
  operations:
    - kind: replace, insert, remove, or move
      instance_id: rule:component-instance-identity
      anchor_or_parent: optional structural target
      content_validator: omitted for removals
      html_template: safe HTML fragment for insertions and replacements
head:
  missing: no field currently carries the document head change, so a navigation cannot express it
  proposed: requirement:client-managed-head supplies the merged tag list; this response either gains a field for it or the framework carries it alongside
behavior:
  unchanged: no HTML operation; carry its validator in next_manifest
  incompatible_version: instruct full navigation or return complete HTML
  unsupported_structure: replace nearest safe ancestor or fall back to complete HTML
safety:
  - HTML fragments preserve rule:template-context-safety
  - operation metadata is encoded separately from script source
  - client applies operations through fixed trusted update runtime
```
