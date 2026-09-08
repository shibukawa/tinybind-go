---
id: rule:secret-dir-layout
type: rule
title: Secret Directory Layout
---
An EnvSecretDirs entry is read as a flat set of one-value files: the file name is the variable name as given, the content minus trailing line endings is the value.

```yaml
model: Docker secrets under /run/secrets and Kubernetes secret volume mounts
entry_name:
  used_as: the environment variable name, exactly as spelled
  no_normalization: >
    env:"OTEL_SERVICE_NAME" overrides are exact strings, and upper-casing would
    make db_password and DB_PASSWORD collide; an operator names the secret as
    the variable is named
  consequence: a Docker secret must be created under the upper-case name, e.g. DB_PASSWORD
  invalid_name: a name no definition asks for is simply never read, as an unknown environment variable is today
value:
  content: the file bytes as a string
  trailing: 'strip trailing \n and \r characters only; inner and leading whitespace is kept'
  why: >
    echo -n is easy to forget when creating a secret, and a password ending in a
    newline is never what was meant; a password ending in a space might be
  empty_file: counts as set with the empty value, matching NAME=
skipped_entries:
  - names starting with a dot, which covers K8s ..data and ..2026_09_09 snapshot links
  - directories
  - anything that is not a regular file after following symlinks, such as a socket
followed: symlinks to regular files, because K8s mounts every key as a symlink into ..data
errors:
  missing_dir: skipped, listed nowhere
  unreadable_dir: load error naming the directory
  unreadable_entry: load error naming the directory and the entry
order_within_dir: irrelevant; names are distinct within one directory
order_across_dirs: slice order, later directory wins on the same name
listing: os.ReadDir plus os.ReadFile, both available under TinyGo wasip1 with a preopened directory
applies_to:
  - requirement:secret-env-sources
  - api:configbind-env-secret-sources
  - rule:env-file-composition
related:
  - decision:env-file-parser
  - requirement:configbind-tinygo
```
