---
id: rule:line-directive-emission
type: rule
title: Line Directive Emission Mechanics
---
An emitted Go line directive must survive go/format unchanged and must be restored at the end of every mapped span.

```yaml
priority: must
status: implemented 2026-08-19; mechanics verified against go 1.26.0
applies_to: requirement:template-source-positions emission in sqlbind, htmlbind, dynamobind, and firestorebind
forms:
  line_comment: "'//line path:line' or '//line path:line:col'; recognized only as the first token of a physical line"
  general_comment: "'/*line path:line*/' or '/*line path:line:col*/'; recognized anywhere a comment is allowed, including an indented one"
  effect_point: the position applies to the character immediately after the directive, and keeps applying to every later line until another directive replaces it
silent_failure:
  what: an indented '//line' is an ordinary comment; the compiler neither maps it nor complains
  consequence: a wrong emission shape produces no error and no mapping, so an acceptance test must assert reported positions rather than assert a clean build
gofmt_interaction:
  requirement: rule:generated-source-self-contained sends every Go artifact through go/format before it is returned, so only a gofmt-stable shape is emittable
  preserved: a column-1 '//line' is left at column 1 and is not indented into a composite literal
  preserved_indented: an indented '/*line*/' on its own line, and an indented '/*line*/' followed by one space and a token, both round-trip unchanged
  rewritten: an inline '/*line*/' written adjacent to its token gains a space, which shifts every mapped column by one; the adjacent form is therefore not emittable
own_line_offset:
  applies_to: the general-comment form only
  what: "a '/*line*/' occupying its own line applies from the newline after it, so the following line reports as 'line + 1'"
  not_the_line_comment_form: "a '//line' consumes its own newline, so the line after it reports as 'line' exactly; this is why the emitted shape needs no compensation"
  consequence: choosing the general-comment form for an own-line directive means emitting the target already decremented, which is an off-by-one waiting to be reintroduced
column_zero: "a column of 0 is a compile error, 'invalid column number: 0', so subtracting one to compensate for the gofmt space breaks at template column 1"
emitted_shape:
  form: "a column-1 '//line path:line' on its own line above what it maps, everywhere"
  why_one_form: a column-1 directive is left at the margin by go/format inside a function body and inside an indented composite literal alike, so the general-comment form the indented Ops element seemed to require is not needed and both dialects share one mechanism
  columns: omitted; requirement:template-source-positions asks for path and line, and every column-carrying form costs either an arithmetic compensation or the column-zero failure
  path: absolute, per requirement:template-source-positions; the toolchain shortens it against the working directory when printing, which is what makes go build and go vet agree
  indentation: an emitter indenting a nested block has to leave a directive line alone; htmlbind's indentBlock skips one rather than pushing it off the margin
  per_line_pinning:
    problem: a directive maps the line after it exactly and every later line as line+1, so only the first Go line a template node emits is right and the rest walk forward through template lines they did not come from
    worst_case: a declaration mapped as a whole function, which is the dynamobind and firestorebind shape, walks past its neighbours and off the end of a short file
    fix: the directive in force is repeated before every line of its span, in a post-format pass, so every line of a span reports the same template line
    not_for_scaffolding: a restore span is left alone, because the generated file's own lines do advance naturally and repeating a restore would be wrong as well as noisy
    ordering: after the last formatting pass and before the restore line numbers are filled in, since inserting a line moves every line a restore names
    caught: the htmlbind acceptance test was asserting the drifted line before pinning existed, and only the exact-line assertion revealed it
    cost: source only; a one-statement SQL package measured 85 lines and 2003 bytes without directives and 113 lines and 2648 bytes with them, and a comment reaches no binary
  doc_comment_adjacency: a directive emitted directly above a documented top-level declaration, which is the dynamobind and firestorebind shape, is moved by go/format below the doc text behind a bare '//' separator; the result is stable and still maps the declaration, and it is how Go formats a '//go:' directive in the same position
restore:
  required: a directive naming the generated file and the next physical line, emitted at the end of each mapped span
  omitted_directive_form: "'//line generated.go:N' with no column restores line numbering and leaves columns unmapped, which is correct for scaffolding"
  nesting: a nested Op list restores at its own end, so the element after it in the enclosing list reopens rather than inheriting the loop's line
  two_step_resolution:
    problem: a restore states a line the emitter cannot know, because go/format has not run, and a file name it cannot know either, because an artifact caller chooses the suffix it writes under
    line: filled in after the last formatting pass, by replacing each placeholder line with a directive of the same line count, so the numbering stays true as the pass walks
    name: filled in separately, as a plain string substitution over an already-numbered file, which is what lets a caller that only renames do so without recomputing anything
    exported_entry: generator.ResolveTemplatePositions runs both steps; the numbering step matches only an unresolved placeholder, so calling it on an artifact whose lines are already fixed renames and nothing more
    placeholder: a real directive naming a synthetic file, so it survives parsing and formatting like any other; a caller that skips the naming step misreports scaffolding and no template position
coverage_fallout:
  measured: a covered package under directives writes a profile that keeps the generated file path and uses the mapped line numbers
  result: block ranges point past the end of the file they name; the profile is neither template coverage nor generated coverage
  tooling: 'go tool cover -html' exits 0 and renders against the wrong lines rather than reporting the mismatch
  consequence: this is the concrete answer to the downstream coverage question; a coverage run needs directives suppressed, which makes the suppression switch a requirement rather than a convenience
combiner_constraint:
  found: the template combiner rebuilt an ast.File from the parsed declarations of each generated source, and printing a synthetic file drops every comment that is not a declaration doc
  effect: every directive lives inside a function body or a composite literal, so all of them were dropped silently and the feature emitted nothing that survived
  not_fixable_by_carrying_comments: setting File.Comments on the synthetic file misplaces them, because the prepended import declaration has no position and the printer interleaves by position
  fixed_as: combining merges text instead, taking each source's bytes from its first declaration onward and parsing only for the import set and the duplicate-name check
  same_for: splitTemplateArtifacts, which had the same shape per artifact
  measured: every existing fixture is byte-identical after the change, because the emitters write no body comment of their own today

acceptance:
  - a generated artifact is byte-identical before and after go/format, directives included
  - a deliberately broken template expression reports the template path and line, asserted on compiler output rather than on build success
  - a line the emitter wrote for itself reports the generated path
  - a diagnostic inside a For body reports the loop body line, and the first scaffolding line after the loop reports the generated path
  - suppression yields output identical to today's, so an existing fixture is unchanged when directives are off
```
