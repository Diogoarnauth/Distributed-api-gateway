package router

import (
	"testing"

	"distributed-api-gateway/internal/config"
)

func TestResolve(t *testing.T) {
	resolver := NewResolver([]config.Route{
		{
			Path:      "/api",
			Upstreams: []string{"http://localhost:8081"},
		},
		{
			Path:      "/api/orders",
			Upstreams: []string{"http://localhost:8082"},
		},
		{
			Path:      "/api/users",
			Upstreams: []string{"http://localhost:8083"},
		},
	})

	tests := []struct {
		name     string
		path     string
		expected string
		found    bool
	}{
		{"Exact match", "/api/orders", "/api/orders", true},
		{"Nested path", "/api/orders/123", "/api/orders", true},
		{"Users route", "/api/users/42", "/api/users", true},
		{"Longest prefix", "/api/orders/123/items", "/api/orders", true},
		{"Fallback route", "/api/products", "/api", true},
		{"Segment boundary", "/api/ordersXYZ", "/api", true},
		{"Unknown route", "/unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, found := resolver.Resolve(tt.path)

			if found != tt.found {
				t.Fatalf("Expected found=%v, got %v", tt.found, found)
			}

			if !found {
				return
			}

			if route.Path != tt.expected {
				t.Errorf("Expected route %q, got %q", tt.expected, route.Path)
			}
		})
	}
}
