// Package config loads the gateway's routing configuration from config.json.
//
// config.json says where each request should go: every route has a path
// prefix (e.g. "/api/orders") and a list of upstream servers that handle it.
// To Go, that file is just text, so Load reads it and json.Unmarshal turns it
// into Go values ([]Route) that the code can use. The struct tags
// (`json:"path"`) tell Unmarshal which JSON key fills which field. Load runs
// only once, at startup in main.go, before the server accepts any request:
// main.go calls Load, then NewGateway uses the routes to build the resolver,
// the balancers and the proxies. After that the file is never read again.
// Validate rejects a bad config at startup (fail fast), so the gateway never
// runs with broken routes.
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Routes []Route `json:"routes"`
}

type Route struct {
	Path string `json:"path"`
	// StripPrefix is removed from the request path before it is sent to the
	// upstream, e.g. "/api" turns "/api/orders/1" into "/orders/1".
	// Empty means the path is forwarded unchanged.
	StripPrefix string   `json:"strip_prefix,omitempty"`
	Upstreams   []string `json:"upstreams"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading configuration: %w", err)
	}

	var cfg Config

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if len(c.Routes) == 0 {
		return fmt.Errorf("at least one route is required")
	}

	seen := make(map[string]bool)

	for _, route := range c.Routes {
		if !strings.HasPrefix(route.Path, "/") {
			return fmt.Errorf("route path must start with /: %q", route.Path)
		}

		if seen[route.Path] {
			return fmt.Errorf("duplicate route: %q", route.Path)
		}

		seen[route.Path] = true

		if err := validateStripPrefix(route); err != nil {
			return err
		}

		if len(route.Upstreams) == 0 {
			return fmt.Errorf("route %q has no upstreams", route.Path)
		}

		for _, upstream := range route.Upstreams {
			u, err := url.Parse(upstream)
			if err != nil {
				return fmt.Errorf(
					"route %q has invalid upstream %q: %w",
					route.Path,
					upstream,
					err,
				)
			}

			if u.Scheme != "http" && u.Scheme != "https" {
				return fmt.Errorf(
					"route %q has unsupported scheme: %q",
					route.Path,
					u.Scheme,
				)
			}

			if u.Host == "" || u.Hostname() == "" {
				return fmt.Errorf(
					"route %q has upstream without host: %q",
					route.Path,
					upstream,
				)
			}

			if u.User != nil || u.Fragment != "" || u.RawQuery != "" {
				return fmt.Errorf(
					"route %q has unsupported upstream URL components: %q",
					route.Path,
					upstream,
				)
			}
		}
	}

	return nil
}

// validateStripPrefix checks that strip_prefix, when set, is a whole-segment
// prefix of the route path ("/api" fits "/api/orders", "/ap" does not), so
// stripping it always leaves a path that starts with "/".
func validateStripPrefix(route Route) error {
	prefix := route.StripPrefix

	if prefix == "" {
		return nil
	}

	if !strings.HasPrefix(prefix, "/") || strings.HasSuffix(prefix, "/") {
		return fmt.Errorf(
			"route %q: strip_prefix must start with / and not end with /: %q",
			route.Path,
			prefix,
		)
	}

	if route.Path != prefix && !strings.HasPrefix(route.Path, prefix+"/") {
		return fmt.Errorf(
			"route %q: strip_prefix %q is not a prefix of the route path",
			route.Path,
			prefix,
		)
	}

	return nil
}
