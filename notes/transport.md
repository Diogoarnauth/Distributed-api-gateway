# http.Transport

## What it is

The piece of Go that **sends an HTTP request over the network and brings back the response**. Nothing more. It implements an interface with a single method:

```go
type RoundTripper interface {
	RoundTrip(*http.Request) (*http.Response, error)
}
```

A ready request goes in, a response (or an error) comes out. Inside, the Transport:

- opens a TCP connection (or reuses one from the **pool**)
- does the TLS handshake if the upstream is https
- writes the request as bytes on the connection
- reads the response coming back
- applies the **timeouts** of each phase

It knows nothing about clients, gateways, routes or hop-by-hop headers. `http.Get(...)` uses it too: `http.DefaultClient` → `http.DefaultTransport`.

## Transport vs ReverseProxy

They work at different levels, and the proxy **uses** the Transport:

```
Client ──► ReverseProxy ──────────────────────────────► Client
            │ prepares the request        ▲ copies the response
            ▼                             │
           Transport.RoundTrip ──► network ──► Upstream
            (connections, pool, timeouts)
```

- **ReverseProxy:** receives the client's request, builds the outgoing one (`Rewrite`, hop-by-hop cleanup, `X-Forwarded-For`), calls `Transport.RoundTrip`, copies the response back, and calls `ErrorHandler` on error.
- **Transport:** only moves bytes between the gateway and the upstream.

Analogy: the ReverseProxy is the secretary who rewrites the letter and hands the reply to the client. The Transport is the post office, with the vans (connections), reusing them (the pool) and giving up if a delivery takes too long (the timeouts).

## Our Transport (`transport.go`)

```go
dialer := &net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}

&http.Transport{
	DialContext:           dialer.DialContext, // how to open TCP connections
	TLSHandshakeTimeout:   3 * time.Second,
	ResponseHeaderTimeout: 10 * time.Second,   // catches hung upstreams
	IdleConnTimeout:       90 * time.Second,
	MaxIdleConnsPerHost:   100,
}
```

- **`net.Dialer`** is the object that opens TCP connections ("dial", like dialing a phone number). `Timeout` limits how long that takes.
- **Idle connection:** open, but not carrying a request right now. It sits in the pool waiting to be reused.
- **`MaxIdleConnsPerHost`:** how many idle connections to keep **per upstream**. It does not limit active connections. Go's default is 2, which closes most connections after a burst.
- **`IdleConnTimeout`:** an idle connection unused for 90s is closed by the gateway.
- **`KeepAlive` (TCP):** a different thing. The OS sends a tiny probe on a quiet connection every 30s to check that the other side is still alive. Not to be confused with HTTP keep-alive, which means reusing a connection for several requests.

## One Transport shared by all proxies

A Transport is **not a thread or a worker**. It is an object holding shared state (the pool) and is safe for concurrent use: every request runs in its own goroutine and calls `RoundTrip` at the same time. The pool keeps connections separately per upstream, so there is no queue and no bottleneck.

Go's docs say Transports should be **reused, not created as needed**. One per proxy would not add capacity; it would only split the pool and duplicate the configuration.
