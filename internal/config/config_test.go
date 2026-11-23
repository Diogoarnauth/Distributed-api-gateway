package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Valid configuration",
			config: Config{
				Routes: []Route{
					{
						Path:      "/api/orders",
						Upstreams: []string{"http://localhost:8081"},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "No routes",
			config:  Config{},
			wantErr: true,
		},
		{
			name: "Duplicate routes",
			config: Config{
				Routes: []Route{
					{
						Path:      "/api/orders",
						Upstreams: []string{"http://localhost:8081"},
					},
					{
						Path:      "/api/orders",
						Upstreams: []string{"http://localhost:8082"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid path",
			config: Config{
				Routes: []Route{
					{
						Path:      "api/orders",
						Upstreams: []string{"http://localhost:8081"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "No upstreams",
			config: Config{
				Routes: []Route{
					{
						Path:      "/api/orders",
						Upstreams: []string{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid upstream URL",
			config: Config{
				Routes: []Route{
					{
						Path:      "/api/orders",
						Upstreams: []string{"http://%invalid"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Unsupported protocol",
			config: Config{
				Routes: []Route{
					{
						Path:      "/api/orders",
						Upstreams: []string{"ftp://localhost:8081"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr && err == nil {
				t.Error("Expected an error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	t.Run("Valid JSON file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")

		content := `{
			"routes": [
				{
					"path": "/api/orders",
					"upstreams": ["http://localhost:8081"]
				}
			]
		}`

		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Expected valid configuration: %v", err)
		}

		if len(cfg.Routes) != 1 {
			t.Errorf("Expected 1 route, got %d", len(cfg.Routes))
		}
	})

	t.Run("Missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.json")

		_, err := Load(path)

		if err == nil {
			t.Error("Expected an error for missing file")
		}
	})

	t.Run("Malformed JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "invalid.json")

		if err := os.WriteFile(path, []byte(`{"routes": [`), 0600); err != nil {
			t.Fatal(err)
		}

		_, err := Load(path)

		if err == nil {
			t.Error("Expected an error for malformed JSON")
		}
	})

	t.Run("Invalid configuration", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "invalid-config.json")

		if err := os.WriteFile(path, []byte(`{"routes": []}`), 0600); err != nil {
			t.Fatal(err)
		}

		_, err := Load(path)

		if err == nil || !strings.Contains(err.Error(), "invalid configuration") {
			t.Errorf("Expected configuration validation error, got: %v", err)
		}
	})
}