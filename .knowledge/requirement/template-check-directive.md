---
id: requirement:template-check-directive
type: requirement
title: Template Check Directive
---
Call a synchronous external for its error alone, binding no name, so a template can refuse a render without inventing a value to read.

```yaml
priority: should
source:
  - owner proposal 2026-08-16, following requirement:template-value-binding failing_external
review_gate: shape approved 2026-08-16 by the owner; built the same day, see as_built
formats: both, per sql_format
problem:
  today: requirement:template-value-binding failing_external gives a failing external exactly one position, the whole value of a val binding, and unread_binding then requires the bound name be read somewhere
  consequence: a call made only to refuse the render must fabricate a result type and fabricate a reader for it
  case: an authorization or precondition check that answers nothing but whether rendering may continue
  not_an_escape_hatch:
    unread_binding_unchanged: its escape_hatch stays none; the rule is about a computed value nothing reads
    why_this_is_outside_it: a directive that binds no name has no result to discard, so there is no discarded call for the rule to catch
    still_forbidden: a call made for a side effect, which rule:render-external-query-semantics refuses whatever the directive is called
shape:
  working: |
    {check Authorize(user)}
  keyword: check
  is: a val minus the binding; same hoisting, same block extent, same destination for a failure
  closer: none, as val has none
  binds_nothing: no name enters scope, so requirement:template-value-binding shadowing and unread_binding have no subject here
  one_call_per_directive:
    decided: 2026-08-16 by the owner
    why: a val comma list buys several names on one line, and with no name there is nothing to share; two checks are two directives
    parser_consequence: parseVal is not reused as it stands, since it parses name-equals-expression pairs
  expression: a call, not any expression; a field path or a literal has no error to check, and parseCheck refuses it against the syntax rather than leaving it to typing, because the position wants a call whatever the callee turns out to be
declaration:
  form: external Name(params) with no result type
  means: the call yields no value, and an error is its whole answer
  value_less_external_is_not_a_value:
    rule: a declaration with no result type may be called only in a check directive; every other position is a generation error naming the function
    same_shape_as: requirement:html-slot-syntax, where an html result is a non-value type usable at one position only
    typing: one more result kind beside the html one, unreachable from parseTypeRef, so no author-visible type is added
  result_type_optional:
    decided: 2026-08-16 by the owner
    rule: an external declared with a result type may also be checked, and the result is discarded
    why: the directive reads the call for its error, and what precedes the error is not this position's business
trailing_error:
  rule: the last Go result is the error and everything before it is dropped
  arity_from_the_declaration: no declared result emits a direct return of the call, one declared result emits a blank assignment beside the error; the grammar has no tuple, so the count is zero or one
no_scan_needed:
  finding: unlike requirement:template-value-binding failing_external, this needs no GenerateOptions.ErrorExternals entry
  why: the directive itself asserts the trailing error, so the emitted shape follows from the template declaration alone
  used_where_it_is_filled: a caller that does fill the map gets a template diagnostic instead of a Go one for a checked call that cannot fail, in the two shapes it can take — no result and no error, or a value and no error; a caller that fills nothing gets the permissive reading and the Go compiler
  contrast: a val cannot know, because `external LoadData(id: string): Record` reads the same whether or not the Go function can fail, which is why internal/externalscan exists for that case
  mismatch: an implementation returning no error is an ordinary Go compile error at the generated call site, the outcome internal/externalscan already documents for every signature that does not match
  consequence: no generator, routetree, or GenerateOptions change; a check means the same thing through every path that compiles a template, with no caller_scope caveat of its own
context:
  allowed: requirement:render-context-externals applies unchanged, so an implementation may declare a leading context.Context
  needs: a RequireCtx op beside Require, added for this
failure_destination:
  same_as_val: the render ends and the error reaches the caller, per requirement:template-value-binding failing_external difference_from_async
  status_selection: decision:value-binding-hoisting decides it; a check outside a control block runs during chain assembly with nothing written, so an error carrying HTTP intent per requirement:redirect-error still chooses the response
  inside_a_control_block: runs when that block runs, so it ends the render but cannot choose a status, unchanged from a val there
no_async_form:
  what: a value-less `external async`, checked concurrently and absorbed by a recover clause
  decided: not wanted, 2026-08-16 by the owner
  why: a check exists to refuse a render, and a boundary's failure lands after the shell is committed, where it can no longer choose a response; the htmlbind Require doc states the same thing about running on the initial pass rather than in the boundary goroutine
  what_recover_would_make_of_it: a refusal a clause can swallow, which is the opposite of the construct
  no_gap_to_close: requirement:async-external-functions already requires an async external to return an error and already gives that error a destination, so nothing there is blocked the way the synchronous case is by requirement:template-value-binding unread_binding
cache:
  decided: 2026-08-16 by the owner — a cache hit skips the check, exactly as it skips a loader bound by a val
  why: the check sits inside the cached subtree and decision:cache-key-derivation keys on the declared parameters, so a hit is a hit and nothing runs
  who_carries_the_hazard: rule:render-external-query-semantics already makes output with undeclared request, user, locale, or authorization dependencies ineligible for reuse; that rule carries this case and is not amended
  rejected:
    what: refusing a context-taking check inside a requirement:component-output-cache component
    why_not: the same argument applies to a context-taking value external in a cached component, so it is not this requirement's to invent, and a check reading only its declared parameters is safe to skip
lowering:
  html:
    op: htmlbind Require, which exists and already has the shape — runs on the initial pass, writes nothing, ends the render on a non-nil error
    context_form: a RequireCtx variant, new
    position: hoisted to the top of its block per decision:value-binding-hoisting, in source order among that block's directives
    no_body: unlike a val the directive nests no siblings, because there is no scope to open; a failure ends the enclosing instruction list on its own
    scope_struct: none
    gates: the keyword joins insertionKeywords so it is recognized in a script or style body, and isControl so an attribute refuses it as a block, as val's did
  sql:
    shape: one error-checked call statement where the directive stands, matching how requirement:template-value-binding emits a local there
    emits_no_sql: the directive contributes no bytes to the statement, so surrounding text nodes carry the spacing unchanged
    nothing_generated_to_carry_scope: no local, no struct, no closure
  formatter: round-trips with no closer and no added indentation, as val does
precondition_defect:
  what: htmlbind requireOp reaches the default arm of sequenceOf and becomes a SeqSlot
  measured: 2026-08-16, throwaway test against the current tree
  why_wrong: SeqSlot consumes one value in Reassemble and Require produces none, so the sequence carries one hole more than the render has values for
  reached_today_by: requirement:awaitable-parameters alone, whose unset check emits Require ahead of an await
  reached_after_this_by: every page carrying a check directive
  fix: a sequenceInline hook returning no nodes, the same hook requirement:template-value-binding sequence_splice established for an op that runs once and contributes no marker
  severity: requirement:component-delta-rendering degrades with no diagnostic, which is why it is stated here rather than left to the implementer
  fixed: 2026-08-16, ahead of the directive and in the same commit, since the second instance below is the directive's own
constraints:
  - rule:render-external-query-semantics is unchanged; a check is a query whose answer is a failure, not a command
  - rule:usage-directed-generation holds; a project writing no check directive generates byte-identical Go
  - decision:reflection-free holds; the call is emitted as a direct typed call
  - requirement:template-v1-scope is untouched; the directive names no value and introduces no control flow
acceptance:
  - a check directive calling a value-less external compiles, and a non-nil error ends the render with that error and writes none of the block
  - the same error carrying HTTP intent reaches api:write-error as the value the external returned
  - a check written after markup in its block runs before that markup, and the rendered bytes are unchanged
  - a check written outside any control block chooses the response status; one written inside an if, for, or await body does not
  - a value-less external called anywhere but a check directive fails generation and names the function
  - an async or live external declared with no result type fails generation, since there is no async form of a check
  - a check on an external declared with a result type compiles and discards the result
  - a checked call that cannot fail is refused by name where the error scan is filled, in both its shapes, and falls through to the Go compiler where it is not
  - two checks in one directive fail generation and the diagnostic says to write two directives
  - a check in an attribute value is refused as a block, with the diagnostic a val binding gets in the same position
  - a check whose implementation declares a leading context receives the render context
  - a page carrying a check still decomposes into a sequence tree with no extra hole, and a page using requirement:awaitable-parameters does too once the precondition defect is fixed
  - a cached component containing a check runs it on a miss and not on a hit
  - a template with no check directive generates exactly the bytes it generates today, in both formats
  - a source written with a check round-trips through the formatter with no closer and no added indentation
as_built:
  when: 2026-08-16, both formats in one change
  shared: syntax.CheckNode, parseCheck for the closerless one-call form, CheckString for printing it back, and an optional result clause on parseExternalDecl whose absence both formats read as no result
  typing: one more result kind in each format's own enum, produced only by the absent clause and refused everywhere but the directive; no author-visible type was added
  html:
    normalization: the hoist collector carries either a binding or a check, and a check is put back as a sibling rather than wrapped around what follows, which is what preserves written order between the two
    op: htmlbind Require, unchanged, plus a new RequireCtx for the context-taking case
    usage_scan: a check counts as a reader, so a binding inspected by one and nothing else is not reported unread
    gates: the keyword joined insertionKeywords and isControl, as val's did
  sql:
    lowering: one error-checked call statement where the directive stands; no local, no scope, nothing to carry
  three_things_the_requirement_did_not_predict:
    sequence_node:
      what: the precondition_defect above, fixed first — requireOp now answers sequenceInline with no nodes
      note: it was predicted; what was not is that the same hole existed for checkedOp below and had to be closed twice
    assembly_prologue:
      what: prepareOps stopped its walk at the check, so a loader written after a guard was never prepared
      symptom: the loader failed after the document shell was written, and its 404 or redirect became a 200 with a problem document in the body
      why: the walk returns after preparing one instruction, because a binding nests the rest of the list inside itself; a check nests nothing, so the siblings after it are still leading instructions
      fix: a second interface, preparableInPlace, for an instruction that runs to completion during assembly and swallows none of its list; the check runs there and leaves a checkedOp behind, and the walk continues
      caught_by: the route fixture, which already asserted that a self-loading page picks its own status; the unit tests all passed
    boundary_root:
      what: boundaryRoot refused the check node, so a component that guarded itself silently stopped being an update boundary
      same_as: the value-binding case fixed 2026-08-16 in the commit before this one, for the same reason — hoisting puts the directive where the root element was
      difference: a check has no body, so it is stepped over outright rather than threaded through
      caught_by: regenerating the route fixture, whose emitted boundary disappeared
  fixture: the records route now declares a check beside its loader, so the 403 case runs the generated code rather than reading it, and the existing boundary test covers the root scan
  tests: lowering and hoisted position in both formats, the context variant, the discarded result, every refused position, the one-call rule, the call shape, the async refusal, the unread-binding interaction, the boundary root, formatter round-trip, the runtime sequence and reassembly, the assembly prologue, and a compiled-and-run SQL check
related:
  - requirement:template-value-binding
  - requirement:render-context-externals
  - requirement:template-language-core
```
