---
id: decision:jsonv2-bench-stays-opt-in
type: decision
title: JSON V2 Bench Stays Opt In
---
internal/benchfixture's encoding/json/v2 comparison stays behind an explicit build tag rather than raising the module's declared go.mod language version, so the benchmark's needs never become every downstream consumer's minimum Go requirement.

```yaml
decided: 2026-08-24, by the maintainer
status: implemented 2026-08-24
found_by: go test ./... failing to build internal/benchfixture under a Go 1.27 host toolchain
the_defect_that_surfaced_it:
  what: 'jsonv2_bench_test.go called Token.Int() and Token.Float() as single-value, from before Go 1.27 changed both to (value, error)'
  fixed: the three call sites now check the error the same way every other token read in the file already does
  that_fix_alone_did_not_restore_the_build: every symbol from encoding/json/v2 and jsontext in the file is separately gated
the_real_cause:
  what: 'the file carried //go:build goexperiment.jsonv2, written when that tag meant "only when GOEXPERIMENT=jsonv2 is set"'
  what_changed_under_it: v2 graduated to stable in Go 1.27, and a goexperiment.X tag for a retired experiment matches unconditionally rather than requiring the flag -- so the file entered every ordinary `go test ./...` on a Go 1.27 host with no author action
  the_second_gate_hit_next: 'even once the token calls were fixed, go vet refused every symbol with "requires go1.27 or later (file is go1.26)"'
  verified_2026_08_24: this gate reads only the module''s go.mod "go" line, confirmed by direct experiment; neither the goexperiment.jsonv2 tag, GOEXPERIMENT=jsonv2, nor a toolchain directive changes it, and there is no per-file language version in Go modules
question:
  what: how to make the file buildable again
  the_fork: 'raise go.mod to go 1.27.0, or keep 1.26.0 and gate the file behind an ordinary opt-in tag'
  why_this_was_not_a_call_to_make_alone: 'go.mod''s go line is a module-wide minimum: Go''s module resolution takes the max go directive across the whole build graph, so raising it would require every downstream importer of any package in this module -- not just this internal benchmark -- to have Go 1.27 available, even one that only imports jsonbind and never touches internal/benchfixture'
  trial_run_2026_08_24: 'go.mod was bumped locally to confirm the blast radius: go build ./... and go vet ./... stayed clean elsewhere, and the benchmark ran; this proved the bump was SAFE, not that it was WANTED -- the cost is downstream, not local, so a clean local trial does not answer the question'
chosen: keep go.mod at 1.26.0; replace the tag with an explicit one that cannot match by accident
why:
  the_module_line_is_a_promise_to_every_consumer: 'raising it moves a floor under everyone who imports jsonbind, cborbind, httpbind or any other package here, for a question -- "is v2 worth switching to" -- that has no bearing on what they import'
  the_original_intent_survives: the file was always meant to be an opt-in comparison nobody hits by accident; the fix restores that contract rather than replacing it
  matches_the_project_s_standing_care_about_versions: scripts/tinygo-check.sh already hard-pins Go and TinyGo versions rather than floating them
what_it_costs:
  the_tag_alone_is_not_enough: 'go.mod''s go line still has to be raised locally -- temporarily, by whoever runs the comparison -- because Go resolves stdlib API availability from the declared language version, not from a build tag or an environment variable; the header comment and both READMEs say this explicitly so it is not rediscovered by trial and error'
  goexperiment_jsonv2_as_an_env_var_now_does_nothing: verified 2026-08-24; it neither helps nor hurts once v2 is stable, so it is no longer part of the invocation
as_built:
  tag: 'tinybind_jsonv2bench, replacing //go:build goexperiment.jsonv2'
  invocation: 'go test ./internal/benchfixture -tags tinybind_jsonv2bench -run xxx -bench JSON -benchmem, run against a go.mod temporarily raised to go 1.27.0 (or from a throwaway module importing this repo for its fixtures)'
  verified_2026_08_24: 'three states checked directly: no tag on Go 1.27 host with go.mod at 1.26 -- excluded, go build ./... and go vet ./... both clean; tag passed with go.mod still at 1.26 -- fails with the same "requires go1.27" message, so the promise the comment makes is accurate; tag passed with go.mod locally raised to 1.27.0 -- builds and the benchmarks run'
  docs: README.md and README.ja.md, both the invocation snippet and the TinyGo warning, which used to say "do not build with GOEXPERIMENT=jsonv2" -- a warning about an env var that no longer does anything now that v2 is stable; restated as "do not let a TinyGo build reach encoding/json/v2 or jsontext," which is the property the opt-in tag now itself guarantees for every package outside internal/benchfixture
not_reopened: whether encoding/json/v2 is worth targeting at all; the 3.5x stripped-wasm-size finding predates this change and is unaffected by it
related:
  - requirement:sized-integer-field-kinds
  - decision:byte-slices-are-base64
open_questions:
  - whether the project's minimum Go version should move to 1.27 for reasons unrelated to this benchmark, at which point this file's gate becomes moot and could fold back into an ordinary _test.go
```
