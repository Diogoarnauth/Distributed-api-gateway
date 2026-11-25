# Timeouts (M3.1)

## Why

Without a limit, a slow or hung upstream holds the client's request, a goroutine and a connection forever. `http.DefaultTransport` has **no** response timeout, so that was our situation before M3.1.

A timeout makes the failure **bounded and predictable**: the client gets an answer (504) in a known time, and resources are freed.

## The trade-off

- **Too short:** cuts legitimate slow requests.
- **Too long:** resources stay busy and latency grows for everyone.

## Two sides, two sets of timeouts

```
Client ══► [http.Server] Gateway [http.Transport] ══► Upstream
```

**Client → gateway** (`http.Server`, in `main.go`):

| Setting | Limits | Ours |
|---|---|---|
| `ReadHeaderTimeout` | receiving the request headers (Slowloris) | 5s |
| `ReadTimeout` | reading the whole request, body included | 30s |
| `IdleTimeout` | a keep-alive connection sitting unused | 60s |
| `WriteTimeout` | writing the response | none (see below) |

**Gateway → upstream** (`http.Transport`, in `transport.go`), one limit per phase:

```
dial (TCP) -> TLS handshake -> wait for response headers -> body
```

| Setting | Limits | Ours |
|---|---|---|
| `net.Dialer.Timeout` | opening the TCP connection | 2s |
| `TLSHandshakeTimeout` | the TLS handshake (https upstreams) | 3s |
| `ResponseHeaderTimeout` | waiting for the upstream to start answering, which **catches hung upstreams** | 10s |
| `IdleConnTimeout` | an unused connection staying in the pool | 90s |

## Deliberate choices

- **No body or write timeout:** large or streamed responses can legitimately take long. The upstream wait is already limited by `ResponseHeaderTimeout`, and the client's context cancels the request if they leave.
- **One shared Transport:** all proxies use the same connection pool.
- **`MaxIdleConnsPerHost: 100`:** Go's default is 2, so under load most connections would be closed and reopened on every request, losing the benefit of the pool.

## 502 vs 504

The proxy's `ErrorHandler` tells them apart:

- **504 Gateway Timeout:** a time limit was hit. The error is a `net.Error` with `Timeout() == true`, or `context.DeadlineExceeded`.
- **502 Bad Gateway:** any other failure (connection refused, reset, ...).

This matters for retries (M3.3): a refused connection means the upstream never got the request, so retrying is safe. A timeout is ambiguous, because the upstream may have processed it.

## Verified

| Upstream | Result |
|---|---|
| fast | `200` in 2ms |
| not running | `502` in 1ms (connection refused) |
| sleeps 12s | `504` after ~10s (`timeout awaiting response headers`) |
