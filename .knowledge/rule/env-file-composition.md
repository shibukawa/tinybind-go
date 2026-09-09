---
id: rule:env-file-composition
type: rule
title: Env File Composition
---
Load composes one environment from the dotenv files then Environ, feeds it to both the env layer and the file-layer expansion, and remembers per name which file won.

```yaml
order_low_to_high:
  - EnvFiles[0]
  - EnvFiles[n-1], plain and Secret entries in the one slice order, per requirement:secret-env-sources
  - EnvSecretDirs in slice order, entries per rule:secret-dir-layout
  - Environ, or os.Environ() when nil
composition:
  - build the map once at the environMap site in Load; later entries overwrite earlier on the same name
  - the env layer readEnvMap and the TOML ${NAME} expansion read that one map, so a file-supplied DATABASE_URL resolves in the TOML exactly as a shell export would
  - a nil EnvFiles leaves the map identical to today
winner_record:
  - per variable name, the file that supplied the winning value, or the process
  - also whether that supplier was a secret source, feeding rule:secret-origin-masking
  - when the env layer sets a term:config-key from a file-supplied name, the overlay place is PlaceEnvFile plus that file instead of env
  - multi-valued keys through MergeMultiMap get the same place
  - a name supplied by a file and by Environ records the process, so the key reports env
empty_assignment: 'NAME= is set, matching an exported empty variable'
precedence_fit: >
  rule:source-precedence is untouched; file_env is a refinement of the env layer's
  place, not a new layer, so cli still overrides it and it still overrides file_toml
not_read_here:
  - api:configbind-subcommand definitions, which keep reading no environment
  - which files exist; that is the caller's decision and LoadResult.EnvFiles reports the outcome
applies_to:
  - requirement:env-file-input
  - api:configbind-env-files
  - flow:config-load
related:
  - decision:env-interpolation-layer
  - rule:env-interpolation-syntax
  - concept:config-overlay
  - term:config-source
```
