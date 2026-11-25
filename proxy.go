// proxy.go: builds the reverse proxy for ONE route.
//
// Each route in config.json gets one proxy, created once at startup in
// NewGateway. The proxy does not know the upstreams: its Transport is the
// route's retryTransport (retry_transport.go), which picks the upstream for
// every attempt and retries safely.
// httputil.ReverseProxy does the heavy work for us: it creates a new request
// to the upstream, removes hop-by-hop headers (Connection, Keep-Alive, ...),
// hands it to the Transport, and copies the response back to the client.
// Rewrite is where we say how the outgoing request (r.Out) differs from the
// client's request (r.In): SetXForwarded writes X-Forwarded-For/Host/Proto
// from the real TCP connection (r.In.RemoteAddr), so the upstream knows the
// real client IP. The destination is set later, per attempt, by the
// retryTransport. Rewrite runs after the proxy has cleaned the headers
// (including any X-Forwarded-* sent by the client, which could be fake), so
// anything we add here reaches the upstream intact. If the final attempt
// fails, ErrorHandler logs the error and answers 499 if the client left,
// 504 Gateway Timeout when a time limit was hit, or 502 Bad Gateway for any
// other failure (connection refused, reset, ...).

package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
)

func newReverseProxy(transport http.RoundTripper) *httputil.ReverseProxy {

	proxy := &httputil.ReverseProxy{
		Transport: transport,

		Rewrite: func(r *httputil.ProxyRequest) {

			r.SetXForwarded()

		},

		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {

			// The client gave up (closed the connection), so the server
			// canceled r.Context() and the Transport aborted. Not the
			// upstream's fault: log it as such and don't count it as an
			// upstream failure. Nobody will read the response; 499 is only
			// for our own logs/metrics.
			if isClientCanceled(r, err) {
				log.Printf("Client canceled request %s %s: %v", r.Method, r.URL.Path, err)
				w.WriteHeader(statusClientClosedRequest)
				return
			}

			if isTimeout(err) {
				log.Printf("Proxy timeout %s %s: %v", r.Method, r.URL.Path, err)
				http.Error(w, "Upstream timeout", http.StatusGatewayTimeout)
				return
			}

			log.Printf("Proxy error %s %s: %v", r.Method, r.URL.Path, err)
			http.Error(w, "Upstream unavailable", http.StatusBadGateway)
		},
	}

	return proxy
}

// statusClientClosedRequest is NGINX's non-standard 499 status, widely used
// to mark requests the client abandoned before getting a response.
const statusClientClosedRequest = 499

// isClientCanceled reports whether the request failed because the client
// went away: the error is context.Canceled AND the client's own request
// context was canceled (so it isn't some other cancellation).
func isClientCanceled(r *http.Request, err error) bool {
	return errors.Is(err, context.Canceled) &&
		errors.Is(r.Context().Err(), context.Canceled)
}

// isTimeout reports whether err means a time limit was hit: a Transport
// timeout (dial, TLS handshake, response headers) is a net.Error whose
// Timeout() is true, and a context deadline is context.DeadlineExceeded.
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
