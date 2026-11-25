package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestRoutes(t *testing.T) {

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantJSON   map[string]string
	}{
		{
			name:       "Health endpoint",
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusOK,
			wantJSON: map[string]string{
				"status": "ok",
			},
		},
		{
			name:       "Hello endpoint",
			method:     http.MethodGet,
			path:       "/hello",
			wantStatus: http.StatusOK,
			wantJSON: map[string]string{
				"message": "Hello from Go",
			},
		},
		{
			name:       "Unknown endpoint",
			method:     http.MethodGet,
			path:       "/unknown",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Invalid HTTP method",
			method:     http.MethodPost,
			path:       "/health",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	mux := newMux(http.NotFoundHandler(), nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"Expected status %d, got %d",
					tt.wantStatus,
					rec.Code,
				)
			}

			if tt.wantJSON != nil {

				if got := rec.Header().Get("Content-Type"); got != "application/json" {
					t.Errorf("Expected application/json, got %s", got)
				}

				var gotJSON map[string]string

				if err := json.Unmarshal(rec.Body.Bytes(), &gotJSON); err != nil {
					t.Fatalf("Invalid JSON response: %v", err)
				}

				if !reflect.DeepEqual(gotJSON, tt.wantJSON) {
					t.Errorf("Expected %v, got %v", tt.wantJSON, gotJSON)
				}
			}
		})
	}
}
