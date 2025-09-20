# gHTTP
gHTTP is a minimal Go file server that mirrors the `python -m http.server` CLI, adds structured request logging for every transfer, supports optional TLS termination, and shuts down gracefully when it receives system termination signals.

## Installation
Install gHTTP with the Go toolchain:

```
go install github.com/temirov/ghttp@latest
```

Go 1.24.6 or newer is required, matching the minimum version declared in `go.mod`.

After installation the `ghttp` binary is placed in `$GOBIN` (or `$GOPATH/bin`), and it accepts an optional positional `PORT` argument that mirrors `python -m http.server`.

### Usage examples

| Scenario | Example command | Notes |
| --- | --- | --- |
| Serve the current working directory on the default port 8000 | `ghttp` | Equivalent to running `python -m http.server` with no arguments. |
| Serve a specific directory on a chosen port | `ghttp --directory /srv/www 9000` | Exposes `/srv/www` at <http://localhost:9000>. |
| Bind to a specific interface | `ghttp --bind 192.168.1.5 8080` | Restricts listening to the provided IP address. |
| Enable TLS with matching certificate and key | `ghttp --tls-cert cert.pem --tls-key key.pem 8443` | Serves HTTPS traffic; omit the port to keep the default 8000. |

### Key capabilities
* Choose between HTTP/1.0 and HTTP/1.1 with `--protocol`/`-p`, allowing the server to tune connection headers and keep-alive behavior automatically.
* Enable or disable TLS by supplying matching `--tls-cert` and `--tls-key` flags, making it easy to toggle encrypted serving without changing other options.
* Suppress automatic directory listings by exporting `GHTTPD_DISABLE_DIR_INDEX=1`, ensuring the file server denies directory browsing when required.

## File Serving Behavior
The server delegates file handling to the Go standard library's `http.FileServer`,
initializing the handler with the target directory via `http.FileServer(http.Dir(...))`.
Because this handler reads content directly from disk for each request, file
changes are reflected immediately without requiring a filesystem watcher or
manual reload step.

Only two response headers are set by default: `Server: ghttpd` is always
emitted, and when HTTP/1.0 is negotiated the handler also sets
`Connection: close`. No cache-control or time-to-live directives are provided,
so clients and intermediate caches decide their own policies.

If you need custom caching semantics, wrap the file server handler with your own
`http.Handler` that sets `Cache-Control`, `ETag`, or other headers before
forwarding the request to the embedded `http.FileServer` instance.
