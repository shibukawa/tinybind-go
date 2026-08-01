---
id: rule:raw-text-insertion-gate
type: rule
title: Raw Text Insertion Gate
---
Inside script and style content a brace opens a template insertion only in a recognized shape; every other brace is authored JavaScript or CSS.

```yaml
source:
  - rule:template-context-safety
  - framework owner review 2026-07-30, petitweb-go
review_gate: approved 2026-07-30
scope:
  contexts: [html:script, html:style]
  not_covered:
    head_contribution: a head element declared outside html already reads script and style bodies verbatim, so no brace reaches this gate
    every_other_context: html:child and html:attribute keep the unconditional brace grammar
problem:
  before: every brace in script and style content entered the expression grammar, so ordinary JavaScript and CSS failed to parse
  messages: the three an author actually hit were 'expected expression' for {}, 'invalid expression character "{"' for JSON, and 'invalid expression character ";"' for a declaration
  none_named_the_context: the position pointed inside the element but the message described an expression, so the language being read was left to the author to infer
  silent_case:
    what: a JavaScript template literal placeholder ${name} was read as an insertion
    consequence: with a trusted_javascript parameter of that name it compiled, and the server substituted a value into what the author wrote as client code
    severity: no diagnostic at all, which is why this is a rule change and not only a diagnostics change
gate:
  tight: the insertion content starts at the brace; a space, tab, or line break after it makes the brace content
  why_tight: a block in either authored language opens with a space or a line break far more often than not, and the shapes alone cannot separate {js} from '{ this.value = 1 }' or '{ render() }'
  dollar: a brace preceded by '$' is content, because it is a JavaScript template literal placeholder the browser evaluates
  shapes:
    closing_directive: '{/' followed by the block keyword
    parenthesized: '{(' any expression ')}'
    control_keyword: one of if, else, for, await, recover, fallback, followed by its clause
    bare_value: an identifier, then optional blanks, then '}'
    member_access: an identifier followed by '.'
    call: an identifier followed by '('
  otherwise: content, appended as text
  closing_brace: a '}' in these contexts is always content, because raw text is terminated by its closing tag and no brace there can close a declaration body or a control block
escape:
  form: '{{ ... }}' emits one literal brace pair and parses nothing inside
  status: present in every format parser since before this rule; this rule documents it rather than adding it
  needed_for: content that does match a shape above, most often the single-property object shorthand '{name}'
residual_ambiguity:
  accepted_direction: a collision resolves toward insertion, so it surfaces as a diagnostic rather than as silent text
  cases:
    - a minified single-statement block such as 'if(x){render()}' reaches analysis and fails as an unknown function
    - a tight object shorthand such as 'const o = {name};' reaches analysis and fails as an unknown identifier
    - a tight shorthand whose name matches a parameter of an insertable type compiles and substitutes; this is the one remaining silent case and the escape is the documented remedy
  why_acceptable: both collisions need the tight form, which authored code writes far less often than the spaced form, and both are named by the diagnostic
diagnostics:
  requirement: a diagnostic raised for an insertion in these contexts names the element and the ways out
  ways_out: the '{{ }}' escape, the requirement:explicit-output-control intrinsic for that context, and moving the body to a requirement:static-asset-extraction file
  parser_and_analysis: the hint is attached in both stages, because a shape the gate accepts fails in analysis rather than in the parser
  shell_head_note: a diagnostic inside the document shell's own head also names the contributing head form, which reads the same markup verbatim
rejected_alternatives:
  keyword_only:
    what: treat a brace as template syntax only before a control keyword
    why_not: expression insertion in these contexts is documented and fixture-covered, so RawCSS, RawJavaScript, and JsonForScript calls would stop working
  try_parse_fallback:
    what: attempt the expression grammar and fall back to text on failure
    why_not: a typo inside a real insertion would become text and ship broken output with no diagnostic, which inverts the accepted_direction above
  configurable_delimiters:
    what: a per-file or per-generation-unit override of the brace delimiters
    why_not: delimiters are grammar, which decision:template-parser-delegation keeps in the shared layer for all three format parsers, and a per-file grammar means no reader can read a body without its header
    also: it does not help the silent ${name} case, because an author who knew to set it would already have known to escape
  new_sigil:
    what: require a distinguishing character before the brace in raw text
    why_not: it breaks every documented insertion at once for a problem the gate closes without a syntax change
compatibility:
  authored_content: markup that failed to parse now parses, so no working template changes meaning by that route
  spaced_insertion: '{ js }' inside script or style content becomes text; the tight form and the documented examples are unaffected
  other_contexts: html:child and html:attribute are untouched, so a child-position or attribute insertion keeps working with any spacing
verified:
  fixture: testdata/templates/htmlparser/raw_text_braces
  parser_tests: TestRawTextBracesStayAuthoredContent, TestRawTextInsertionShapes, TestParserDiagnostics
  analysis_tests: TestGenerateDiagnostics
related:
  - rule:template-context-safety
  - requirement:explicit-output-control
  - decision:component-style-delivery
  - decision:framework-script-delivery
```
