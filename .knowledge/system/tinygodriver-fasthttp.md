---
id: system:tinygodriver-fasthttp
type: system
title: tinygodriver fasthttp Fork
---
Drop-in fork of valyala/fasthttp v1.73.0 that builds under TinyGo, supplying the transport fasthttpbind targets.

```yaml
import_path: github.com/shibukawa/tinygodriver/fasthttp
upstream: valyala/fasthttp v1.73.0
drop_in: change the import path only; no wrapper package selects between fork and upstream
licence: MIT for that directory, not the Apache 2.0 of the rest of tinygodriver
host_go: upstream behaviour for behaviour; every divergence is on the TinyGo side
verified: TinyGo 0.41.1 on darwin/arm64, one suite running on both compilers
build_tags:
  noasm: required under TinyGo, because klauspost/compress zstd assembly cannot link; fails at link time
  fasthttp_nozstd: drops zstd, halves the binary, makes noasm unnecessary; zstd is the only assembly in the tree
  host_go: no tag needed
tinygo_prerequisite: blank-import github.com/shibukawa/tinygodriver/netdev for the socket layer; no-op on host Go
works_under_tinygo:
  - server and client round trips, keep-alive, every method
  - chunked SetBodyStreamWriter
  - multipart/form-data uploads
  - repeated headers, cookies, 4 MiB bodies
  - FSHandler with range requests, graceful Shutdown
impossible_under_tinygo:
  serve_tls: ServeTLS and friends report ErrTLSUnsupported, because TinyGo defines neither tls.Server nor tls.X509KeyPair
  http2: Server.NextProto never fires, because TinyGo tls.ConnectionState carries no NegotiatedProtocol
  client_tls: Client cannot originate TLS; supply a Dial returning an already-encrypted conn
different_under_tinygo:
  pool_never_released: TinyGo sync.Pool is one mutex-guarded slice with no eviction, so pooled contexts settle at peak concurrency and stay; that lock is also most of the throughput gap
  throughput: about 40 percent of standard Go on the same machine
  file_serving: not zero-copy; sendfile paths need ReadFrom on os.File and net.TCPConn
  listen_port_zero: Listener.Addr reports the address asked for, so :0 hides the port
binary_size_tinygo_two_routes:
  net_http: 1.21 MB
  fork_nozstd: 2.77 MB
  fork_noasm: 5.28 MB
binary_size_host_go: fork and net/http are the same size, near 7.5 MB
related:
  - system:tinybind
  - requirement:fasthttpbind-tinygo
  - decision:fasthttpbind-runtime-package
```
