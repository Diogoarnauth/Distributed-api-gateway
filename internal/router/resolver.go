package router

import (
	"strings"

	"distributed-api-gateway/internal/config"
)

type Resolver struct {
	routes []config.Route
}

func NewResolver(routes []config.Route) *Resolver {
	return &Resolver{
		routes: routes,
	}
}

func (r *Resolver) Resolve(path string) (*config.Route, bool) {
	var bestMatch *config.Route

	for i := range r.routes {
		route := &r.routes[i]

		if !matchesPrefix(path, route.Path) {
			continue
		}

		if bestMatch == nil || len(route.Path) > len(bestMatch.Path) {
			bestMatch = route
		}
	}

	if bestMatch == nil {
		return nil, false
	}

	return bestMatch, true
}

func matchesPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}

	if prefix == "/" {
		return strings.HasPrefix(path, "/")
	}

	return strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/")
}