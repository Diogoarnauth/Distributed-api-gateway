# X-Forwarded-For

## The problem

The upstream only sees who is on the other side of **its** TCP connection, and that is always the gateway (e.g. `127.0.0.1`). The real client's IP is lost.

```
Client 203.0.113.7 ──► Gateway 127.0.0.1 ──► Upstream
                                              "the request came from 127.0.0.1"
```

The upstream needs the real IP for logs, blocking abusive clients and per-client rate limiting (M4).

## The solution

The gateway writes the client's IP in a header:

```
X-Forwarded-For: 203.0.113.7
```

If there are several proxies, each one appends to the list: `client, proxy1, proxy2`. By convention the **first** entry is the original client.

Related headers: `X-Forwarded-Host` (the host the client asked for) and `X-Forwarded-Proto` (`http` or `https`).

## The trap: clients can fake it

Headers are just text the client writes. An attacker whose real IP is `203.0.113.7` can send:

```
X-Forwarded-For: 1.2.3.4
```

- **With `Director` (old API):** the proxy keeps the client's header and **appends** the real IP. The upstream gets `1.2.3.4, 203.0.113.7`, takes the first entry and believes the client is `1.2.3.4`. By changing the fake IP on every request, the attacker escapes per-IP limits and bans.
- **With `Rewrite` (what we use):** the proxy **deletes** any `X-Forwarded-*` from the client before calling our function. `r.SetXForwarded()` then writes them from the **real TCP connection** (`r.In.RemoteAddr`), which the client can't fake. The upstream gets `X-Forwarded-For: 203.0.113.7`.

## In our code

```go
Rewrite: func(r *httputil.ProxyRequest) {
	r.SetURL(target)
	r.SetXForwarded()
},
```

We use `Rewrite`, so fake headers from the client are removed, and `SetXForwarded()` writes the real IP. Without that call the upstream would receive **no** `X-Forwarded-For` at all (not a fake one).

## Rule of thumb

Only trust `X-Forwarded-For` entries added by proxies **you control**. Anything that came from the client is untrusted input. This matters again in M4, when rate limiting by client means identifying clients without trusting their headers blindly.
