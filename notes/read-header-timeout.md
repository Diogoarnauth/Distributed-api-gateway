# ReadHeaderTimeout

## What it is

A field on `http.Server` that limits how long the server waits for a client to finish sending the request headers.

```go
server := &http.Server{
	Addr:              ":8080",
	Handler:           newMux(gateway),
	ReadHeaderTimeout: 5 * time.Second,
}
```

If the headers are not complete within 5 seconds, the server closes the connection.

## Why the server has to wait

HTTP runs over TCP, which is a stream of bytes, not a message that arrives all at once. The server reads bytes until it finds the empty line (`\r\n\r\n`) that ends the headers.

Until then it doesn't know the method, path or host, so it cannot pick a handler. The `ServeMux` and `Gateway.ServeHTTP` are only called after the headers are complete.

A normal client sends all the headers in a few milliseconds. Nothing forces a client to be that fast, though.

## The problem: Slowloris

Every open connection uses resources, even before the request is processed:

- a socket (file descriptor, limited per process, often 1024 by default)
- a goroutine waiting for bytes
- memory for the bytes received so far

An attacker opens thousands of connections and sends one header line every few seconds, never finishing. Without a timeout, each connection stays open forever. When the sockets run out, legitimate clients can't connect. The attack needs almost no bandwidth.

## What the timeout gives us

- **Bounded waiting:** a slow or malicious connection is closed and its resources are freed.
- **No cost for real clients:** they finish sending headers in milliseconds, far below 5s.
- **Cheap protection:** one line of configuration.

## What it does not cover

- **The body:** covered by `ReadTimeout`, which also counts the header time.
- **Writing the response:** covered by `WriteTimeout`.
- **Idle keep-alive connections between requests:** covered by `IdleTimeout`.
- **Requests to upstreams:** covered by the proxy's `http.Transport` (milestone M3).

Of these, only `ReadHeaderTimeout` is set at the moment.

## Try it

With the gateway running:

```bash
nc localhost 8080
```

Type `GET /health HTTP/1.1`, press Enter, then wait. After about 5 seconds the connection is closed.

## Analogy

A restaurant with 10 tables. Ten people sit down and say one word of their order per minute, never finishing. The kitchen is idle, but the restaurant is full and can't serve anyone. The timeout is the waiter saying: "if you haven't ordered in 5 minutes, please free the table."
