---
id: requirement:secret-env-sources
type: requirement
title: Secret Env Sources
---
Load accepts secret-carrying inputs beside the dotenv files: dotenv files whose values are secret by origin, and Docker-secret style directories where each file name is a variable and its content the value; every value they supply is masked in provenance without a tag or a key-name match.

```yaml
priority: should
status: implemented
source: owner follow-up 2026-09-09, after requirement:env-file-input shipped in v0.5.29
motivation:
  - a Docker or Kubernetes secret mount is a directory of one-value files, the production twin of a developer's .env
  - '.env.local and .env.{env}.local hold what a developer must not commit, so what they supply is secret whether or not the key name says so'
  - rule:secret-redaction reads the key and the tag; neither knows the value came from a secret store, so a secret under an innocent key name such as webhook.url is printed today
shape: decision:env-secret-source-shape
api: api:configbind-env-secret-sources
composition: rule:env-file-composition
directory_reading: rule:secret-dir-layout
masking: rule:secret-origin-masking
expected_consumer_mapping:
  EnvFiles:
    - '{Path: .env}'
    - '{Path: .env.local, Secret: true}'
    - '{Path: .env.{APP_ENV}}'
    - '{Path: .env.{APP_ENV}.local, Secret: true}'
  EnvSecretDirs: ['/run/secrets']
  order_note: the dotenv-flow order, corrected by the owner on 2026-09-09; a per-environment plain file outranks the shared local secrets
  note: the split between plain and secret files is the caller's, as file selection already is; configbind never infers secrecy from a file name
out_of_scope:
  - '${NAME}_FILE indirection, where a variable names a file holding the value; a directory source covers the same deployments with no per-field convention'
  - encryption, secret managers, or fetching over the network
  - masking the values written into the Bind structs; the struct keeps the real value, as with the secret tag
  - scaffolds; ScaffoldEnv lists names, never values
  - api:configbind-subcommand fields, which read no environment
compatibility: 'breaking in v0.5.31: EnvFiles changed from []string to []EnvFile; nil EnvFiles and nil EnvSecretDirs still behave as v0.5.29'
implementation:
  files: configbind/load.go EnvFile; configbind/env.go composeEnviron, readEnvFile, readEnvDir; configbind/overlay.go Entry.Secret and MarkSecret; configbind/interpolate.go expandEnvRefsFrom; configbind/provenance.go displayValue origin check
  tests: configbind/env_secret_test.go, one per acceptance line
  tinygo: wasip1 build ran under wasmtime with a secret file and a mounted directory; wasm target builds
  docs: docs/configbind.md and docs/configbind.ja.md, Secret sources
acceptance:
  - a name set only by a Secret EnvFiles entry reaches the struct unmasked and reports Masked true in provenance
  - a name set only by a file under an EnvSecretDirs entry reaches the struct with trailing line endings stripped and reports Masked true
  - '.env < .env.local < .env.stg < .env.stg.local: the key set by .env.stg over .env.local is shown, the one set by .env.stg.local is masked'
  - the same key set by a secret source and Environ takes Environ and reports Masked by key and tag only, as today
  - a key whose name matches no token and carries no tag masks when a secret source supplied it
  - 'a key tagged secret:"hide" from a secret source is dropped, as hide outranks mask'
  - a TOML string that expanded a secret-origin name through ${NAME} reports Masked true
  - a TOML string that expanded only plain-origin names keeps today's masking
  - 'a directory entry named .hidden, a subdirectory, and a K8s ..data link target are skipped, and a symlinked file is read'
  - a missing directory is skipped and a present unreadable one fails the load
  - LoadResult.EnvFiles carries each read entry Secret flag and LoadResult.EnvSecretDirs lists the directories read
  - the place of a directory value names the entry path, so EnvFileOf returns /run/secrets/DB_PASSWORD
related:
  - requirement:env-file-input
  - requirement:source-provenance-logging
  - requirement:config-env-interpolation
  - rule:secret-redaction
  - api:configbind-provenance
  - data:provenance-event
  - concept:config-overlay
  - flow:config-load
  - system:configbind
```
