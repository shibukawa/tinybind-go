---
id: api:configbind-env-secret-sources
type: api
title: EnvFile Secret and EnvSecretDirs
---
A Secret flag on each EnvFiles entry and one LoadOptions slice of directories add secret-origin inputs, LoadResult reports what was read, and the existing file place names the entry.

```yaml
status: implemented
options:
  - field: 'EnvFile.Secret bool'
    doc: marks one EnvFiles entry's values secret by origin; the entry keeps its slice position, so secret and plain files interleave
    parser: decision:env-file-parser, unchanged
    replaced: 'EnvSecretFiles []string of v0.5.30, which forced every secret file above every plain one'
  - field: 'EnvSecretDirs []string'
    doc: directories where each regular file is one variable, the file name the variable name and the content the value, laid over every EnvFiles entry
    layout: rule:secret-dir-layout
    missing: skipped
    unreadable_dir_or_entry: load error, 'configbind: read env dir %q: %v' and 'configbind: read env dir %q entry %q: %v'
result:
  - field: 'EnvFiles []EnvFile'
    meaning: the entries actually read with their Secret flag, so a summary filters plain from secret without a second list
  - field: 'EnvSecretDirs []string'
    meaning: the directories actually read, in order
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
