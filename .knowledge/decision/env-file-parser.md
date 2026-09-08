---
id: decision:env-file-parser
type: decision
title: Env File Parser
---
Dotenv files are parsed with hashicorp/go-envparse v0.1.0, the only measured library that builds under TinyGo wasm without regexp or os/exec at near hand-written size.

```yaml
status: accepted
chosen: github.com/hashicorp/go-envparse v0.1.0
entry: 'envparse.Parse(io.Reader) (map[string]string, error)'
license: MPL-2.0
imports: bufio, bytes, fmt, unicode/utf8, unicode/utf16
measured: TinyGo 0.42.0 and Go 1.27 on 2026-09-08, native, -target wasm, -target wasip1
sizes_wasm:
  - hand-written baseline 722 KB
  - go-envparse 744 KB
  - joho/godotenv v1.5.1 1166 KB
  - subosito/gotenv v1.6.0 1197 KB
alternatives_rejected:
  - id: joho/godotenv
    why_not: regexp and os/exec, and it expands ${VAR} itself, which would race the file layer of requirement:config-env-interpolation
  - id: subosito/gotenv
    why_not: regexp and x/text
  - id: hand-written
    why_not: saves 22 KB against owning a grammar and its edge cases
constraint: requirement:configbind-tinygo; the consumers build for Cloudflare Workers and TinyGo containers
error_surface:
  type: '*envparse.ParseError carrying Line'
  message: 'configbind: read env file %q line %d: %v'
grammar:
  - export prefix accepted
  - '# comments'
  - unquoted, single-quoted, and double-quoted text may mix in one value
  - JSON escapes inside double quotes
  - later duplicate wins within one file
  - no ${NAME} expansion; the TOML layer still expands from the composed environment per rule:env-file-composition
  - no YAML forms
related:
  - requirement:env-file-input
  - api:configbind-env-files
  - decision:configbind-runtime-architecture
  - concept:reusable-source-parsers
  - system:configbind
```
