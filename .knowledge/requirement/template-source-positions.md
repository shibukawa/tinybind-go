---
id: requirement:template-source-positions
type: requirement
title: Template Source Positions In Generated Go
---
Report a compile-time diagnostic in generated Go at the template line that produced it, so an author reads the error against the file they wrote.

```yaml
priority: should
status: implemented 2026-08-19, opt-in, in sqlbind, htmlbind, dynamobind, and firestorebind
source:
  - downstream Popcorn Wave request 2026-08-19
  - upstream mechanism verification 2026-08-19
problem:
  ownership: a generated file is output, so the downstream keeps it out of version control; an error naming it names a file the author never wrote and cannot bookmark
  today: a type error in a template expression reports a data:generation-artifact line, and the author opens the generated file to map it back by hand
  leverage: the Go compiler, go vet, delve, gopls, and editors all honor a Go line directive; none is a tool tinybind owns, and one emitter change moves all of them at once
scope:
  covers: compile-time position, plus the runtime stack frame wherever generated code is a real function
  excludes: HTML render-time error position, which requirement:render-error-positions covers separately because htmlbind's generated shape puts that failure outside generated code
emission_sites:
  sqlbind:
    shape: a real function body; emitStatement writes 'func _tinybindBuild<Name>' and emitNodes walks body nodes into Go statements
    position_source: every body node struct carries a Pos field of the shared line and column pair
    accessor_gap: the Node interface declares only NodeType, so a per-node position needs a new interface method or an emitter type switch; the field alone is not reachable from the walk
    resolved_as: an unexported nodePos type switch over the six body node types, rather than a method on the exported Node alias, which an out-of-module format parser also satisfies
    reach: compile error and runtime stack frame both land on the template, because the generated frame is the template's own function
    verified: TestPanicInAStatementNamesTheTemplate reads a real stack out of a compiled fixture
  htmlbind:
    shape: a decision:generated-render-plan literal plus a one-line binder; the Ops slice is emitted one element per line, so a per-element directive is mechanically possible
    reach: compile error only; a render-time frame sits in the shared coordinator, not in generated code
    coalesced_static: a Static op merges adjacent runs across template constructs after requirement:static-whitespace-normalization, so it owns a span rather than a point, and its start is the only honest answer
    nested_lists: a For body and an If branch are bare Op slices over their own scope type with their own element lines, so each needs its own directives and its own restore
    verified: TestLineDirectivesReportTheTemplateLine puts the failing expression inside a loop body, which is the position a top-level-only mapping would have missed
  dynamobind:
    shape: QueryDecl carries SourcePath and Line and no column, so a directive there is line-only; requirement:dynamo-typed-queries declarations gain nothing from column support
    span: the whole generated function, opened above it and closed after it; the table and key-condition constants above are named by the emitter rather than by the declaration and keep their own position
  firestorebind:
    shape: identical to dynamobind, including the transactional twin, which is mapped to the same declaration
    included: the request named three dialects; the fourth has the same declaration shape and the same emitter, so leaving it out would have made the gap the harder thing to explain
path_form:
  choice: absolute
  why: the Go toolchain shortens an absolute path against the working directory before printing it, so one string reads correctly from anywhere and both readers agree on it; go build prints 'store/users.tb.sql' from the module root and './users.tb.sql' from inside the package, and go vet prints the same
  why_not_module_root_relative:
    proposed: the downstream asked for a path relative to the module root
    measured: go vet resolves a relative directive path a second time, against the directory of the file holding the directive, so it reports 'store/store/users.tb.sql'
  why_not_the_base_name:
    tried: shipped first, on the reasoning that a template and the file generated from it share a directory, so go vet resolves the bare name correctly
    measured: go vet does resolve it, and go build does not; go build prints the directive string verbatim, so the message names 'users.tb.sql' with no directory, which does not open from the module root and is ambiguous the moment two packages hold a template of the same name
    settled: 2026-08-19, on downstream review; absolute is the only form both readers print correctly
  trimpath:
    original_premise: the downstream request argued against absolute on the grounds that a directive path is a literal the compiler adopts verbatim, unrelated to the recorded compile path -trimpath rewrites, so an absolute one would leave a machine-specific string in the artifact
    measured: false for the binary; a panic in a mapped function reports 't/store/users.tb.sql' under -trimpath and the absolute path without it, exactly as -trimpath treats any other source path
    consequence: a release build carries no machine-specific string it would not have carried anyway, so the objection reaches generated source and nothing else
  machine_dependence:
    reaches: the generated Go bytes, which now differ between checkouts at different paths
    does_not_reach: a -trimpath binary, per above; the rule:generation-input-hash inputs fingerprint, which hashes sources, options, and a module-relative package path and no output
    who_it_matters_to: a project that commits generated files, where the committed absolute paths are wrong on every other machine and the data:generation-stamp inputs hash still reports up to date, so nothing forces the regeneration that would fix them
    who_it_does_not: the requesting downstream, which keeps generated Go out of version control under its own generated-artifacts policy, so every machine generates its own
scaffolding:
  rule: only a template-derived span is mapped; a line the emitter wrote for itself keeps its own position
  why: a directive keeps applying until replaced, so an unrestored span reports generator scaffolding at template positions and makes an emitter bug unfindable
  mechanics: rule:line-directive-emission
output_stability:
  one_time_churn: every generated file carrying a template gains directive text, so the first run after this ships rewrites all of them and changes every data:generation-stamp self digest
  precedent: requirement:head-contribution-provenance kept byte-identical output by emitting only beside a non-empty field; no equivalent exists here, because a template-derived span is the common case rather than the rare one
  after: determinism holds on one machine; identical input yields identical bytes, and requirement:incremental-generation skips unchanged sources as before
  across_machines: it does not; the directives carry an absolute path, so two checkouts at different paths generate different bytes, per path_form machine_dependence
acceptance:
  - a type error in a template expression reports the template path and line, and no generated path appears in the message
  - a panic inside a generated SQL statement function names the '.tb.sql' file in its stack frame
  - a generated line with no template origin reports against the generated file
  - a package containing generated code builds and passes go test
  - column accuracy is not required; path and line are the contract
  - the line is the line the code was written on, not that line plus the code's offset inside its generated span; see rule:line-directive-emission per-line pinning
  - with the option off, generated bytes are unchanged
surface:
  option: data:generator-options TemplateLineDirectives, off by default
  cli: 'tinybind-gen generate -template-line-directives'
  stamp: the option is part of the hashed Options value, so flipping it invalidates the requirement:incremental-generation stamp and forces the regeneration it asks for
  dialect_options: LineDirectives on each dialect's own GenerateOptions, plus OutputName naming the file the result is written as
  artifact_caller: generator.ResolveTemplatePositions names the file an artifact is written under; see rule:line-directive-emission two-step resolution
open_questions_settled:
  release_builds: the option is the flag, so a release build chooses by not setting it; nothing distinguishes release from debug inside generation
  suppression_switch: a data:generator-options field rather than an environment variable, and off rather than on, because turning it on rewrites every generated file and breaks coverage attribution
open_questions:
  - whether coverage attribution can be made coherent at all, given the rule:line-directive-emission finding that it currently is not; today the answer is to leave the option off for a covered run
  - whether the option belongs in rule:generator-feature-disable, which names features to remove rather than output shapes to add, and would read oddly for one that is already off
```
