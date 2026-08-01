---
id: decision:boundary-update-execution
type: decision
title: Boundary Update Execution Model
---
Split update execution two ways: a navigation re-executes the whole handler, and a redraw executes one registered component alone.

```yaml
source:
  - requirement:component-redraw-endpoint
  - requirement:component-delta-rendering
  - user execution questions 2026-08-01
review_gate: proposed architecture requires user approval
modes:
  navigation:
    trigger: a URL change, including a search parameter
    execution: run the handler, layouts, and page as an ordinary request
    inputs: derived by the server from the request, exactly as for a complete document
    response: only the boundaries whose markup changed
  redraw:
    trigger: api:client-component-update naming one instance
    execution: run that component alone
    inputs: every declared parameter, supplied by the caller
    response: that component's subtree
completeness:
  claim: the two modes cover the design surface, and no third mode is needed
  self_contained_state: a component whose inputs the browser owns is a redraw
  server_derived_state: anything the server must derive belongs in the URL, which makes it a navigation
retired_third_mode:
  what: re-execute the whole handler, patch one client-mutable parameter, and return only that subtree
  fatal_flaw: the patch reaches only the target component, so it cannot influence the data fetch that produces that component's other inputs
  example: a sort order patched into a table cannot reach the page's ORDER BY, so the page returns the same rows and the parameter is limited to presentation below the fetch
  contract_cost: the narrow scope is easy to misread as general, so a parameter that should have changed the query silently does not
  avoidable_both_ways:
    url: state that must reach the data fetch belongs in the URL, where a navigation already handles it and the user gains a shareable, bookmarkable, back-navigable page
    self_contained: state that must stay out of the URL means the component should fetch its own data, which is exactly the condition for registering it as reloadable
  removed_with_it:
    - the client-mutable parameter allowlist, since a registered component's whole parameter list is its public interface
    - the patch request shape and its flow
    - signed continuations, which existed only to avoid re-execution
inputs_and_trust:
  navigation: inputs come from the request, so they carry the trust the request already had
  redraw: inputs come from the caller, so rule:redraw-input-trust applies and the component authorizes them itself
  identity: neither mode reconstructs anything; a navigation reproduces positional identity by re-executing, and a redraw is named by decision:author-declared-boundary-id
history:
  2026_08_01_first: rejected subtree-only execution, because reconstructing inputs from a captured continuation would render from stale data and stale authorization
  2026_08_01_second: accepted subtree-only execution for a redraw, because the caller supplies the inputs fresh and nothing is captured; the objection applied to the continuation, not to the execution scope
open_questions:
  - declaration syntax for registering a component as reloadable
  - whether a redraw needs policy:html-update-csrf-protection, given a side-effect-free GET with ambient credentials
```
