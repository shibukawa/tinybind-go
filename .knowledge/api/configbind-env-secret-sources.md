---
id: api:configbind-env-secret-sources
type: api
title: LoadOptions EnvSecretFiles and EnvSecretDirs
---
Two LoadOptions slices add secret-origin inputs above EnvFiles, LoadResult reports what was read, and the existing file place names the entry.

```yaml
status: implemented
options:
  - field: 'EnvSecretFiles []string'
    doc: dotenv files read like EnvFiles, laid over every EnvFiles entry, whose values are secret by origin
    parser: decision:env-file-parser, unchanged
    missing: skipped
    unreadable_or_rejected_line: load error, same messages as EnvFiles
  - field: 'EnvSecretDirs []string'
    doc: directories where each regular file is one variable, the file name the variable name and the content the value, laid over EnvSecretFiles
    layout: rule:secret-dir-layout
    missing: skipped
    unreadable_dir_or_entry: load error, 'configbind: read env dir %q: %v' and 'configbind: read env dir %q entry %q: %v'
result:
  - field: 'EnvSecretFiles []string'
    meaning: the secret files actually read, in order
  - field: 'EnvSecretDirs []string'
    meaning: the directories actually read, in order
  - note: EnvFiles keeps listing only plain files, so a summary can print the three groups apart
place:
  reuse: PlaceEnvFile + path for both, per decision:env-secret-source-shape
  file: 'file_env:.env.local'
  dir_entry: 'file_env:/run/secrets/DB_PASSWORD, the entry path, so EnvFileOf names the exact file'
  secrecy: not in the place; data:provenance-event Masked carries it, per rule:secret-origin-masking
unchanged:
  - api:configbind-env-files fields and semantics
  - PlaceEnv for a name Environ supplied, even when a secret source also supplied it
  - api:configbind-provenance shape and ordering
  - EnvVariable, ScaffoldEnv, ScaffoldTOML
related:
  - requirement:secret-env-sources
  - rule:env-file-composition
  - term:config-source
  - flow:config-load
  - system:configbind
```
