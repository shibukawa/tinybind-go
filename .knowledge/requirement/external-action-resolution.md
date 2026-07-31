---
id: requirement:external-action-resolution
type: requirement
title: External Action Resolution
---
Let a framework supply the URL of a server action a template names from outside the route tree.

```yaml
priority: should
status: implemented 2026-07-30
source:
  - downstream framework integration request 2026-07-30
  - decision:server-action-lowering resolution_dataflow
problem:
  integrated_path: routetree resolution matches a name against the exported handlers of the declaring route package only
  outside_tree: a template compiled outside the route tree names a handler that discovery never sees, so lowering has no address to write
  stated_gap: the missing piece is a mechanism resolving a handler name to a URL, which is what a framework's own route table already holds
already_present:
  option: htmlbind GenerateOptions.ServerActions is a caller-supplied name-to-URL map, and ActionRefs reports what needs resolving
  consequence: a framework compiling its own templates directly can already supply addresses; only the integrated routetree.Generate path cannot
implemented:
  hook: a name resolver on the route-tree generation options, and one on the template compile options it forwards to
  consulted: for a name the declaring package does not export
  precedence: a handler in the declaring package wins, so adding a resolver cannot silently retarget an existing action
  diagnostic: the unresolved-name error stops asserting that the handler must sit beside the template once a resolver is configured, and names both sources it tried
  unchanged: the hash, the prefix, and the lowering of decision:server-action-lowering; only the source of one URL string moves
gain:
  downstream: a mature classic-side template outside the discovered tree can carry a server-action button
  upstream: the two-pass dataflow already treats the resolved map as a compile option, so this exposes an existing seam rather than adding one
acceptance:
  - a template naming a handler outside the route tree lowers to the URL the resolver returned
  - a name the declaring package exports resolves to the generated endpoint even when the resolver would answer it
  - an unresolved name with a resolver configured reports the template position and both attempted sources
  - generation with no resolver behaves exactly as before
related:
  - requirement:template-server-functions
  - decision:server-action-lowering
  - decision:action-lowering-profile
  - decision:framework-integration-seams
```
