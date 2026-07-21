---
id: system:configdir
type: system
title: configdir Library
---
github.com/shibukawa/configdir resolves OS convention config and cache directories for vendor and tool names.

```yaml
module: github.com/shibukawa/configdir
role: multi-platform config directory lookup
api_sketch:
  - 'configdir.New(vendorName, applicationName) ConfigDir'
  - 'QueryFolderContainsFile(fileName) *Config  // first existing file wins'
  - 'QueryFolders(type) []*Config'
inputs:
  - vendor name from configbind API
  - application / tool name from configbind API
  - config file name e.g. app.toml
path_roots:
  windows:
    system: '%PROGRAMDATA%'
    user: '%APPDATA%'
  linux_bsd_xdg:
    system: '${XDG_CONFIG_DIRS} default /etc/xdg'
    user: '${XDG_CONFIG_HOME} default ${HOME}/.config'
  darwin:
    system: '/Library/Application Support'
    user: '${HOME}/Library/Application Support'
search_order_existing_file:
  - optional LocalPath if set
  - user Global folder
  - system folders
semantics:
  - returns one folder that contains the file; does not merge file contents
  - user path is preferred over system when both exist
tinygo:
  status: host TinyGo build smoke passed
  evidence: tinygo build of import-only program succeeded
  deps: stdlib os path/filepath ioutil only; no cgo no reflect
  note: wasm/js targets without filesystem remain out of scope for config file load
used_by:
  - decision:config-file-path-resolution
  - requirement:config-file-discovery
  - system:configbind
related:
  - requirement:configbind-tinygo
  - flow:config-load
```
