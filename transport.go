// transport.go: the HTTP client side of the gateway (gateway -> upstream).
//
// httputil.ReverseProxy uses an http.RoundTripper to actually send requests.
// Without one it falls back to http.DefaultTransport, which has NO limit on
// how long to wait for an upstream's response: a hung upstream would hold the
// client's request, a goroutine and a connection forever. Here we build one
// Transport with explicit limits for each phase of the request. It is shared
// by every route's retryTransport (retry_transport.go), which wraps it and
// calls it once per attempt, so all upstreams share one connection pool:
//
//	dial (TCP connect) -> TLS handshake -> wait for response headers -> body
//
// The response body has no timeout on purpose: a large or streamed response
// can legitimately take long, and the client's context still cancels it.
// When a limit is hit (and no retry is allowed or left), the proxy's
// ErrorHandler answers 504 Gateway Timeout.

package main

import (
	"net"
	"net/http"
	"time"
)

const (
	// Time to open the TCP connection. Upstreams are on our network, so a
	// healthy one connects in milliseconds; waiting longer only delays the 502.
	dialTimeout = 2 * time.Second

	// Time for the TLS handshake with https upstreams.
	tlsHandshakeTimeout = 3 * time.Second

	// Time from sending the request until the upstream starts answering
	// (status line + headers). This is the one that catches a hung upstream.
	responseHeaderTimeout = 10 * time.Second

	// How long an unused connection stays in the pool before being closed.
	idleConnTimeout = 90 * time.Second

	// Idle connections kept per upstream. Go's default is only 2, so under
	// load most connections would be closed and reopened on every request.
	maxIdleConnsPerHost = 100
)

func newTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: 30 * time.Second,
	}

	return &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       idleConnTimeout,
		MaxIdleConns:          maxIdleConnsPerHost * 10,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		ForceAttemptHTTP2:     true,
	}
}
