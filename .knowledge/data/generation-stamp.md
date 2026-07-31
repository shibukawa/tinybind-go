---
id: data:generation-stamp
type: data
title: Generation Stamp Comment
---
Record of one generation run, written as a comment into every file that run produced.

```yaml
location: line directly below the generated-code header, above the package clause
format: "// tinybind:generated v<version> inputs=sha256:<hex> self=sha256:<hex> outputs=<name>,<name>"
fields:
  version: stamp encoding version; an unrecognized version reads as no stamp
  inputs: hash of everything the recording run read, per rule:generation-input-hash
  self: hash of the containing file with the stamp line removed
  outputs: base names of every file the run wrote, in artifact order
properties:
  - the generated-code header keeps the first line, so Go tooling still sees generated source
  - written by the producing run; never hand-maintained
  - identical inputs and outputs values across the files of one run
  - excluded from its own inputs, so a run does not invalidate itself
  - absent from api:generator-artifacts output, which writes no file
consumers:
  - api:generator-execution skip decision
  - reviewers identifying which run produced an artifact
related:
  - rule:generation-input-hash
  - requirement:incremental-generation
  - data:generation-artifact
```
