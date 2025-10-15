# gHTTP

[![GitHub release](https://img.shields.io/github/release/temirov/ghttp.svg)](https://github.com/temirov/ghttp/releases)

gHTTP is a Go-powered file server that mirrors the ergonomics of `python -m http.server`, adds structured zap-based request logging, renders mardown files as HTML, and provisions self-signed HTTPS certificates for local development.

*gHTTP is fast.*

## Installation

### Docker

The workflow defined in `.github/workflows/docker-publish.yml` publishes container images to GitHub Container Registry whenever the `master` branch is updated or a release tag is created.

Pull and run the latest Docker image:

```bash
docker pull ghcr.io/temirov/ghttp:latest
docker run -p 8080:8080 -v $(pwd):/data ghcr.io/temirov/ghttp:latest --directory /data
```

The published manifest covers:
- `linux/amd64` (x86_64)
- `linux/arm64` (ARM 64-bit)

Custom port and directory examples:

```bash
# Serve current directory on port 9000
docker run -p 9000:9000 -v $(pwd):/data ghcr.io/temirov/ghttp:latest --directory /data 9000

# Serve with HTTPS (requires certificate setup)
docker run -p 8443:8443 -v $(pwd):/data -v ~/.config/ghttp:/root/.config/ghttp ghcr.io/temirov/ghttp:latest --directory /data --https 8443
```

### Releases

Download the latest binaries from the [Releases page](https://github.com/temirov/ghttp/releases).

### Go toolchain

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
| Serve HTTPS with self-signed certificates | `ghttp --https 8443` | Installs the development CA, serves HTTPS, and removes credentials on exit. |
| Disable Markdown rendering | `ghttp --no-md` | Serves raw Markdown assets without HTML conversion. |
| Switch logging format | `ghttp --logging-type JSON` | Emits structured JSON logs instead of the default console view. |
| Remove the development certificates | `ghttp https uninstall` | Deletes local key material and removes the CA from the OS trust store. |

### Key capabilities
* Choose between HTTP/1.0 and HTTP/1.1 with `--protocol`/`-p`; the server tunes keep-alive behaviour automatically.
* Provision a development certificate authority with `ghttp --https` (or `ghttp https setup` for manual control), storing it at `~/.config/ghttp/certs` and installing it into macOS, Linux, or Windows trust stores using native tooling.
* Issue SAN-aware leaf certificates on demand whenever HTTPS is enabled, covering `localhost`, `127.0.0.1`, `::1`, and additional hosts supplied via repeated `--host` flags or Viper configuration.
* Render Markdown files (`*.md`) to HTML automatically, treat `README.md` as a directory landing page, and skip the feature entirely with `--no-md` or `serve.no_markdown: true` in configuration.
* When Firefox is installed, automatically configure its profiles to trust the generated certificates so browser warnings disappear on the next restart.
* Suppress automatic directory listings by exporting `GHTTPD_DISABLE_DIR_INDEX=1`; the handler returns HTTP 403 for directory roots.
* Configure every flag via `~/.config/ghttp/config.yaml` or environment variables prefixed with `GHTTP_` (for example, `GHTTP_SERVE_DIRECTORY=/srv/www`).

### Browser trust behaviour
| Browser | Trust source | Restart needed? | Notes |
| --- | --- | --- | --- |
| Safari (macOS) | System keychain | No | macOS keychain updates apply immediately to Safari and other WebKit clients. |
| Chrome / Edge | OS certificate store | No | Chromium-based browsers rely on the OS trust store and accept the CA on the next handshake. |
| Firefox | Firefox NSS store or enterprise roots | Yes | Profiles are updated automatically: if `certutil` is available the CA is imported, otherwise `security.enterprise_roots.enabled` is set via `user.js`. Restart Firefox to apply the change. |
| Other browsers | OS certificate store | No | Most modern browsers reuse the system trust store; no manual action required. |

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
