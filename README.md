# gHTTP
A simple http server written in Go with TLS support

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
