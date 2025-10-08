# gHTTP
gHTTP is a Go-powered file server that mirrors the ergonomics of `python -m http.server`, adds structured zap-based request logging, integrates Cobra + Viper configuration, and now provisions self-signed HTTPS certificates that can be trusted system-wide for local development.

## Installation
Install gHTTP with the Go toolchain:

```
go install github.com/temirov/ghttp/cmd/ghttp@latest
```

Go 1.24.6 or newer is required, matching the minimum version declared in `go.mod`.

After installation the `ghttp` binary is placed in `$GOBIN` (or `$GOPATH/bin`). The root command accepts an optional positional `PORT` argument so existing workflows keep working.

### Usage examples

| Scenario | Example command | Notes |
| --- | --- | --- |
| Serve the current working directory on the default port 8000 | `ghttp` | Mirrors `python -m http.server` with structured logging. |
| Serve a specific directory on a chosen port | `ghttp --directory /srv/www 9000` | Exposes `/srv/www` at <http://localhost:9000>. |
| Bind to a specific interface | `ghttp --bind 192.168.1.5 8080` | Restricts listening to the provided IP address. |
| Serve HTTPS with an existing certificate | `ghttp --tls-cert cert.pem --tls-key key.pem 8443` | Keeps backwards-compatible manual TLS support. |
| Provision and trust the development root CA | `ghttp https setup` | Generates `~/.config/ghttp/certs/ca.pem` and installs it into the OS trust store (may require elevated privileges). |
| Serve HTTPS with self-signed certificates | `ghttp https serve 8443` | Issues leaf certificates for `localhost`, `127.0.0.1`, and `::1`, then launches the HTTPS server. |
| Remove the development certificates | `ghttp https uninstall` | Deletes local key material and removes the CA from the OS trust store. |

### Key capabilities
* Choose between HTTP/1.0 and HTTP/1.1 with `--protocol`/`-p`; the server tunes keep-alive behaviour automatically.
* Provision a development certificate authority with `ghttp https setup`, store it at `~/.config/ghttp/certs`, and install it into macOS, Linux, or Windows trust stores using native tooling.
* Issue SAN-aware leaf certificates on demand during `ghttp https serve`, covering `localhost`, `127.0.0.1`, `::1`, and additional hosts supplied via repeated `--host` flags or Viper configuration.
* Suppress automatic directory listings by exporting `GHTTPD_DISABLE_DIR_INDEX=1`; the handler returns HTTP 403 for directory roots.
* Configure every flag via `~/.config/ghttp/config.yaml` or environment variables prefixed with `GHTTP_` (for example, `GHTTP_SERVE_DIRECTORY=/srv/www`).

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

## License
This project is distributed under the terms of the [MIT License](./LICENSE).
Copyright (c) 2025 Vadym Tyemirov. Refer to the license file for the complete text, including permissions and limitations.
