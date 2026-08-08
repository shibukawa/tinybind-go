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
  head_operations: requirement:delta-head-sync contributions, ordered before dependent content operations
  operations:
    - kind: replace, insert, remove, move, or replace_with_retained
      instance_id: rule:component-instance-identity
      anchor_or_parent: optional structural target
      content_validator: omitted for removals
      html_template: safe HTML fragment for insertions and replacements
      retained: descendant instance IDs whose existing DOM the client moves into holes in html_template
  directives: navigate, reload, or in-band error once the response has committed
boundary_list:
  shipped: 2026-08-08 as the boundaries field, on the buffered body and on the stream record alike
  added: 2026-08-08 by requirement:boundary-decomposed-render
  what: one record per fragment naming the reloadable boundary ids inside it, declared for that fragment's scope
  why: a hole with no fragment in the response has to read as a retain rather than as a truncation, and nothing else in the body distinguishes them
  and: it separates the structure from the selection, so a caller may omit fragments without the response becoming ambiguous
  static_marker: an entry may report that its boundary's subtree is entirely static, which is settled at generation time and keyed by the build identity rather than by a validator
retain_holes:
  now_the_ordinary_shape: 2026-08-08; a decomposed response carries placeholders at every nested boundary rather than only where an optimization applies
  purpose: replace a changed ancestor without resending or recreating unchanged descendant boundaries
  mechanism: the fragment carries an empty placeholder per retained instance; the client moves the live node in
  benefit: preserves rule:preserved-client-subtree state that wholesale ancestor replacement would destroy
  fallback: a server that cannot express holes sends the full ancestor subtree
transport:
  streamed: requirement:streaming-delta-response emits these fields as records, each carrying its own manifest entry
  buffered: a non-streaming response may carry one whole next_manifest instead
head:
  carried: the merged tag list of the new composition, per requirement:delta-head-sync
  was_missing: recorded as a gap on main until this response gained the field
behavior:
  unchanged: no HTML operation; carry its validator in next_manifest
  incompatible_version: instruct full navigation or return complete HTML
  unsupported_structure: replace nearest safe ancestor or fall back to complete HTML
safety:
  - HTML fragments preserve rule:template-context-safety
  - operation metadata is encoded separately from script source
  - client applies operations through fixed trusted update runtime
  - retained IDs are matched against live DOM state; an unknown ID falls back to rendering that region absent
```
