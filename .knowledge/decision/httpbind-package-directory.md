---
id: decision:httpbind-package-directory
type: decision
title: The HTTP Runtime Lives In httpbind/, Not At The Module Root
---
Move package httpbind from the module root to github.com/shibukawa/tinybind-go/httpbind for v0.6, so the import path's last element is the package name and the module root holds no runtime.

```yaml
status: implemented 2026-10-25, for v0.6
was: package httpbind at github.com/shibukawa/tinybind-go, imported as httpbind "github.com/shibukawa/tinybind-go" because the path's last element was the module name
now: github.com/shibukawa/tinybind-go/httpbind, imported without an alias like every sibling runtime
why:
  - every other runtime is a directory named for its package; the root was the one exception, and the exception cost an alias at every import and a declared package name wherever a path had to be resolved, which is why routetree.ActionDeclaration carries Package beside Import
  - a root package makes the whole module read as the net/http runtime, when jsonbind, tomlbind and fasthttpbind are peers that must never pull it in per decision:runtime-package-boundaries
compatibility: breaking at the import line only; a caller replaces the path and drops the alias, and the package name, every declaration and the qualifier generated code spells are unchanged
touched:
  - generator: httpbindImportPath, which the OpenAPI fragment emitter now also spells its import from, and the transform's net/http-to-fasthttp swap keyed on it
  - parser and routetree: the path constants discovery resolves calls and the ServerAction annotation against
  - fixtures, testdata, examples, docs and this catalog: the import line and the package identity it names
  - scripts/tinygo-check.sh: tests ./httpbind where it tested .
not_changed:
  - the module path, so go.mod, go get and the replace directives test modules write stay the same
  - cmd/tinybind-gen, fasthttpbind and the other runtime paths
related:
  - decision:runtime-package-boundaries
  - decision:fasthttpbind-runtime-package
  - system:tinybind
```
