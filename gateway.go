package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"distributed-api-gateway/internal/config"
	"distributed-api-gateway/internal/router"
)

// requestTimeout is the total time budget for one client request, counted
// from when it reaches the gateway. It covers every attempt to reach an
// upstream (retries), so the client never waits longer than this.
// It must be larger than responseHeaderTimeout (transport.go), the limit
// for a single attempt, or the per-attempt limit would never matter.
const requestTimeout = 15 * time.Second

type Gateway struct {
	resolver *router.Resolver
	proxies  map[string]http.Handler // one proxy per route path
}

func NewGateway(routes []config.Route) (*Gateway, error) {
	resolver := router.NewResolver(routes)

	// One http.Transport for everything: one shared connection pool and the
	// same timeouts for every upstream (see transport.go).
	transport := newTransport()

	proxies := make(map[string]http.Handler)

	for _, route := range routes {
		upstreams := make([]*url.URL, 0, len(route.Upstreams))

		for _, upstream := range route.Upstreams {
			target, err := url.Parse(upstream)
			if err != nil {
				return nil, fmt.Errorf("invalid upstream %q: %w", upstream, err)
			}

			upstreams = append(upstreams, target)
		}

		// The route's retryTransport owns its upstreams and its round-robin
		// balancer, and wraps the shared http.Transport.
		proxies[route.Path] = newReverseProxy(newRetryTransport(upstreams, transport))
	}

	return &Gateway{
		resolver: resolver,
		proxies:  proxies,
	}, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route, found := g.resolver.Resolve(r.URL.Path)

	if !found {
		http.NotFound(w, r)
		return
	}

	// Total time budget: a child of the client's context that is canceled
	// when the client leaves (inherited) OR when requestTimeout passes
	// (context.DeadlineExceeded -> the proxy answers 504). The Transport
	// watches it, so it aborts the upstream request (and any retry) when
	// either happens. cancel releases the timer if the request ends earlier.
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	// Remove the route's strip_prefix (if any) before forwarding,
	// e.g. "/api/orders/1" -> "/orders/1" with strip_prefix "/api".
	r2 := r.Clone(ctx)
	r2.URL.Path = strings.TrimPrefix(r.URL.Path, route.StripPrefix)

	if r2.URL.Path == "" {
		r2.URL.Path = "/"
	}

	r2.URL.RawPath = ""

	// The route's proxy; its retryTransport picks the upstream per attempt.
	g.proxies[route.Path].ServeHTTP(w, r2)
}
