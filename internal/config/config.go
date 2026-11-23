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
	Path      string   `json:"path"`
	Upstreams []string `json:"upstreams"`
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