---
id: requirement:env-file-input
type: requirement
title: Env File Input
---
Load accepts dotenv files as an input laid under the process environment, and provenance names the file that set a key, so no consumer re-implements the env layer to learn where a value came from.

```yaml
priority: must
status: implemented
source: downstream request 2026-09-08, filed against v0.5.28
consumer:
  who: the framework building on this stack, whose startup summary and doctor command name the source of each key
  today: >
    a package outside configbind reads .env files, lays them under the process
    environment, and calls Load once per prefix of the file list plus once with
    everything, then attributes each env-placed key to the last baseline whose
    entry differs; N+1 loads, each re-applying onto the Bind targets
  evidence: internal/dotenv on the consumer's env-file-handling branch at 5a025f3f, not yet on its main
  after: passes the paths and deletes the baseline loads and the attribution
why_here:
  - configbind already owns the env layer and the key-to-variable table; the consumer can only guess what Load knows
  - a dotenv file is the ordinary home for a developer's secrets, so every consumer wants the same input
api: api:configbind-env-files
composition: rule:env-file-composition
parser: decision:env-file-parser
provenance:
  place: term:config-source value file_env, carrying the file name
  masking: rule:secret-redaction is keyed by term:config-key, so file-supplied secrets mask as today
  unchanged: api:configbind-provenance reports the place and nothing else moves
out_of_scope:
  - choosing which files to read from an environment token; the caller derives .env.{APP_ENV} itself and hands over paths
  - '${NAME} expansion inside dotenv values; the file layer of requirement:config-env-interpolation still expands from the composed set'
  - scaffolds; ScaffoldEnv and ScaffoldTOML do not change
  - api:configbind-subcommand fields, which never read the environment
compatibility: additive; a nil EnvFiles behaves exactly as v0.5.28
implementation:
  files: configbind/env.go composeEnviron and mergeEnv, configbind/overlay.go PlaceEnvFile and EnvFileOf, configbind/load.go the environ site
  tests: configbind/env_file_test.go, one per acceptance line
  tinygo: wasip1 build ran under wasmtime with two files and a missing one; wasm target builds
  docs: docs/configbind.md and docs/configbind.ja.md, Reading dotenv files
acceptance:
  - same name in file[0], file[1], and Environ resolves to Environ; file[1] wins when Environ lacks it
  - a key set only by .env.stg reports place file_env:.env.stg
  - the same key exported in Environ reports place env
  - a key set in both files reports the later file
  - '${NAME} in the TOML resolves from a file-supplied name'
  - a missing file is skipped and absent from LoadResult.EnvFiles
  - a present unreadable file fails the load
  - a rejected line fails the load naming the file and line
  - LoadResult.EnvFiles lists only the files read, in order
  - 'NAME= counts as set, as an exported empty variable does'
  - a subcommand definition still reads no environment
  - the existing env and ExtraConfigReadPaths tests pass unchanged
release:
  tag: v0.5.29
  consumer_follow_up: bump the pin, pass EnvFiles, replace the interim dotenv: place prefix with EnvFileOf, keep file selection and doctor checks on its side
related:
  - requirement:layered-config-load
  - requirement:config-env-interpolation
  - requirement:source-provenance-logging
  - requirement:configbind-tinygo
  - concept:layered-config-sources
  - concept:config-overlay
  - rule:source-precedence
  - flow:config-load
  - system:configbind
```
