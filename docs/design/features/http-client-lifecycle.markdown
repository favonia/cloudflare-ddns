# Design Note: HTTP Client Lifecycle

Read when: changing HTTP client construction, transport sharing, or connection cleanup.

Defines: the sharing and reclamation policy for HTTP clients and Transports controlled by this project. Shoutrrr manages its HTTP clients and connection lifecycle internally. We assume it does not replace `http.DefaultClient` or `http.DefaultTransport`, or mutate their configuration.

## Sharing and Reclamation

A Transport owns the connection pool. Separate HTTP clients and retry wrappers may share a Transport while retaining different request policies.

- **IP detection:** keep separate IPv4 and IPv6 pools. The updater calls `CloseIdleConnections()` on these pools after each round's detection work to reduce reuse of connections established under an earlier network state.
- **General HTTP traffic:** use the process-default pool for clients this project constructs or supplies to libraries. After normal completion, rely on the Transport's `IdleConnTimeout` rather than explicitly closing idle connections after each operation. The default five-minute update interval does not justify separate pools merely to optimize reuse.

Configure shared clients and Transports before use. Copying a struct containing a client pointer preserves sharing; it does not create an independent pool. Do not mutate shared transport settings for an individual request.

## Cleanup Boundaries

Callers must close response bodies and bound request execution with appropriate deadlines or cancellation. Idle reclamation does not replace these responsibilities. Preserve response-size limits rather than draining unlimited data to enable reuse.

[`CloseIdleConnections()`](https://pkg.go.dev/net/http#Client.CloseIdleConnections) leaves active connections alone and does not guarantee a fresh route or repair DNS failures. Its effect covers the shared Transport, not just the caller's service. Dependencies may also invoke it during failure handling; `go-retryablehttp` does so on some error paths.
