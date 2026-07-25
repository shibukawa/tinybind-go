---
id: policy:frontend-convention-alignment
type: policy
title: Frontend Convention Alignment
---
Resolve open authoring-convention choices toward the React ecosystem, because its user base is the largest.

```yaml
source: user direction 2026-07-25
rule:
  prefer: React ecosystem convention, including CSS Modules for styling semantics
  conflict: when React and Svelte or Vue disagree, React wins
  note: React ships no scoped-style feature itself, so CSS Modules is the concrete reference for styling
scope:
  applies_to: open questions about authoring syntax, naming, and styling semantics
  not_retroactive: decisions already recorded stand; this policy steers what is still undecided
override:
  allowed_when: a project constraint makes the React convention unimplementable
  requires: recording the divergence and its reason in the affected concept
examples:
  styling: requirement:scoped-component-style follows CSS Modules local-name and global-escape behavior
  known_divergence:
    fill_blocks: requirement:html-slot-syntax fill blocks resemble single-file component syntax because markup, not JSX, is the authoring surface
    style_authoring: decision:component-style-delivery keeps styles inline in the template file for the same reason; there is no JS module to import a stylesheet into
```
