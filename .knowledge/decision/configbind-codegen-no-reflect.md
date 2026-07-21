---
id: decision:configbind-codegen-no-reflect
type: decision
title: configbind Codegen Without Reflection
---
configbind uses code generation for type-specific bind and CLI code; runtime reflection and runtime tag parsing are forbidden.

```yaml
status: accepted
runtime_forbidden:
  - reflect package for field discovery or assignment
  - runtime parsing of struct tags
  - generic mapstructure-style struct fill via reflection
compile_time:
  - generator reads Bind and SubCommand usage and struct AST
  - generator emits apply functions, CLI parsers, scaffolds, key tables
  - tags default|help|enum|secret|arg resolved at generation time
rationale:
  - TinyGo and small binaries
  - deterministic validation of tags and types
  - same source of truth as CLI and scaffolds
aligns_with:
  - decision:reflection-free
  - requirement:configbind-tinygo
related:
  - decision:configbind-runtime-architecture
  - flow:configbind-codegen
  - concept:config-struct-mapping
  - concept:cli-option-codegen
  - system:configbind
```
