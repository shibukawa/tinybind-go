---
id: requirement:template-file-scope
type: requirement
title: Template File Scope
---
Treat one template file as the scope unit; export publishes a declaration and external imports one.

```yaml
source:
  - requirement:template-language-core
  - user scope decision 2026-07-25
unit: one template file
visibility:
  private: a declaration without export is reachable only inside its file
  exported: the export modifier from decision:template-declaration-kinds publishes the declaration to other files and to generated Go
  single_axis: file visibility and generated Go symbol visibility are the same decision, not two modifiers
import:
  keyword: external, reusing the requirement:template-language-core external function form
  kinds: components and functions
  model: the importing file restates the contract; the generator binds it to the resolved target
  rationale: an explicit local contract keeps every call site type-checked without cross-file parsing order rules
verification:
  - a resolved target must match the external declaration in parameter names, types, slots, and output type
  - a mismatch is a generation error naming both the external declaration and the resolved target
  - an external declaration with no resolvable target is a generation error
  - importing a non-exported declaration is a generation error
naming:
  - external local names are file-scoped, so two files may import different components under the same local name
  - diagnostics show the file-relative path and the local name
constraints:
  - resolution happens at generation time; no runtime lookup, registry, or reflection
  - a file exports nothing implicitly
  - cycles among template files are a generation error
acceptance:
  - a component is usable from another file only after it is exported and externally declared
  - renaming a private declaration cannot break another file
  - a signature change in an exported component fails every importing file with an actionable diagnostic
open_questions:
  - whether external declarations name a path, a module, or a configured generation unit
  - relationship to the existing optional package and imports declarations in requirement:template-language-core
  - whether a file may re-export an imported declaration
```
