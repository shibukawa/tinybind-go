---
id: rule:allocation-assertion-toolchain-guard
type: rule
title: An Allocation Assertion Must Check That The Toolchain Counts
---
Guard every allocation assertion on a probe that the toolchain counts allocations at all, because TinyGo reports zero for one that plainly escapes and an unguarded assertion passes there while measuring nothing.

```yaml
source:
  - requirement:append-style-json-escaper as_built 2026-08-21
  - requirement:tinygo-wasm
review_gate: proposed
finding:
  what: testing.AllocsPerRun under TinyGo 0.41.1 returns 0 for make([]byte, 4096) assigned to an escaping package variable
  therefore: a zero from it means nothing was measured, not that nothing was allocated
  failure_mode: silent; the assertion the test exists for passes on the toolchain the project treats as first-class, and only a negative control catches it
  how_it_surfaced: an assertion that JSONString still allocates, written to keep the corpus honest, failed under scripts/tinygo-check.sh while the zero-allocation assertions beside it passed
rule: probe with something that certainly allocates, and skip rather than assert when the probe reports zero
second_finding:
  what: TinyGo's t.Skip does not end the test — SkipNow there prints "SkipNow is incomplete, requires runtime.Goexit()" and execution continues
  consequence: the first version of this guard skipped and then ran the assertion it had just skipped, and TinyGo reported the test as failed
  therefore: the guard cannot lean on Skip's unwinding; the caller has to return
  general_reading: any t.Skip in a test this project runs under TinyGo must be followed by a return, whatever the reason for skipping
shape:
  probe: AllocsPerRun over a make() whose result escapes, in a helper the assertions call first
  outcome: the caller logs the reason and returns; it does not call t.Skip, per second_finding
  example: htmlbind allocationsAreCounted
why_not_a_build_tag:
  alternative: exclude the test with !tinygo
  why_not: it names the toolchain rather than the condition, so a toolchain that starts counting does not get the assertion back until someone edits the tag, and any other non-counting toolchain stays unguarded
  cost_of_the_probe: one AllocsPerRun call per test, which is the same measurement the test was going to make anyway
  reconsidered: second_finding weakens this, because a tag would have avoided the broken Skip entirely; the probe still wins on naming the condition, and a plain return costs nothing once Skip is off the table
scope:
  applies_to: assertions on an allocation count
  not_benchmarks: b.ReportAllocs reports rather than asserts, and the project's benchmarks are host-only, so nothing there passes falsely
  first_case: this repository had no allocation assertion before requirement:append-style-json-escaper, so the rule is written from the first one rather than from a defect found in an old one
```
