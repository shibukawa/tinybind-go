---
id: decision:env-secret-source-shape
type: decision
title: Env Secret Source Shape
---
Secret inputs are two more LoadOptions slices with a fixed position in the composition order, and they reuse the file place rather than adding one.

```yaml
status: accepted
chosen:
  fields: EnvSecretFiles and EnvSecretDirs beside EnvFiles
  order_low_to_high: [EnvFiles, EnvSecretFiles, EnvSecretDirs, Environ]
  why_fixed_order: >
    the dotenv-flow convention is .env < .env.{env} < .env.local < .env.{env}.local,
    so every secret file sits above every plain file already; a mounted secret
    store outranks a checked-in-adjacent file; the process outranks all, as today
  why_two_kinds_not_one: a directory has no parser and no line numbers; a file has no entry names
alternatives:
  - id: ordered-source-list
    shape: 'EnvSources []EnvSource{Path string, Kind Kind, Secret bool}'
    why_not_now: >
      strictly more general, and the right shape if a caller ever needs a plain
      file above a secret one; nothing asks for that today and three slices read
      the same way ExtraConfigReadPaths does
    revisit_when: a consumer needs interleaving or a fourth kind
  - id: secret-flag-on-envfiles
    shape: 'EnvFiles []EnvFile{Path string, Secret bool}'
    why_not: breaks the v0.5.29 field for every caller to add one bool
  - id: infer-secrecy-from-name
    shape: '.local suffix or /run/secrets prefix marks the source secret'
    why_not: configbind receives paths and never reads intent from them, per requirement:env-file-input
place:
  chosen: PlaceEnvFile + path for secret files and for directory entries
  why: a summary wants the file name, and EnvFileOf already returns it; secrecy is a provenance fact and data:provenance-event Masked carries it
  rejected: 'PlaceSecretFile and PlaceSecretDir, which would make every consumer switch on three prefixes to print one name'
result_reporting:
  chosen: LoadResult.EnvSecretFiles and LoadResult.EnvSecretDirs beside EnvFiles
  why: a doctor command reports plain and secret inputs apart
related:
  - requirement:secret-env-sources
  - api:configbind-env-secret-sources
  - api:configbind-env-files
  - rule:env-file-composition
  - rule:secret-origin-masking
  - rule:secret-dir-layout
  - system:configbind
```
