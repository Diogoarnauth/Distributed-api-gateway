package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"distributed-api-gateway/internal/config"
	"distributed-api-gateway/internal/router"
	"distributed-api-gateway/internal/balancer"
)

type Gateway struct {
	resolver  *router.Resolver
	proxies   map[string][]http.Handler
	balancers map[string]*balancer.RoundRobin
}

func NewGateway(routes []config.Route) (*Gateway, error) {
	resolver := router.NewResolver(routes)

	proxies := make(map[string][]http.Handler)

	balancers := make(map[string]*balancer.RoundRobin)

	for _, route := range routes {
		balancers[route.Path] = balancer.NewRoundRobin()

		for _, upstream := range route.Upstreams {
			target, err := url.Parse(upstream)
			if err != nil {
				return nil, fmt.Errorf("invalid upstream %q: %w", upstream, err)
			}

			proxies[route.Path] = append(
				proxies[route.Path],
				newReverseProxy(target),
			)
		}
	}

	return &Gateway{
		resolver:  resolver,
		proxies:   proxies,
		balancers: balancers,
	}, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route, found := g.resolver.Resolve(r.URL.Path)

	if !found {
		http.NotFound(w, r)
		return
	}

	// Temporário: selecionar o primeiro proxy.
	proxies := g.proxies[route.Path]

	index := g.balancers[route.Path].Next(len(proxies))

	proxy := proxies[index]
	
	r2 := r.Clone(r.Context())
	r2.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")

	if r2.URL.Path == "" {
		r2.URL.Path = "/"
	}

	r2.URL.RawPath = ""

	proxy.ServeHTTP(w, r2)
}