# Hop-by-hop vs end-to-end headers

## The idea

With a gateway in the middle there are two separate TCP connections:

```
Client ══ connection 1 ══► Gateway ══ connection 2 ══► Upstream
```

Some headers describe **the connection** and some describe **the request**.

- **Hop-by-hop:** about one connection only. They stay at the gateway and are **not** forwarded.
  Example: `Connection: keep-alive` means "keep *this* connection open". The client is talking about connection 1; connection 2 is the gateway's business.
- **End-to-end:** about the request itself. They are **forwarded** to the upstream unchanged.
  Examples: `Authorization`, `Content-Type`, `Accept`, `Cookie`.

## How the proxy decides

It doesn't look at the header contents. `httputil.ReverseProxy` follows two rules from the HTTP spec:

1. **A fixed list** that is always hop-by-hop (`hopHeaders` in Go's source):
   `Connection`, `Proxy-Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `Te`, `Trailer`, `Transfer-Encoding`, `Upgrade`
2. **Headers named in `Connection`.** `Connection: X-Foo` makes `X-Foo` hop-by-hop too, so it is removed.

**Everything else is forwarded by default.** The proxy removes hop-by-hop headers in both directions: from the request on the way to the upstream, and from the response on the way back.

Exceptions: for WebSockets the proxy keeps `Upgrade` so the protocol switch works, and it keeps `TE: trailers`.

## Why it matters

- **Correctness:** each hop manages its own connection (keep-alive, chunking). Forwarding these headers would give the upstream instructions about a connection it isn't part of.
- **Security:** a client can use `Connection: X-Something` to make the proxy delete a header. With `Rewrite`, the cleanup runs **before** our function, so headers we add (e.g. `X-User`) can't be removed by the client. With the older `Director`, the cleanup ran **after**, so they could be.

## Protocol rules vs business rules

The hop-by-hop list comes from the HTTP protocol and is the same for every gateway. Business-specific rules go in `Rewrite`:

```go
Rewrite: func(r *httputil.ProxyRequest) {
	r.SetURL(target)
	r.Out.Header.Del("X-Internal")   // don't leak this header to the upstream
	r.Out.Header.Set("X-User", user) // add info for the upstream
},
```

Read from `r.In` (the client's request), change `r.Out` (the request sent upstream).

## Analogy

A letter sent through several post offices. The envelope (hop-by-hop) is replaced at every office. The letter inside (end-to-end) reaches the recipient untouched.
