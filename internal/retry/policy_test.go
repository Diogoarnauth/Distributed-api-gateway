package retry

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The errors below are produced by a real http.Transport, so the policy is
// checked against what the gateway will actually see, not hand-made values.

func refusedError(t *testing.T) error {
	t.Helper()

	// Grab a free port, then close it: nothing listens there anymore.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	req, _ := http.NewRequest(http.MethodPost, "http://"+addr, nil)
	_, err = (&http.Transport{}).RoundTrip(req)
	if err == nil {
		t.Fatal("expected connection refused")
	}
	return err
}

func responseTimeoutError(t *testing.T) error {
	t.Helper()

	release := make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // hang until the test ends
	}))
	t.Cleanup(func() { close(release); up.Close() })

	req, _ := http.NewRequest(http.MethodGet, up.URL, nil)
	tr := &http.Transport{ResponseHeaderTimeout: 50 * time.Millisecond}
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatal("expected response header timeout")
	}
	return err
}

func connectionCutError(t *testing.T) error {
	t.Helper()

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close() // read the request, then hang up without answering
	}))
	t.Cleanup(up.Close)

	req, _ := http.NewRequest(http.MethodGet, up.URL, nil)
	_, err := (&http.Transport{}).RoundTrip(req)
	if err == nil {
		t.Fatal("expected connection cut")
	}
	return err
}

func TestShouldRetry(t *testing.T) {
	refused := refusedError(t)
	timeout := responseTimeoutError(t)
	cut := connectionCutError(t)

	tests := []struct {
		name    string
		attempt Attempt
		want    bool
	}{
		// Request never sent: any method.
		{"refused GET", Attempt{Method: "GET", Err: refused}, true},
		{"refused POST", Attempt{Method: "POST", Err: refused}, true},

		// Ambiguous failures: only idempotent requests.
		{"timeout GET", Attempt{Method: "GET", Err: timeout}, true},
		{"timeout PUT", Attempt{Method: "PUT", Err: timeout}, true},
		{"timeout POST", Attempt{Method: "POST", Err: timeout}, false},
		{"timeout POST with key", Attempt{Method: "POST", HasIdempotencyKey: true, Err: timeout}, true},
		{"cut GET", Attempt{Method: "GET", Err: cut}, true},
		{"cut PATCH", Attempt{Method: "PATCH", Err: cut}, false},

		// Our own context ended: never.
		{"client canceled", Attempt{Method: "GET", Err: fmt.Errorf("proxy: %w", context.Canceled)}, false},
		{"budget exceeded", Attempt{Method: "GET", Err: context.DeadlineExceeded}, false},

		// Upstream answered.
		{"503 GET", Attempt{Method: "GET", StatusCode: 503}, true},
		{"502 DELETE", Attempt{Method: "DELETE", StatusCode: 502}, true},
		{"503 POST", Attempt{Method: "POST", StatusCode: 503}, false},
		{"503 POST with key", Attempt{Method: "POST", HasIdempotencyKey: true, StatusCode: 503}, true},
		{"200", Attempt{Method: "GET", StatusCode: 200}, false},
		{"404", Attempt{Method: "GET", StatusCode: 404}, false},
		{"500", Attempt{Method: "GET", StatusCode: 500}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := ShouldRetry(tt.attempt)

			if got != tt.want {
				t.Errorf("ShouldRetry = %v (%s), want %v", got, reason, tt.want)
			}
		})
	}
}
