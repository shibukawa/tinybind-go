---
id: requirement:explicit-output-control
type: requirement
title: Explicit Output Escaping Control
---
Provide context-typed intrinsics for explicit trusted output and safe JSON embedding in HTML documents.

```yaml
source: concept:typed-template-language
intrinsics:
  RawHTML:
    signature: RawHTML(value string) -> trusted_html
    context: HTML child-node list
    behavior: emit unchanged; caller explicitly asserts trusted markup
  JsonForScript:
    signature: JsonForScript(value statically-serializable) -> script_json
    context: JavaScript data position inside script content
    behavior: generated typed JSON serialization safe for HTML script embedding
  RawCSS:
    signature: RawCSS(value string) -> trusted_css
    context: style element content
    behavior: emit unchanged; caller explicitly asserts trusted CSS
  RawJavaScript:
    signature: RawJavaScript(value string) -> trusted_javascript
    context: script element content
    behavior: emit unchanged; caller explicitly asserts trusted JavaScript
type_rules:
  - trusted_html, trusted_css, trusted_javascript, and script_json are distinct opaque types
  - no implicit conversion from string or between trusted output types
  - each type is accepted only in its declared output context
  - Raw* calls are explicit unsafe trust boundaries, not sanitizers
json_for_script:
  - reject values without a statically known generated JSON codec
  - preserve normal JSON string escaping
  - escape HTML-sensitive less-than, greater-than, and ampersand characters
  - escape U+2028 and U+2029 for JavaScript parser compatibility
  - prevent a serialized value from terminating the containing script element
  - produce data only; never treat input as executable JavaScript
script_json_lowering:
  call_form: JsonForScript is lowered rather than called, because the encoding is generated per argument type, so the emitter reaches the argument through the call expression
  value_form: a script_json value reaching script content any other way is already encoded and is emitted raw, without unwrapping
  sources: a parameter declared script_json, a record field, and an external result
  fixed: 2026-07-30; the emitter asserted every script_json insertion was a JsonForScript call and panicked on the value form, which rule:raw-text-insertion-gate documentation surfaced
diagnostics:
  - reject trusted_html in attributes, style content, or script content
  - reject trusted_css outside style content
  - reject trusted_javascript and script_json outside script content
  - identify Raw* as a security-sensitive trust assertion
out_of_scope:
  - sanitizing untrusted HTML, CSS, or JavaScript
  - CSP nonce or hash management
  - safe dynamic CSS value construction
```
