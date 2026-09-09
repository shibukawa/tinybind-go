---
id: decision:env-secret-source-shape
type: decision
title: Env Secret Source Shape
---
Secret files are EnvFiles entries carrying a Secret flag, read in the one slice order; directories are a separate slice above all files; both reuse the file place rather than adding one.

```yaml
status: accepted
history:
  v0.5.30: 'EnvSecretFiles []string beside EnvFiles []string, fixed order plain < secret'
  v0.5.31: >
    replaced by the Secret flag after the owner corrected the convention; both
    tags were pushed but fresh enough that compatibility was waived
chosen:
  fields: 'EnvFiles []EnvFile{Path, Secret} and EnvSecretDirs []string'
  order_low_to_high: [EnvFiles in slice order, EnvSecretDirs in slice order, Environ]
  why_one_slice_for_files: >
    the dotenv-flow convention is .env < .env.local < .env.{env} < .env.{env}.local,
    so a plain per-environment file sits above the shared local secrets; a
    fixed plain-then-secret order cannot express that, and the caller already
    owns the order
  why_dirs_stay_separate: >
    a directory has no parser and no line numbers, and a mounted secret store
    belongs above every file in every deployment seen; interleaving it bought
    nothing
alternatives:
  - id: two-slices
    shape: 'EnvFiles []string and EnvSecretFiles []string'
    why_not: the v0.5.30 shape; cannot put .env.{env} above .env.local
  - id: ordered-source-list-with-kind
    shape: 'EnvSources []EnvSource{Path string, Kind Kind, Secret bool}'
    why_not: folds directories into the same list for an interleaving no deployment asks for
  - id: infer-secrecy-from-name
    shape: '.local suffix or /run/secrets prefix marks the source secret'
    why_not: configbind receives paths and never reads intent from them, per requirement:env-file-input
place:
  chosen: PlaceEnvFile + path for secret files and for directory entries
  why: a summary wants the file name, and EnvFileOf already returns it; secrecy is a provenance fact and data:provenance-event Masked carries it
  rejected: 'PlaceSecretFile and PlaceSecretDir, which would make every consumer switch on three prefixes to print one name'
result_reporting:
  chosen: LoadResult.EnvFiles carries the Secret flag; LoadResult.EnvSecretDirs beside it
  why: a doctor command filters plain from secret by the flag, with no second list to keep aligned
related:
  - requirement:secret-env-sources
  - api:configbind-env-secret-sources
  - api:configbind-env-files
  - rule:env-file-composition
  - rule:secret-origin-masking
  - rule:secret-dir-layout
  - system:configbind
```
