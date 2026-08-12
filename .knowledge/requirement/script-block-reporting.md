---
id: requirement:script-block-reporting
type: requirement
title: Script Block Reporting
---
Report, per component declaring a script block, the block's text as authored and the client handler names its markup referenced, so a caller resolves both without reading the template itself.

```yaml
priority: should
status: implemented 2026-08-12
as_built:
  reader: htmlbind.ComponentScripts, beside ActionRefs and Signatures, running the analysis Generate runs
  also_on_the_result: Result.ComponentScripts, so a caller that already compiled need not parse a second time
  reports: the block text as authored, its position, the client handler names the component's markup referenced, and the component's declared parameter names
  parameters_were_added: not asked for; a caller choosing which parameters to emit needs the declared set to pick from, and deriving it from the template a second time is the drift this concept exists to avoid
  integrated_path: routetree.GenerateOptions.ScriptResolver reports the blocks and takes back a ScriptAnswers, so the feature is reachable from the tree generator and not only from a direct htmlbind compile
  second_parse_is_opt_in: a resolver costs one extra parse per template carrying a block, because the blocks must be reported before the compile that consumes the answers; a tree configuring none parses once, as before
  tests: templates/htmlbind/script_report_test.go over verbatim text, referenced handlers, declared parameters, omission of a blockless component, failing like Generate, and stability across runs
source:
  - downstream framework change request 2026-08-11, ask 3
  - decision:client-handler-seams
  - requirement:component-script-block
review_gate: proposed
model:
  what_it_is: the seam requirement:template-client-handlers and requirement:component-parameter-emission both consume, and the reason neither needs a JavaScript parser in this module
  round_trip: the module reports what it holds, the caller reads the JavaScript and answers, and the answer returns as a compile option
  precedent: identical in shape to ActionRefs and GenerateOptions.ServerActions, which decision:server-action-lowering resolution_dataflow already established for a name the compiler cannot resolve
shape:
  reader: a third function beside ActionRefs and Signatures, running the analysis Generate runs, so a module that fails to compile fails here with the same diagnostic rather than yielding a partial answer
  per_component: only a component declaring a script block appears
  block_text: the content as authored, which templates/htmlbind collectScriptBlock already holds verbatim
  referenced_handlers: the handler names the component's markup named, which requirement:template-client-handlers records
  positions: carried, so the caller's answer can be reported against source
why_the_module_reports_the_text:
  the_caller_could_find_it: the block is in the template source and the reader could locate it
  drift: doing so duplicates the raw-text boundary rules of the parser's readRawUntilClose path, which is a divergence the caller would be creating deliberately
  ordering: the stronger reason; the extracted asset exists only after the compile that needs the answer, so reading the output is not available to a pass that feeds the input
  scale: a field on a value the compiler already builds, against a second implementation of a parsing rule
two_halves_with_different_readiness:
  block_text: standalone; it depends on nothing in this round
  referenced_handlers: depends on requirement:template-client-handlers having reserved and recorded the attribute, because there is nothing to report before then
  consequence: the reporter sequenced this concept first among its three and half of it cannot be built first; see decision:client-handler-seams sequencing_correction
constraints:
  - reporting only; no emitted byte changes and no generated artifact moves
  - the module reads no JavaScript, per decision:client-runtime-ownership and requirement:scoped-script-declaration what_the_module_does_not_do
  - a component declaring no block is absent from the report rather than present and empty
acceptance:
  - a component declaring a script block reports that block's text byte for byte, braces and template literals included
  - a component declaring no block appears in no report
  - a module that fails to compile fails this reader with the same diagnostic
  - the report is stable across runs for unchanged input, per requirement:static-asset-extraction determinism
  - calling the reader emits nothing and changes no output
related:
  - requirement:component-script-block
  - requirement:template-client-handlers
  - requirement:component-parameter-emission
  - decision:lifecycle-from-declaration-block
```
