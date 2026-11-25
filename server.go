package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"distributed-api-gateway/internal/config"
)

// newMux registers the gateway's own endpoints (/health, /hello) and one
// pattern pair per configured route, so any path in config.json reaches the
// gateway, not only paths under /api. Paths that match no route get the mux's
// 404, and a wrong method on /health or /hello still gets 405.
func newMux(gateway http.Handler, routes []config.Route) *http.ServeMux {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Hello from Go",
		})
	})

	// "/api/orders" matches only that exact path; "/api/orders/" matches
	// everything below it. Register both, once each ("/api/orders" and
	// "/api/orders/" in the config would otherwise register a pattern twice,
	// which makes the mux panic).
	registered := make(map[string]bool)

	for _, route := range routes {
		base := strings.TrimSuffix(route.Path, "/")

		for _, pattern := range []string{base, base + "/"} {
			if pattern == "" || registered[pattern] {
				continue
			}

			registered[pattern] = true
			mux.Handle(pattern, gateway)
		}
	}

	return mux
}
